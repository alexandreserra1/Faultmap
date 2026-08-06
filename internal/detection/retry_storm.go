package detection

import (
	"fmt"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
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
	if method := firstRetryAttribute(attributes, "http.request.method", "http.method"); method != "" {
		target := firstRetryAttribute(attributes, "http.route", "url.template", "server.address")
		if target == "" {
			return "", "", false
		}
		method = strings.ToUpper(method)
		return "http|" + strings.ToLower(method) + "|" + strings.ToLower(target), method + " " + target, true
	}

	if system := firstRetryAttribute(attributes, "db.system.name", "db.system"); system != "" {
		operation := firstRetryAttribute(attributes, "db.operation.name", "db.operation")
		if operation == "" {
			return "", "", false
		}
		collection := firstRetryAttribute(attributes, "db.collection.name", "db.sql.table")
		labelSystem := system
		if strings.EqualFold(system, "postgresql") {
			labelSystem = "PostgreSQL"
		}
		label := strings.TrimSpace(labelSystem + " " + strings.ToUpper(operation) + " " + collection)
		return "db|" + strings.ToLower(system) + "|" + strings.ToLower(operation) + "|" + strings.ToLower(collection), label, true
	}

	if system := firstRetryAttribute(attributes, "rpc.system"); system != "" {
		method := firstRetryAttribute(attributes, "rpc.method")
		if method == "" {
			return "", "", false
		}
		service := firstRetryAttribute(attributes, "rpc.service")
		label := strings.TrimSpace(system + " " + service + "/" + method)
		return "rpc|" + strings.ToLower(system) + "|" + strings.ToLower(service) + "|" + strings.ToLower(method), label, true
	}

	if system := firstRetryAttribute(attributes, "messaging.system"); system != "" {
		operation := firstRetryAttribute(attributes, "messaging.operation.name", "messaging.operation")
		destination := firstRetryAttribute(attributes, "messaging.destination.template", "messaging.destination.name")
		if operation == "" || destination == "" {
			return "", "", false
		}
		label := strings.TrimSpace(system + " " + operation + " " + destination)
		return "messaging|" + strings.ToLower(system) + "|" + strings.ToLower(operation) + "|" + strings.ToLower(destination), label, true
	}

	return "", "", false
}

func firstRetryAttribute(attributes map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(attributes[key]); value != "" {
			return value
		}
	}
	return ""
}
