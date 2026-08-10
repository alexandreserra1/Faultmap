package detection

import (
	"fmt"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// DetectTraceBreak identifica ligações entre serviços que existiam na baseline e
// desapareceram no incidente.
//
// Instrumentação parcial é o estado normal: spans órfãos existem em quase todo
// sistema, e reportá-los como perda de contexto faria o detector gritar em toda
// investigação. Por isso a regra é comparativa e nunca olha o incidente
// sozinho — uma lacuna presente nas duas janelas descreve a instrumentação, não
// o incidente, e é deliberadamente silenciada.
//
// A ligação só é reportada quando o serviço de baixo continua emitindo
// telemetria no incidente. Se ele sumiu por completo, o que se observou foi
// ausência de dados, e chamar isso de quebra de propagação seria inventar um
// fenômeno a partir de uma coleta incompleta.
func DetectTraceBreak(baseline, incident []domain.Signal) []Finding {
	baselineLinks := parentChildLinks(baseline)
	baselineChildSpans := spanCountByService(baseline)
	incidentLinks := parentChildLinks(incident)
	incidentServices := servicesWithSignals(incident)

	links := make([]serviceLink, 0, len(baselineLinks))
	for link := range baselineLinks {
		links = append(links, link)
	}
	// A ordenação impede que a iteração sobre o mapa vaze para a saída.
	sort.Slice(links, func(first, second int) bool {
		if links[first].child != links[second].child {
			return links[first].child < links[second].child
		}
		return links[first].parent < links[second].parent
	})

	findings := make([]Finding, 0, len(links))
	for _, link := range links {
		if incidentLinks[link] > 0 {
			continue
		}
		if _, present := incidentServices[link.child]; !present {
			continue
		}
		if _, present := incidentServices[link.parent]; !present {
			continue
		}

		baselineCount := baselineLinks[link]
		// O score mede o quanto a ligação sustentava o serviço filho. Perder a
		// ligação que acompanhava quase todos os seus spans é evidência muito
		// mais forte do que perder uma que aparecia raramente; um score fixo
		// trataria as duas como idênticas, o que é caro demais num detector que
		// já nasce sob suspeita de falso positivo.
		share := fraction(baselineCount, baselineChildSpans[link.child])

		// O finding pertence ao serviço de baixo porque são os spans dele que
		// deixaram de ter pai identificável nos traces do incidente.
		finding := newFinding(
			RuleTraceBreak,
			link.child,
			share,
			sampleConfidence(baselineCount, baselineCount),
			[]Evidence{{
				Summary: fmt.Sprintf(
					"Na baseline, %d de %d spans de %s apareceram ligados a spans de %s no mesmo trace; no incidente essa ligação não foi observada nenhuma vez, embora os dois serviços continuem emitindo telemetria.",
					baselineCount,
					baselineChildSpans[link.child],
					link.child,
					link.parent,
				),
				SignalIDs:     signalIDs(incidentSpansOf(incident, link.child)),
				BaselineValue: float64(baselineCount),
				IncidentValue: 0,
			}},
			baselineCount,
			baselineCount,
		)
		finding.Limitations = append(finding.Limitations,
			"A ausência da ligação pode vir de mudança de instrumentação ou de coleta incompleta, e não comprova perda de contexto em produção.",
		)
		findings = append(findings, finding)
	}
	return findings
}

// serviceLink representa uma chamada observada entre dois serviços: um span do
// serviço filho tendo como pai um span do serviço pai, no mesmo trace.
type serviceLink struct {
	parent string
	child  string
}

// parentChildLinks conta quantos spans ligaram cada par de serviços na janela.
// Só entram pares em que o pai foi de fato encontrado entre os sinais carregados,
// já que um pai ausente é indistinguível de um span cuja origem não foi coletada.
func parentChildLinks(signals []domain.Signal) map[serviceLink]int {
	spanByID := indexSpansByID(signals)

	links := make(map[serviceLink]int)
	for _, signal := range signals {
		parentID := strings.TrimSpace(signal.Attributes["span.parent_id"])
		if parentID == "" {
			continue
		}
		parent, found := spanByID[spanKey(signal.TraceID, parentID)]
		if !found || parent.ServiceName == signal.ServiceName {
			continue
		}
		links[serviceLink{parent: parent.ServiceName, child: signal.ServiceName}]++
	}
	return links
}

func servicesWithSignals(signals []domain.Signal) map[string]struct{} {
	services := make(map[string]struct{})
	for _, signal := range signals {
		services[signal.ServiceName] = struct{}{}
	}
	return services
}

// spanCountByService informa quantos spans cada serviço emitiu na janela, base
// para medir o peso relativo de uma ligação perdida.
func spanCountByService(signals []domain.Signal) map[string]int {
	counts := make(map[string]int)
	for _, signal := range signals {
		counts[signal.ServiceName]++
	}
	return counts
}

// incidentSpansOf devolve os spans do serviço no incidente, que são a
// proveniência observável de uma ligação que deixou de existir.
func incidentSpansOf(signals []domain.Signal, serviceName string) []domain.Signal {
	matched := make([]domain.Signal, 0, len(signals))
	for _, signal := range signals {
		if signal.ServiceName == serviceName {
			matched = append(matched, signal)
		}
	}
	return matched
}
