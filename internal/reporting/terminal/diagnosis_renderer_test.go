package terminal

import (
	"bytes"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/detection"
)

// TestRenderDiagnosisExplicaEvidênciasELimitações garante uma saída auditável para investigação humana.
func TestRenderDiagnosisExplicaEvidênciasELimitações(t *testing.T) {
	t.Parallel()

	findings := []detection.Finding{
		{
			Rule:        detection.RuleErrorRateDelta,
			ServiceName: "checkout-service",
			Score:       1,
			Confidence:  detection.ConfidenceLow,
			Evidence: []detection.Evidence{{
				Summary: "Foram observadas respostas HTTP 500; taxa de erro aumentou de 0.00% para 100.00%.",
			}},
			Limitations: []string{"Amostra pequena.", "Correlação não comprova causalidade."},
		},
		{
			Rule:        detection.RuleDatabaseTimeout,
			ServiceName: "checkout-service",
			Score:       1,
			Confidence:  detection.ConfidenceLow,
			Evidence: []detection.Evidence{{
				Summary: "Foram observados 1 timeout(s) no banco PostgreSQL.",
			}},
			Limitations: []string{"Amostra pequena."},
		},
	}

	var output bytes.Buffer
	if err := RenderDiagnosis(&output, "checkout-service", 2, 2, findings); err != nil {
		t.Fatalf("RenderDiagnosis() erro = %v", err)
	}

	for _, expected := range []string{
		"Diagnóstico do incidente — checkout-service",
		"Baseline: 2 sinais · Incidente: 2 sinais",
		"error_rate_delta",
		"HTTP 500",
		"database_timeout",
		"PostgreSQL",
		"Confiança: baixa",
		"Limitação: Amostra pequena.",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("saída não contém %q:\n%s", expected, output.String())
		}
	}
}
