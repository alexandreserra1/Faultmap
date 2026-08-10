package detection

import (
	"fmt"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// maxAncestorHops limita quantos saltos de span.parent_id o detector percorre.
// Telemetria defeituosa produz cadeias cíclicas ou absurdamente longas, e o
// limite mantém o custo previsível mesmo nesses casos.
const maxAncestorHops = 32

// DetectDependencyFailure procura, dentro de um mesmo trace, spans que falharam
// tendo um ancestral de outro serviço também em falha.
//
// Recebe os sinais completos das duas janelas, de todos os serviços do escopo,
// porque a regra só existe na relação entre serviços — por isso não participa de
// Run, que enxerga um serviço por vez.
//
// O finding é atribuído ao serviço mais profundo do par, candidato à origem, e
// só é emitido quando a propagação aumentou em relação à baseline: um sistema em
// que a falha sempre atravessa a cadeia não descreve nada de novo sobre o
// incidente.
func DetectDependencyFailure(baseline, incident []domain.Signal) []Finding {
	baselinePropagation := propagationByService(baseline)
	incidentPropagation := propagationByService(incident)

	services := make([]string, 0, len(incidentPropagation))
	for service := range incidentPropagation {
		services = append(services, service)
	}
	// A ordenação impede que a iteração sobre o mapa vaze para a saída.
	sort.Strings(services)

	findings := make([]Finding, 0, len(services))
	for _, service := range services {
		observed := incidentPropagation[service]
		if len(observed.propagatedTraces) == 0 {
			continue
		}
		previous := baselinePropagation[service]
		baselineRate := fraction(len(previous.propagatedTraces), previous.totalTraces)
		incidentRate := fraction(len(observed.propagatedTraces), observed.totalTraces)
		if incidentRate <= baselineRate {
			continue
		}

		traceIDs := sortedSetValues(observed.propagatedTraces)
		upstreams := sortedSetValues(observed.upstreamServices)
		finding := newFinding(
			RuleDependencyFailure,
			service,
			incidentRate-baselineRate,
			sampleConfidence(previous.totalTraces, observed.totalTraces),
			[]Evidence{{
				Summary: fmt.Sprintf(
					"Em %d de %d traces com spans de %s, uma falha deste serviço aparece sob uma falha de %s (traces %s); na baseline isso ocorreu em %d de %d traces.",
					len(observed.propagatedTraces),
					observed.totalTraces,
					service,
					strings.Join(upstreams, ", "),
					strings.Join(limitTraceIDs(traceIDs), ", "),
					len(previous.propagatedTraces),
					previous.totalTraces,
				),
				SignalIDs:     observed.signalIDs,
				BaselineValue: baselineRate,
				IncidentValue: incidentRate,
			}},
			previous.totalTraces,
			observed.totalTraces,
		)
		// A ordem dos spans em um trace mostra quem chamou quem, e nada além
		// disso: a falha de baixo pode ser consequência da de cima, ou ambas
		// podem ter uma terceira origem.
		finding.Limitations = append(
			finding.Limitations,
			"A topologia observada nos traces indica a ordem das chamadas, não a causa da falha.",
		)
		findings = append(findings, finding)
	}
	return findings
}

// propagationSummary acumula, por serviço, quantos traces o contêm e em quantos
// deles uma falha sua aparece sob a falha de um serviço acima na cadeia.
type propagationSummary struct {
	totalTraces      int
	propagatedTraces map[string]struct{}
	upstreamServices map[string]struct{}
	signalIDs        []string
}

// propagationByService percorre cada span em falha subindo por span.parent_id
// até encontrar ancestrais de outros serviços que também falharam.
func propagationByService(signals []domain.Signal) map[string]propagationSummary {
	spanByID := indexSpansByID(signals)

	tracesByService := make(map[string]map[string]struct{})
	summaries := make(map[string]propagationSummary)
	for _, signal := range signals {
		traceID := strings.TrimSpace(signal.TraceID)
		if traceID == "" {
			continue
		}
		if tracesByService[signal.ServiceName] == nil {
			tracesByService[signal.ServiceName] = make(map[string]struct{})
		}
		tracesByService[signal.ServiceName][traceID] = struct{}{}
	}

	// A varredura segue a ordem do fatiamento de entrada, que a camada de
	// aplicação já entrega estável, de modo que os SignalIDs saiam reproduzíveis.
	for _, signal := range signals {
		if !isFailedSpan(signal) || strings.TrimSpace(signal.TraceID) == "" {
			continue
		}
		upstreams := failingAncestorServices(signal, spanByID)
		if len(upstreams) == 0 {
			continue
		}
		summary := summaries[signal.ServiceName]
		if summary.propagatedTraces == nil {
			summary.propagatedTraces = make(map[string]struct{})
			summary.upstreamServices = make(map[string]struct{})
		}
		summary.propagatedTraces[strings.TrimSpace(signal.TraceID)] = struct{}{}
		for upstream := range upstreams {
			summary.upstreamServices[upstream] = struct{}{}
		}
		summary.signalIDs = append(summary.signalIDs, signal.ID)
		summaries[signal.ServiceName] = summary
	}

	// Todo serviço observado na janela entra no resultado, mesmo sem propagação
	// nenhuma. Sem isso, a baseline saudável — que é o caso normal — ficava sem
	// denominador, a evidência dizia "0 de 0 traces" como se o serviço nunca
	// tivesse sido visto, e a confiança do finding era rebaixada por falta de
	// amostra que na verdade existia.
	for service, traces := range tracesByService {
		summary := summaries[service]
		summary.totalTraces = len(traces)
		summaries[service] = summary
	}
	return summaries
}

// failingAncestorServices sobe a cadeia de um span em falha e devolve os
// serviços distintos, diferentes do próprio, cujos spans ancestrais também
// falharam. Spans já visitados encerram a subida: telemetria com parent_id
// cíclico não pode transformar a leitura em laço infinito.
func failingAncestorServices(signal domain.Signal, spanByID map[string]domain.Signal) map[string]struct{} {
	services := make(map[string]struct{})
	visited := map[string]struct{}{spanKey(signal.TraceID, signal.SpanID): {}}
	current := signal
	for hop := 0; hop < maxAncestorHops; hop++ {
		parentID := strings.TrimSpace(current.Attributes["span.parent_id"])
		if parentID == "" {
			break
		}
		key := spanKey(current.TraceID, parentID)
		parent, found := spanByID[key]
		if !found {
			break
		}
		if _, seen := visited[key]; seen {
			break
		}
		visited[key] = struct{}{}
		if parent.ServiceName != signal.ServiceName && isFailedSpan(parent) {
			services[parent.ServiceName] = struct{}{}
		}
		current = parent
	}
	return services
}

// isFailedSpan trata a severidade de erro como a marca de falha do span, que é o
// que a normalização registra tanto para status de erro quanto para exceções.
func isFailedSpan(signal domain.Signal) bool {
	return strings.EqualFold(strings.TrimSpace(signal.Severity), "error")
}

// indexSpansByID permite alcançar o pai de um span sem varrer a lista inteira.
// A chave inclui o trace para que dois traces com o mesmo span_id — possível com
// geradores defeituosos — não se misturem.
func indexSpansByID(signals []domain.Signal) map[string]domain.Signal {
	spanByID := make(map[string]domain.Signal, len(signals))
	for _, signal := range signals {
		if strings.TrimSpace(signal.SpanID) == "" {
			continue
		}
		spanByID[spanKey(signal.TraceID, signal.SpanID)] = signal
	}
	return spanByID
}

func spanKey(traceID, spanID string) string {
	return traceID + "\x00" + spanID
}

// limitTraceIDs mantém a evidência legível quando a propagação atinge muitos
// traces, sem esconder o total, que o resumo já informa.
func limitTraceIDs(traceIDs []string) []string {
	const maxListedTraces = 3
	if len(traceIDs) <= maxListedTraces {
		return traceIDs
	}
	return traceIDs[:maxListedTraces]
}
