package detection

import (
	"fmt"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// versionStats resume o comportamento HTTP de uma única versão dentro da janela
// de incidente.
type versionStats struct {
	version   string
	signals   []domain.Signal
	count     int
	errorRate float64
	p95       float64
	hasP95    bool
}

// DetectVersionRegression compara duas versões do mesmo serviço que atenderam
// tráfego simultaneamente durante o incidente — o caso de rollout parcial ou
// canário.
//
// A comparação aqui é entre versões, não entre janelas, e é justamente isso que
// a distingue de deployment_proximity: lá a pergunta é se a versão mudou antes
// do incidente; aqui as duas versões convivem agora e a pergunta é se uma delas
// está pior que a outra atendendo o mesmo tráfego. A versão melhor cumpre o
// papel de referência que a baseline cumpre nos demais detectores, então um
// serviço com falha crônica igual nas duas versões não gera hipótese.
func DetectVersionRegression(input Input) (Finding, bool) {
	versions := comparableVersionStats(input.Incident)
	if len(versions) < 2 {
		return Finding{}, false
	}

	// Erro é o sinal mais direto de regressão, então ele decide primeiro; a
	// latência só é consultada quando as taxas de erro não se distinguem do ruído.
	if reference, candidate, ok := worstVersionByErrorRate(versions); ok {
		delta := candidate.errorRate - reference.errorRate
		if exceedsSamplingNoise(reference.errorRate, reference.count, candidate.errorRate, candidate.count, delta) {
			return versionFinding(
				input.ServiceName,
				reference,
				candidate,
				delta,
				fmt.Sprintf(
					"A versão %s apresentou taxa de erro HTTP de %.2f%% em %d requisições, contra %.2f%% em %d requisições da versão %s, na mesma janela de incidente.",
					candidate.version, candidate.errorRate*100, candidate.count,
					reference.errorRate*100, reference.count, reference.version,
				),
				reference.errorRate,
				candidate.errorRate,
			), true
		}
	}

	if reference, candidate, ok := worstVersionByLatency(versions); ok {
		if exceedsLatencyNoise(reference.p95, candidate.p95) {
			return versionFinding(
				input.ServiceName,
				reference,
				candidate,
				(candidate.p95-reference.p95)/candidate.p95,
				fmt.Sprintf(
					"A versão %s apresentou latência p95 de %.0f ms em %d requisições, contra %.0f ms em %d requisições da versão %s, na mesma janela de incidente.",
					candidate.version, candidate.p95, candidate.count,
					reference.p95, reference.count, reference.version,
				),
				reference.p95,
				candidate.p95,
			), true
		}
	}

	return Finding{}, false
}

// versionFinding centraliza as limitações próprias desta regra: comparar versões
// só é honesto se ficar dito que o tráfego de cada uma pode não ser equivalente.
func versionFinding(serviceName string, reference, candidate versionStats, score float64, summary string, baselineValue, incidentValue float64) Finding {
	finding := newFinding(
		RuleVersionRegression,
		serviceName,
		score,
		sampleConfidence(reference.count, candidate.count),
		[]Evidence{{
			Summary:       summary,
			SignalIDs:     signalIDs(candidate.signals),
			BaselineValue: baselineValue,
			IncidentValue: incidentValue,
		}},
		reference.count,
		candidate.count,
	)
	finding.Limitations = append(
		finding.Limitations,
		fmt.Sprintf("As versões %s e %s podem ter recebido tráfego de perfis diferentes; o roteamento do rollout não é observado pelo Faultmap.", reference.version, candidate.version),
	)
	return finding
}

// comparableVersionStats devolve as versões em ordem estável e descarta as que
// não têm volume suficiente. Um canário com três requisições não sustenta
// conclusão nenhuma: qualquer falha nele já valeria uma taxa enorme.
func comparableVersionStats(signals []domain.Signal) []versionStats {
	grouped := make(map[string][]domain.Signal)
	for _, signal := range filterHTTPSignals(signals) {
		version := strings.TrimSpace(signal.Attributes["service.version"])
		if version == "" {
			continue
		}
		grouped[version] = append(grouped[version], signal)
	}

	stats := make([]versionStats, 0, len(grouped))
	for version, versionSignals := range grouped {
		if len(versionSignals) < minimumSampleSize {
			continue
		}
		p95, hasP95 := percentile95(versionSignals)
		stats = append(stats, versionStats{
			version:   version,
			signals:   versionSignals,
			count:     len(versionSignals),
			errorRate: errorRate(versionSignals),
			p95:       p95,
			hasP95:    hasP95,
		})
	}
	// A ordenação por nome de versão é o que impede a ordem de iteração do mapa
	// de escolher qual par será comparado quando houver empate.
	sort.Slice(stats, func(first, second int) bool {
		return stats[first].version < stats[second].version
	})
	return stats
}

// worstVersionByErrorRate escolhe o par mais informativo: a versão com maior
// taxa de erro contra a de menor taxa, que serve de referência do que o serviço
// consegue entregar agora.
func worstVersionByErrorRate(versions []versionStats) (versionStats, versionStats, bool) {
	best, worst := versions[0], versions[0]
	for _, current := range versions[1:] {
		if current.errorRate < best.errorRate {
			best = current
		}
		if current.errorRate > worst.errorRate {
			worst = current
		}
	}
	return best, worst, best.version != worst.version
}

// worstVersionByLatency repete o critério anterior sobre o p95, ignorando
// versões cujos spans não trazem duração medida.
func worstVersionByLatency(versions []versionStats) (versionStats, versionStats, bool) {
	measured := make([]versionStats, 0, len(versions))
	for _, current := range versions {
		if current.hasP95 {
			measured = append(measured, current)
		}
	}
	if len(measured) < 2 {
		return versionStats{}, versionStats{}, false
	}
	best, worst := measured[0], measured[0]
	for _, current := range measured[1:] {
		if current.p95 < best.p95 {
			best = current
		}
		if current.p95 > worst.p95 {
			worst = current
		}
	}
	return best, worst, best.version != worst.version
}
