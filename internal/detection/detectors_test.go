package detection

import (
	"fmt"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

func TestRunDetectsRelevantChanges(t *testing.T) {
	t.Parallel()

	input := Input{
		ServiceName: "checkout-service",
		Baseline: append(
			httpSignals("baseline", 10, 0, 100),
			databaseTimeoutSignals("baseline-database", 5, 0, 50)...,
		),
		Incident: append(
			httpSignals("incident", 10, 6, 800),
			databaseTimeoutSignals("database", 5, 3, 1_500)...,
		),
	}

	findings := Run(input)

	assertFinding(t, findings, RuleErrorRateDelta, ConfidenceHigh)
	assertFinding(t, findings, RuleLatencyDelta, ConfidenceHigh)
	databaseFinding := assertFinding(t, findings, RuleDatabaseTimeout, ConfidenceHigh)
	if len(databaseFinding.Evidence) == 0 {
		t.Fatal("detector de banco deveria incluir evidências")
	}
	for _, finding := range findings {
		if finding.ServiceName != "checkout-service" {
			t.Fatalf("serviço inesperado: %q", finding.ServiceName)
		}
		if finding.Score < 0 || finding.Score > 1 {
			t.Fatalf("score fora do intervalo [0,1]: %f", finding.Score)
		}
		if !contains(finding.Limitations, "não comprova causalidade") {
			t.Fatalf("finding %q deve declarar que não comprova causalidade: %#v", finding.Rule, finding.Limitations)
		}
	}
}

func TestRunDoesNotCreateFindingsWithoutEvidence(t *testing.T) {
	t.Parallel()

	findings := Run(Input{
		ServiceName: "checkout-service",
		Baseline:    httpSignals("baseline", 10, 0, 100),
		Incident: append(
			httpSignals("incident", 10, 0, 100),
			databaseTimeoutSignals("database", 5, 0, 100)...,
		),
	})

	if len(findings) != 0 {
		t.Fatalf("esperava nenhum finding sem evidência, recebeu %#v", findings)
	}
}

func TestRunReducesConfidenceForSmallSamples(t *testing.T) {
	t.Parallel()

	findings := Run(Input{
		ServiceName: "checkout-service",
		Baseline:    httpSignals("baseline", 1, 0, 100),
		Incident: append(
			httpSignals("incident", 1, 1, 2_000),
			databaseTimeoutSignals("database", 1, 1, 1_900)...,
		),
	})

	for _, rule := range []string{RuleErrorRateDelta, RuleLatencyDelta, RuleDatabaseTimeout} {
		finding := assertFinding(t, findings, rule, ConfidenceLow)
		if !contains(finding.Limitations, "amostra pequena") {
			t.Fatalf("finding %q deveria declarar amostra pequena: %#v", rule, finding.Limitations)
		}
	}
}

// TestRunExplicaProtocolosEStatus torna a evidência terminal verificável por pessoas.
func TestRunExplicaProtocolosEStatus(t *testing.T) {
	t.Parallel()

	findings := Run(Input{
		ServiceName: "checkout-service",
		Baseline: append(
			httpSignals("baseline", 1, 0, 120),
			databaseTimeoutSignals("baseline-database", 1, 0, 50)...,
		),
		Incident: append(
			httpSignals("incident", 1, 1, 2300),
			databaseTimeoutSignals("incident-database", 1, 1, 2100)...,
		),
	})

	errorRate := assertFinding(t, findings, RuleErrorRateDelta, ConfidenceLow)
	if !contains([]string{errorRate.Evidence[0].Summary}, "HTTP 500") {
		t.Fatalf("evidência de erro não informa HTTP 500: %q", errorRate.Evidence[0].Summary)
	}
	database := assertFinding(t, findings, RuleDatabaseTimeout, ConfidenceLow)
	if !contains([]string{database.Evidence[0].Summary}, "PostgreSQL") {
		t.Fatalf("evidência de banco não informa PostgreSQL: %q", database.Evidence[0].Summary)
	}
}

func assertFinding(t *testing.T, findings []Finding, rule string, confidence Confidence) Finding {
	t.Helper()
	for _, finding := range findings {
		if finding.Rule != rule {
			continue
		}
		if finding.Confidence != confidence {
			t.Fatalf("confiança de %q = %q, esperava %q", rule, finding.Confidence, confidence)
		}
		return finding
	}
	t.Fatalf("finding %q não encontrado em %#v", rule, findings)
	return Finding{}
}

func httpSignals(prefix string, count, errorCount int, durationMS float64) []domain.Signal {
	signals := make([]domain.Signal, 0, count)
	for index := range count {
		statusCode := "201"
		severity := "INFO"
		if index < errorCount {
			statusCode = "500"
			severity = "ERROR"
		}
		signals = append(signals, domain.Signal{
			ID:          fmt.Sprintf("%s-http-%d", prefix, index),
			ServiceName: "checkout-service",
			Severity:    severity,
			Attributes: map[string]string{
				"http.response.status_code": statusCode,
			},
			Measurements: map[string]float64{"duration_ms": durationMS},
		})
	}
	return signals
}

func databaseTimeoutSignals(prefix string, count, timeoutCount int, durationMS float64) []domain.Signal {
	signals := make([]domain.Signal, 0, count)
	for index := range count {
		attributes := map[string]string{
			"db.system.name":    "postgresql",
			"db.operation.name": "INSERT",
		}
		severity := "INFO"
		if index < timeoutCount {
			attributes["error.type"] = "timeout"
			severity = "ERROR"
		}
		signals = append(signals, domain.Signal{
			ID:           fmt.Sprintf("%s-db-%d", prefix, index),
			ServiceName:  "checkout-service",
			Severity:     severity,
			Attributes:   attributes,
			Measurements: map[string]float64{"duration_ms": durationMS},
		})
	}
	return signals
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), strings.ToLower(want)) {
			return true
		}
	}
	return false
}
