package terminal

import (
	"bytes"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/detection"
)

// TestRenderDiagnosisApresentaRegrasDeFormaHumanaEAuditavel garante que o relatório seja legível sem ocultar os identificadores técnicos.
func TestRenderDiagnosisApresentaRegrasDeFormaHumanaEAuditavel(t *testing.T) {
	t.Parallel()

	findings := []detection.Finding{
		{
			Rule:       detection.RuleLatencyDelta,
			Score:      0.95,
			Confidence: detection.ConfidenceHigh,
			Evidence: []detection.Evidence{{
				Summary: "A duração p95 HTTP aumentou de 120 ms para 2300 ms.",
			}},
		},
		{
			Rule:       detection.RuleErrorRateDelta,
			Score:      1,
			Confidence: detection.ConfidenceHigh,
			Evidence: []detection.Evidence{{
				Summary: "A taxa de erro aumentou de 0.00% para 40.00%.",
			}},
		},
		{
			Rule:       detection.RuleDatabaseTimeout,
			Score:      1,
			Confidence: detection.ConfidenceHigh,
			Evidence: []detection.Evidence{{
				Summary: "Foram observados 6 erros no banco PostgreSQL.",
			}},
		},
	}

	result := renderDiagnosisForTest(t, findings)
	for _, expected := range []string{
		"Diagnóstico do incidente — checkout-service",
		"Baseline: 20 sinais · Incidente: 20 sinais",
		"Timeout no PostgreSQL",
		"ID da regra: database_timeout",
		"Aumento da taxa de erros HTTP",
		"ID da regra: error_rate_delta",
		"Aumento da latência HTTP",
		"ID da regra: latency_delta",
		"Score: 1.00",
		"Confiança: alta",
		"Evidência: A taxa de erro aumentou de 0.00% para 40.00%.",
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("saída não contém %q:\n%s", expected, result)
		}
	}
}

// TestRenderDiagnosisConsolidaLimitacoesRepetidas mantém limitações gerais uma única vez e preserva as específicas junto de sua hipótese.
func TestRenderDiagnosisConsolidaLimitacoesRepetidas(t *testing.T) {
	t.Parallel()

	findings := []detection.Finding{
		{
			Rule:        detection.RuleErrorRateDelta,
			Score:       1,
			Confidence:  detection.ConfidenceLow,
			Limitations: []string{"Correlação não comprova causalidade.", "Amostra HTTP pequena."},
		},
		{
			Rule:        detection.RuleDatabaseTimeout,
			Score:       1,
			Confidence:  detection.ConfidenceLow,
			Limitations: []string{"Diagnóstico baseado apenas na telemetria recebida.", "Correlação não comprova causalidade."},
		},
		{
			Rule:        detection.RuleLatencyDelta,
			Score:       0.8,
			Confidence:  detection.ConfidenceLow,
			Limitations: []string{"Diagnóstico baseado apenas na telemetria recebida.", "Amostra HTTP pequena."},
		},
	}

	result := renderDiagnosisForTest(t, findings)
	for _, limitation := range []string{
		"Correlação não comprova causalidade.",
		"Diagnóstico baseado apenas na telemetria recebida.",
		"Amostra HTTP pequena.",
	} {
		if occurrences := strings.Count(result, limitation); occurrences != 1 {
			t.Errorf("limitação %q aparece %d vezes; esperado 1:\n%s", limitation, occurrences, result)
		}
	}
	if !strings.Contains(result, "Limitações gerais:\n- Amostra HTTP pequena.\n- Correlação não comprova causalidade.\n- Diagnóstico baseado apenas na telemetria recebida.") {
		t.Errorf("limitações gerais não foram ordenadas e consolidadas:\n%s", result)
	}
}

// TestRenderDiagnosisMantemLimitacaoEspecificaNaHipotese garante que uma ressalva exclusiva não perca seu contexto.
func TestRenderDiagnosisMantemLimitacaoEspecificaNaHipotese(t *testing.T) {
	t.Parallel()

	result := renderDiagnosisForTest(t, []detection.Finding{{
		Rule:        detection.RuleDatabaseTimeout,
		Score:       0.7,
		Confidence:  detection.ConfidenceLow,
		Limitations: []string{"Não foi possível identificar a operação SQL."},
	}})

	if !strings.Contains(result, "  Limitação específica: Não foi possível identificar a operação SQL.") {
		t.Errorf("limitação específica não permaneceu junto da hipótese:\n%s", result)
	}
	if strings.Contains(result, "Limitações gerais:") {
		t.Errorf("limitação exclusiva foi tratada como geral:\n%s", result)
	}
}

// TestRenderDiagnosisOrdenaHipotesesDeterministicamente protege a estabilidade da saída para a mesma coleção de findings.
func TestRenderDiagnosisOrdenaHipotesesDeterministicamente(t *testing.T) {
	t.Parallel()

	result := renderDiagnosisForTest(t, []detection.Finding{
		{Rule: detection.RuleLatencyDelta, Score: 0.5, Confidence: detection.ConfidenceLow},
		{Rule: detection.RuleErrorRateDelta, Score: 0.9, Confidence: detection.ConfidenceLow},
		{Rule: detection.RuleDatabaseTimeout, Score: 0.9, Confidence: detection.ConfidenceLow},
	})

	databasePosition := strings.Index(result, "ID da regra: database_timeout")
	errorPosition := strings.Index(result, "ID da regra: error_rate_delta")
	latencyPosition := strings.Index(result, "ID da regra: latency_delta")
	if databasePosition < 0 || errorPosition < 0 || latencyPosition < 0 || !(databasePosition < errorPosition && errorPosition < latencyPosition) {
		t.Errorf("hipóteses não estão ordenadas por score e ID da regra:\n%s", result)
	}
}

func renderDiagnosisForTest(t *testing.T, findings []detection.Finding) string {
	t.Helper()

	var output bytes.Buffer
	if err := RenderDiagnosis(&output, "checkout-service", 20, 20, findings); err != nil {
		t.Fatalf("RenderDiagnosis() erro = %v", err)
	}
	return output.String()
}
