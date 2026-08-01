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
	if !contains([]string{database.Evidence[0].Summary}, "1 timeout observado") {
		t.Fatalf("evidência de banco não usa singular natural: %q", database.Evidence[0].Summary)
	}
}

// TestDatabaseTimeoutIgnoraErroGenerico impede que o detector confunda falhas sem timeout.
func TestDatabaseTimeoutIgnoraErroGenerico(t *testing.T) {
	t.Parallel()

	baseline := databaseTimeoutSignals("baseline", 5, 0, 50)
	incident := databaseTimeoutSignals("incident", 5, 0, 100)
	incident[0].Severity = "ERROR"
	incident[0].Attributes["error.type"] = "constraint_violation"

	_, found := DetectDatabaseTimeout(Input{
		ServiceName: "checkout-service",
		Baseline:    baseline,
		Incident:    incident,
	})
	if found {
		t.Fatal("DetectDatabaseTimeout() encontrou timeout em erro genérico")
	}
}

// TestTraceCorrelationRelacionaTimeoutEImpactoHTTPNoMesmoTrace protege a hipótese contra correlações apenas temporais.
func TestTraceCorrelationRelacionaTimeoutEImpactoHTTPNoMesmoTrace(t *testing.T) {
	t.Parallel()

	baseline := tracePairs("baseline", 5, false, false, 120)
	incident := tracePairs("incident", 5, true, true, 2_000)

	finding, found := DetectTraceCorrelation(Input{
		ServiceName: "checkout-service",
		Baseline:    baseline,
		Incident:    incident,
	})
	if !found {
		t.Fatal("DetectTraceCorrelation() não relacionou timeout e erro HTTP pertencentes ao mesmo trace")
	}
	if finding.Rule != RuleTraceCorrelation {
		t.Fatalf("regra = %q, esperava %q", finding.Rule, RuleTraceCorrelation)
	}
	if finding.Confidence != ConfidenceHigh {
		t.Fatalf("confiança = %q, esperava %q para cinco traces correlacionados", finding.Confidence, ConfidenceHigh)
	}
	if len(finding.Evidence) != 1 {
		t.Fatalf("evidências = %d, esperava uma evidência quantitativa consolidada", len(finding.Evidence))
	}
	evidence := finding.Evidence[0]
	if !contains([]string{evidence.Summary}, "5 de 5") {
		t.Fatalf("evidência não quantifica os traces correlacionados: %q", evidence.Summary)
	}
	if !contains([]string{evidence.Summary}, "mesmo trace") {
		t.Fatalf("evidência não explicita a correlação pelo mesmo trace_id: %q", evidence.Summary)
	}
	if !contains([]string{evidence.Summary}, "hipótese") {
		t.Fatalf("evidência deveria apresentar hipótese, sem afirmar causa: %q", evidence.Summary)
	}
	if !contains(finding.Limitations, "não comprova causalidade") {
		t.Fatalf("finding deveria preservar a limitação causal: %#v", finding.Limitations)
	}
}

// TestTraceCorrelationNaoRelacionaSinaisDeTracesDiferentes evita atribuir um erro HTTP a outro fluxo distribuído.
func TestTraceCorrelationNaoRelacionaSinaisDeTracesDiferentes(t *testing.T) {
	t.Parallel()

	incident := tracePairs("incident", 5, true, true, 2_000)
	for index := range incident {
		if incident[index].Attributes["db.system.name"] != "" {
			incident[index].TraceID = "database-" + incident[index].TraceID
		}
	}

	_, found := DetectTraceCorrelation(Input{
		ServiceName: "checkout-service",
		Baseline:    tracePairs("baseline", 5, false, false, 120),
		Incident:    incident,
	})
	if found {
		t.Fatal("DetectTraceCorrelation() correlacionou timeout e erro HTTP de trace_ids diferentes")
	}
}

// TestTraceCorrelationAceitaLatenciaAltaSemErroHTTP cobre impacto percebido mesmo quando a resposta termina com sucesso.
func TestTraceCorrelationAceitaLatenciaAltaSemErroHTTP(t *testing.T) {
	t.Parallel()

	finding, found := DetectTraceCorrelation(Input{
		ServiceName: "checkout-service",
		Baseline:    tracePairs("baseline", 5, false, false, 100),
		Incident:    tracePairs("incident", 5, true, false, 2_000),
	})
	if !found {
		t.Fatal("DetectTraceCorrelation() não relacionou timeout e alta latência HTTP no mesmo trace")
	}
	if finding.Rule != RuleTraceCorrelation {
		t.Fatalf("regra = %q, esperava %q", finding.Rule, RuleTraceCorrelation)
	}
}

// TestTraceCorrelationReduzConfiancaParaAmostraPequena deixa explícita a cautela estatística da hipótese.
func TestTraceCorrelationReduzConfiancaParaAmostraPequena(t *testing.T) {
	t.Parallel()

	finding, found := DetectTraceCorrelation(Input{
		ServiceName: "checkout-service",
		Baseline:    tracePairs("baseline", 1, false, false, 100),
		Incident:    tracePairs("incident", 1, true, true, 2_000),
	})
	if !found {
		t.Fatal("DetectTraceCorrelation() deveria produzir hipótese para um trace correlacionado")
	}
	if finding.Confidence != ConfidenceLow {
		t.Fatalf("confiança = %q, esperava %q para um trace correlacionado", finding.Confidence, ConfidenceLow)
	}
	if !contains(finding.Limitations, "amostra pequena") {
		t.Fatalf("finding deveria declarar amostra pequena: %#v", finding.Limitations)
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

// tracePairs cria pares HTTP/PostgreSQL com trace_id compartilhado para exercitar a correlação sem depender da ordem dos spans.
func tracePairs(prefix string, count int, databaseTimeout, httpError bool, httpDurationMS float64) []domain.Signal {
	signals := make([]domain.Signal, 0, count*2)
	for index := range count {
		traceID := fmt.Sprintf("%s-trace-%d", prefix, index)
		statusCode := "201"
		httpSeverity := "INFO"
		if httpError {
			statusCode = "500"
			httpSeverity = "ERROR"
		}
		databaseAttributes := map[string]string{
			"db.system.name":    "postgresql",
			"db.operation.name": "INSERT",
		}
		databaseSeverity := "INFO"
		if databaseTimeout {
			databaseAttributes["error.type"] = "timeout"
			databaseSeverity = "ERROR"
		}
		signals = append(signals,
			domain.Signal{
				ID:           fmt.Sprintf("%s-http-%d", prefix, index),
				ServiceName:  "checkout-service",
				TraceID:      traceID,
				Severity:     httpSeverity,
				Attributes:   map[string]string{"http.response.status_code": statusCode},
				Measurements: map[string]float64{"duration_ms": httpDurationMS},
			},
			domain.Signal{
				ID:           fmt.Sprintf("%s-database-%d", prefix, index),
				ServiceName:  "checkout-service",
				TraceID:      traceID,
				Severity:     databaseSeverity,
				Attributes:   databaseAttributes,
				Measurements: map[string]float64{"duration_ms": httpDurationMS - 20},
			},
		)
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
