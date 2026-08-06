package detection

import (
	"fmt"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

func TestDetectRetryStormComparaTentativasPorTrace(t *testing.T) {
	t.Parallel()

	finding, found := DetectRetryStorm(Input{
		ServiceName: "checkout-service",
		Baseline:    retrySignals("baseline", 5, 1, "POST", "/payment"),
		Incident:    retrySignals("incident", 5, 4, "POST", "/payment"),
	})
	if !found {
		t.Fatal("DetectRetryStorm() não detectou aumento de uma para quatro tentativas por trace")
	}
	if finding.Rule != RuleRetryStorm {
		t.Fatalf("regra = %q, esperava %q", finding.Rule, RuleRetryStorm)
	}
	if finding.Confidence != ConfidenceHigh {
		t.Fatalf("confiança = %q, esperava %q com cinco traces em cada janela", finding.Confidence, ConfidenceHigh)
	}
	if finding.Score != 0.75 {
		t.Fatalf("score = %.2f, esperava 0.75 para crescimento de 1 para 4 tentativas", finding.Score)
	}
	if len(finding.Evidence) != 1 {
		t.Fatalf("evidências = %d, esperava uma consolidação da operação", len(finding.Evidence))
	}
	evidence := finding.Evidence[0]
	if evidence.BaselineValue != 1 || evidence.IncidentValue != 4 {
		t.Fatalf("médias baseline/incidente = %.2f/%.2f, esperava 1/4", evidence.BaselineValue, evidence.IncidentValue)
	}
	if len(evidence.SignalIDs) != 20 {
		t.Fatalf("IDs de evidência = %d, esperava os 20 spans repetidos no incidente", len(evidence.SignalIDs))
	}
	for _, fragment := range []string{"POST /payment", "5 de 5 traces", "1.00", "4.00"} {
		if !strings.Contains(evidence.Summary, fragment) {
			t.Fatalf("evidência %q não contém %q", evidence.Summary, fragment)
		}
	}
	if !contains(finding.Limitations, "não comprova causalidade") {
		t.Fatalf("limitação causal ausente: %#v", finding.Limitations)
	}
	if !contains(finding.Limitations, "fan-out") {
		t.Fatalf("limitação sobre repetições legítimas ausente: %#v", finding.Limitations)
	}
}

func TestDetectRetryStormIgnoraTraceIDVazioESpansServidor(t *testing.T) {
	t.Parallel()

	incident := retrySignals("incident", 5, 4, "POST", "/payment")
	for index := range incident {
		incident[index].TraceID = ""
	}
	incident = append(incident, serverRetrySignals("server", 5, 4)...)

	_, found := DetectRetryStorm(Input{
		ServiceName: "checkout-service",
		Baseline:    retrySignals("baseline", 5, 1, "POST", "/payment"),
		Incident:    incident,
	})
	if found {
		t.Fatal("DetectRetryStorm() associou spans sem trace_id ou contou requisições recebidas como retries")
	}
}

func TestDetectRetryStormUsaSomenteIdentidadeSegura(t *testing.T) {
	t.Parallel()

	baseline := databaseRetrySignals("baseline", 5, 1)
	incident := databaseRetrySignals("incident", 5, 3)
	for index := range incident {
		incident[index].Attributes["db.statement"] = fmt.Sprintf("SELECT segredo_%d FROM payments", index)
	}

	finding, found := DetectRetryStorm(Input{ServiceName: "checkout-service", Baseline: baseline, Incident: incident})
	if !found {
		t.Fatal("DetectRetryStorm() deveria agrupar operações de banco pela identidade permitida")
	}
	if strings.Contains(finding.Evidence[0].Summary, "segredo") || strings.Contains(finding.Evidence[0].Summary, "SELECT") {
		t.Fatalf("evidência vazou SQL bruto: %q", finding.Evidence[0].Summary)
	}
	if !strings.Contains(finding.Evidence[0].Summary, "PostgreSQL INSERT payments") {
		t.Fatalf("evidência não descreve a assinatura segura: %q", finding.Evidence[0].Summary)
	}
}

func TestDetectRetryStormExigeVolumeMinimoEIntegracaoNoRun(t *testing.T) {
	t.Parallel()

	inputPequeno := Input{
		ServiceName: "checkout-service",
		Baseline:    retrySignals("baseline-small", 2, 1, "POST", "/payment"),
		Incident:    retrySignals("incident-small", 2, 5, "POST", "/payment"),
	}
	if _, found := DetectRetryStorm(inputPequeno); found {
		t.Fatal("DetectRetryStorm() deveria exigir ao menos três traces comparáveis por janela")
	}

	input := Input{
		ServiceName: "checkout-service",
		Baseline:    retrySignals("baseline", 5, 1, "POST", "/payment"),
		Incident:    retrySignals("incident", 5, 3, "POST", "/payment"),
	}
	assertFinding(t, Run(input), RuleRetryStorm, ConfidenceHigh)
}

func TestDetectRetryStormNaoSinalizaPadraoEstavel(t *testing.T) {
	t.Parallel()

	_, found := DetectRetryStorm(Input{
		ServiceName: "checkout-service",
		Baseline:    retrySignals("baseline", 5, 2, "POST", "/payment"),
		Incident:    retrySignals("incident", 5, 2, "POST", "/payment"),
	})
	if found {
		t.Fatal("DetectRetryStorm() sinalizou uma quantidade estável de tentativas")
	}
}

func retrySignals(prefix string, traceCount, attempts int, method, route string) []domain.Signal {
	signals := make([]domain.Signal, 0, traceCount*attempts)
	for traceIndex := range traceCount {
		for attempt := range attempts {
			signals = append(signals, domain.Signal{
				ID:          fmt.Sprintf("%s-%d-%d", prefix, traceIndex, attempt),
				ServiceName: "checkout-service",
				TraceID:     fmt.Sprintf("%s-trace-%d", prefix, traceIndex),
				Attributes: map[string]string{
					"span.kind":           "SPAN_KIND_CLIENT",
					"http.request.method": method,
					"http.route":          route,
				},
			})
		}
	}
	return signals
}

func serverRetrySignals(prefix string, traceCount, attempts int) []domain.Signal {
	signals := retrySignals(prefix, traceCount, attempts, "POST", "/checkout")
	for index := range signals {
		signals[index].Attributes["span.kind"] = "SPAN_KIND_SERVER"
	}
	return signals
}

func databaseRetrySignals(prefix string, traceCount, attempts int) []domain.Signal {
	signals := make([]domain.Signal, 0, traceCount*attempts)
	for traceIndex := range traceCount {
		for attempt := range attempts {
			signals = append(signals, domain.Signal{
				ID:          fmt.Sprintf("%s-db-%d-%d", prefix, traceIndex, attempt),
				ServiceName: "checkout-service",
				TraceID:     fmt.Sprintf("%s-trace-%d", prefix, traceIndex),
				Attributes: map[string]string{
					"span.kind":          "SPAN_KIND_CLIENT",
					"db.system.name":     "postgresql",
					"db.operation.name":  "INSERT",
					"db.collection.name": "payments",
				},
			})
		}
	}
	return signals
}
