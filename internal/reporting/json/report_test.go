package jsonreport

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

// TestRenderProduzContratoVersionadoEDeterministico protege a API pública do
// artefato contra nomes implícitos dos structs internos.
func TestRenderProduzContratoVersionadoEDeterministico(t *testing.T) {
	t.Parallel()

	diagnosis := reportDiagnosis()
	var first bytes.Buffer
	if err := Render(&first, diagnosis); err != nil {
		t.Fatalf("Render() erro = %v", err)
	}
	var second bytes.Buffer
	if err := Render(&second, diagnosis); err != nil {
		t.Fatalf("Render() segunda execução erro = %v", err)
	}
	if first.String() != second.String() {
		t.Fatalf("Render() não é determinístico:\n%s\n---\n%s", first.String(), second.String())
	}

	var document map[string]any
	if err := json.Unmarshal(first.Bytes(), &document); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, first.String())
	}
	if document["schema_version"] != "1" {
		t.Fatalf("schema_version = %#v, esperado 1", document["schema_version"])
	}
	for _, expected := range []string{
		`"service_name": "checkout-service"`,
		`"started_at": "2025-12-01T10:00:00Z"`,
		`"rule_id": "database_timeout"`,
		`"signal_ids": [`,
		`"value": 0.06`,
	} {
		if !strings.Contains(first.String(), expected) {
			t.Errorf("JSON não contém %q:\n%s", expected, first.String())
		}
	}
}

// TestRenderRepresentaBaselineLegadaComoNull garante que ausência histórica
// não seja serializada como uma janela ou contagem igual a zero.
func TestRenderRepresentaBaselineLegadaComoNull(t *testing.T) {
	t.Parallel()

	diagnosis := reportDiagnosis()
	diagnosis.BaselineStart = nil
	diagnosis.BaselineEnd = nil
	diagnosis.BaselineSignalCount = nil
	var output bytes.Buffer
	if err := Render(&output, diagnosis); err != nil {
		t.Fatalf("Render() erro = %v", err)
	}
	if !strings.Contains(output.String(), `"baseline": null`) {
		t.Fatalf("JSON legado não contém baseline null:\n%s", output.String())
	}
}

func reportDiagnosis() application.PersistedDiagnosis {
	baselineStart := time.Date(2025, time.December, 1, 10, 0, 0, 0, time.UTC)
	baselineEnd := baselineStart.Add(time.Minute)
	incidentEnd := baselineEnd.Add(time.Minute)
	baselineCount := 40
	incidentCount := 40
	return application.PersistedDiagnosis{
		Incident: application.IncidentSummary{
			ID: "inc_1", ServiceName: "checkout-service", Status: "diagnosed",
			IncidentStart: baselineEnd, IncidentEnd: incidentEnd,
		},
		BaselineStart: &baselineStart, BaselineEnd: &baselineEnd,
		BaselineSignalCount: &baselineCount, IncidentSignalCount: &incidentCount,
		Findings: []detection.Finding{{
			Rule: detection.RuleDatabaseTimeout, ServiceName: "checkout-service",
			Score: 0.3, Confidence: detection.ConfidenceHigh,
			Evidence: []detection.Evidence{{
				Summary: "Timeout observado.", SignalIDs: []string{"signal-2", "signal-1"},
				BaselineValue: 0, IncidentValue: 0.3,
			}},
			Limitations: []string{"Correlação não comprova causalidade."},
		}},
		Suspects: []ranking.Suspect{{
			ID: "checkout-service", Label: "checkout-service", Score: 0.06,
			Confidence: detection.ConfidenceHigh,
			Contributions: []ranking.ScoreContribution{{
				RuleID: detection.RuleDatabaseTimeout, Value: 0.06, Reason: "score 0.30 × peso 0.20 = 0.06",
			}},
		}},
	}
}
