package detection

import (
	"fmt"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
	"github.com/faultmap/faultmap/internal/telemetry/semconv"
)

const minimumRetryTraceCount = 3

type retryOperationStats struct {
	label       string
	attempts    map[string]int
	signalIDs   map[string][]string
	total       int
	repeated    int
	traceCount  int
	average     float64
	repeatedIDs []string
}

type retryCandidate struct {
	signature string
	baseline  retryOperationStats
	incident  retryOperationStats
	score     float64
}

// DetectRetryStorm compara a média de chamadas equivalentes por trace e exige
// repetição distribuída entre vários traces para reduzir falsos positivos.
func DetectRetryStorm(input Input) (Finding, bool) {
	baseline := retryStats(input.Baseline)
	incident := retryStats(input.Incident)

	var selected retryCandidate
	found := false
	for signature, incidentStats := range incident {
		baselineStats, comparable := baseline[signature]
		if !comparable || baselineStats.traceCount < minimumRetryTraceCount || incidentStats.traceCount < minimumRetryTraceCount {
			continue
		}
		if incidentStats.repeated < minimumRetryTraceCount || incidentStats.average < 2 || incidentStats.average-baselineStats.average < 1 {
			continue
		}

		score := clamp((incidentStats.average - baselineStats.average) / incidentStats.average)
		candidate := retryCandidate{
			signature: signature,
			baseline:  baselineStats,
			incident:  incidentStats,
			score:     score,
		}
		if !found || candidate.score > selected.score || (candidate.score == selected.score && candidate.signature < selected.signature) {
			selected = candidate
			found = true
		}
	}
	if !found {
		return Finding{}, false
	}

	finding := newFinding(
		RuleRetryStorm,
		input.ServiceName,
		selected.score,
		sampleConfidence(selected.baseline.traceCount, selected.incident.traceCount),
		[]Evidence{{
			Summary: fmt.Sprintf(
				"A operação %s foi repetida em %d de %d traces; a média de tentativas por trace aumentou de %.2f para %.2f.",
				selected.incident.label,
				selected.incident.repeated,
				selected.incident.traceCount,
				selected.baseline.average,
				selected.incident.average,
			),
			SignalIDs:     selected.incident.repeatedIDs,
			BaselineValue: selected.baseline.average,
			IncidentValue: selected.incident.average,
		}},
		selected.baseline.traceCount,
		selected.incident.traceCount,
	)
	finding.Limitations = append(finding.Limitations, "Spans semelhantes também podem representar fan-out, paginação, loops de negócio ou instrumentação duplicada.")
	return finding, true
}

// retryStats agrupa somente spans client com identidade construída a partir de
// uma allowlist; atributos livres, URL bruta e SQL nunca participam da chave.
func retryStats(signals []domain.Signal) map[string]retryOperationStats {
	stats := make(map[string]retryOperationStats)
	for _, signal := range signals {
		traceID := strings.TrimSpace(signal.TraceID)
		if traceID == "" || !strings.EqualFold(strings.TrimSpace(signal.Attributes["span.kind"]), "SPAN_KIND_CLIENT") {
			continue
		}
		signature, label, ok := safeRetryIdentity(signal.Attributes)
		if !ok {
			continue
		}

		operation := stats[signature]
		if operation.attempts == nil {
			operation = retryOperationStats{
				label:     label,
				attempts:  make(map[string]int),
				signalIDs: make(map[string][]string),
			}
		}
		operation.attempts[traceID]++
		operation.signalIDs[traceID] = append(operation.signalIDs[traceID], signal.ID)
		stats[signature] = operation
	}

	for signature, operation := range stats {
		for traceID, attempts := range operation.attempts {
			operation.total += attempts
			if attempts > 1 {
				operation.repeated++
				operation.repeatedIDs = append(operation.repeatedIDs, operation.signalIDs[traceID]...)
			}
		}
		operation.traceCount = len(operation.attempts)
		operation.average = fraction(operation.total, operation.traceCount)
		sort.Strings(operation.repeatedIDs)
		stats[signature] = operation
	}
	return stats
}

func safeRetryIdentity(attributes map[string]string) (string, string, bool) {
	if method := semconv.HTTPMethod(attributes); method != "" {
		target := semconv.HTTPTarget(attributes)
		if target == "" {
			return "", "", false
		}
		method = strings.ToUpper(method)
		return "http|" + strings.ToLower(method) + "|" + strings.ToLower(target), method + " " + target, true
	}

	if system := semconv.DatabaseSystem(attributes); system != "" {
		// A operação refina o rótulo, mas não é indispensável para reconhecer a
		// repetição: o sistema de banco já identifica a chamada. Exigi-la deixava
		// invisível toda tempestade de retry vinda de instrumentações que não a
		// emitem — a oficial do Node, por exemplo, coloca a operação no nome do
		// span e não em um atributo.
		operation := semconv.DatabaseOperation(attributes)
		collection := semconv.DatabaseCollection(attributes)
		labelSystem := system
		if strings.EqualFold(system, "postgresql") {
			labelSystem = "PostgreSQL"
		}
		label := strings.TrimSpace(labelSystem + " " + strings.ToUpper(operation) + " " + collection)
		return "db|" + strings.ToLower(system) + "|" + strings.ToLower(operation) + "|" + strings.ToLower(collection), label, true
	}

	if system := strings.TrimSpace(attributes["rpc.system"]); system != "" {
		method := strings.TrimSpace(attributes["rpc.method"])
		if method == "" {
			return "", "", false
		}
		service := strings.TrimSpace(attributes["rpc.service"])
		label := strings.TrimSpace(system + " " + service + "/" + method)
		return "rpc|" + strings.ToLower(system) + "|" + strings.ToLower(service) + "|" + strings.ToLower(method), label, true
	}

	if system := strings.TrimSpace(attributes["messaging.system"]); system != "" {
		operation := semconv.MessagingOperation(attributes)
		destination := semconv.MessagingDestination(attributes)
		if operation == "" || destination == "" {
			return "", "", false
		}
		label := strings.TrimSpace(system + " " + operation + " " + destination)
		return "messaging|" + strings.ToLower(system) + "|" + strings.ToLower(operation) + "|" + strings.ToLower(destination), label, true
	}

	return "", "", false
}
