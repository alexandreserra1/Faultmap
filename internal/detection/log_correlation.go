package detection

import (
	"fmt"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// DetectLogCorrelation identifica crescimento de logs de erro que compartilham
// trace com requisições que falharam.
//
// Um log de erro sozinho é uma contagem: ele pode vir de rotina de fundo, de
// migração ou de tarefa agendada, sem relação com o que a pessoa está
// investigando. Ligado ao trace de uma requisição que falhou, ele mostra que os
// dois sinais descrevem o mesmo evento — e é essa ligação, não o volume, que
// sustenta a hipótese.
//
// O texto da mensagem não participa e sequer chega aqui: o normalizador o
// descarta antes de persistir. A evidência descreve contagem e correlação, e
// remete ao sistema de logs de origem para quem precisar ler o conteúdo.
func DetectLogCorrelation(input Input) (Finding, bool) {
	baselineLogs := filterLogSignals(input.Baseline)
	incidentLogs := filterLogSignals(input.Incident)
	if len(baselineLogs) == 0 || len(incidentLogs) == 0 {
		return Finding{}, false
	}

	baselineRate := fraction(len(errorLogs(baselineLogs)), len(baselineLogs))
	incidentErrors := errorLogs(incidentLogs)
	incidentRate := fraction(len(incidentErrors), len(incidentLogs))
	delta := incidentRate - baselineRate
	if delta <= 0 {
		return Finding{}, false
	}
	if !exceedsSamplingNoise(baselineRate, len(baselineLogs), incidentRate, len(incidentLogs), delta) {
		return Finding{}, false
	}

	// A correlação é a razão de ser da regra: sem ela, o crescimento pode ser
	// trabalho de fundo que nada tem a ver com o incidente investigado.
	failingTraces := tracesWithFailedRequests(input.Incident)
	correlated := make([]domain.Signal, 0, len(incidentErrors))
	for _, signal := range incidentErrors {
		if _, failed := failingTraces[signal.TraceID]; failed && signal.TraceID != "" {
			correlated = append(correlated, signal)
		}
	}
	if len(correlated) == 0 {
		return Finding{}, false
	}

	return newFinding(
		RuleLogCorrelation,
		input.ServiceName,
		delta,
		sampleConfidence(len(baselineLogs), len(incidentLogs)),
		[]Evidence{{
			Summary: fmt.Sprintf(
				"%d de %d registros de log foram de erro, contra %d de %d na baseline; %d deles compartilham trace com requisições que falharam. O texto das mensagens não é armazenado.",
				len(incidentErrors), len(incidentLogs),
				len(errorLogs(baselineLogs)), len(baselineLogs),
				len(correlated),
			),
			SignalIDs:     signalIDs(correlated),
			BaselineValue: baselineRate,
			IncidentValue: incidentRate,
		}},
		len(baselineLogs),
		len(incidentLogs),
	), true
}

func filterLogSignals(signals []domain.Signal) []domain.Signal {
	filtered := make([]domain.Signal, 0, len(signals))
	for _, signal := range signals {
		if signal.Type == domain.SignalTypeLog {
			filtered = append(filtered, signal)
		}
	}
	return filtered
}

func errorLogs(signals []domain.Signal) []domain.Signal {
	filtered := make([]domain.Signal, 0, len(signals))
	for _, signal := range signals {
		if isFailedSpan(signal) {
			filtered = append(filtered, signal)
		}
	}
	return filtered
}

// tracesWithFailedRequests reúne os traces em que alguma requisição HTTP
// terminou em erro de servidor.
func tracesWithFailedRequests(signals []domain.Signal) map[string]struct{} {
	traces := make(map[string]struct{})
	for _, signal := range filterHTTPSignals(signals) {
		if signal.TraceID == "" {
			continue
		}
		if statusCode, err := httpStatusValue(signal); err == nil && statusCode >= 500 && statusCode <= 599 {
			traces[signal.TraceID] = struct{}{}
		}
	}
	return traces
}
