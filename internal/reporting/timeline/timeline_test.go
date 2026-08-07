package timeline

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

// TestRenderOrdenaEventosCronologicamente confirma o contrato do timeline.json:
// janelas primeiro, findings ancorados no início do incidente e ordem estável.
func TestRenderOrdenaEventosCronologicamente(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	if err := Render(&buffer, completeDiagnosis()); err != nil {
		t.Fatalf("Render() erro = %v", err)
	}

	var document struct {
		SchemaVersion string `json:"schema_version"`
		IncidentID    string `json:"incident_id"`
		ServiceName   string `json:"service_name"`
		Events        []struct {
			At         string   `json:"at"`
			Type       string   `json:"type"`
			RuleID     string   `json:"rule_id"`
			Summary    string   `json:"summary"`
			TimeSource string   `json:"time_source"`
			SignalIDs  []string `json:"signal_ids"`
			ChangeIDs  []string `json:"change_ids"`
		} `json:"events"`
		Limitations []string `json:"limitations"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &document); err != nil {
		t.Fatalf("timeline.json inválido: %v\n%s", err, buffer.String())
	}
	if document.SchemaVersion != schemaVersion {
		t.Fatalf("schema_version = %q, esperado %q", document.SchemaVersion, schemaVersion)
	}
	if document.IncidentID != "inc_001" || document.ServiceName != "checkout-service" {
		t.Fatalf("cabeçalho inesperado: %+v", document)
	}

	expectedTypes := []string{
		"baseline_window_start",
		"baseline_window_end",
		"incident_window_start",
		"finding",
		"finding",
		"incident_window_end",
		"diagnosis",
	}
	if len(document.Events) != len(expectedTypes) {
		t.Fatalf("eventos = %d, esperado %d: %s", len(document.Events), len(expectedTypes), buffer.String())
	}
	previous := time.Time{}
	for index, event := range document.Events {
		if event.Type != expectedTypes[index] {
			t.Fatalf("evento %d tipo = %q, esperado %q", index, event.Type, expectedTypes[index])
		}
		at, err := time.Parse(time.RFC3339Nano, event.At)
		if err != nil {
			t.Fatalf("evento %d com instante inválido %q: %v", index, event.At, err)
		}
		if at.Before(previous) {
			t.Fatalf("evento %d quebrou a ordem cronológica: %s antes de %s", index, event.At, previous)
		}
		previous = at
	}

	// Findings empatam no início do incidente e precisam de desempate estável por regra.
	if document.Events[3].RuleID != "deployment_proximity" || document.Events[4].RuleID != "error_rate_delta" {
		t.Fatalf("findings fora da ordem determinística: %q, %q", document.Events[3].RuleID, document.Events[4].RuleID)
	}
	if document.Events[3].TimeSource != "incident_window_start" {
		t.Fatalf("time_source = %q, esperado incident_window_start", document.Events[3].TimeSource)
	}
	if len(document.Events[3].ChangeIDs) != 1 || document.Events[3].ChangeIDs[0] != "deploy-9" {
		t.Fatalf("proveniência de mudança perdida: %+v", document.Events[3])
	}
	if len(document.Limitations) == 0 {
		t.Fatal("limitations vazio: o timeline precisa declarar que findings não têm instante próprio")
	}
}

// TestRenderÉDeterminístico garante que duas execuções do mesmo snapshot geram
// bytes idênticos, requisito para comparar artefatos entre execuções.
func TestRenderÉDeterminístico(t *testing.T) {
	t.Parallel()

	var first, second bytes.Buffer
	if err := Render(&first, completeDiagnosis()); err != nil {
		t.Fatalf("Render() primeira erro = %v", err)
	}
	if err := Render(&second, completeDiagnosis()); err != nil {
		t.Fatalf("Render() segunda erro = %v", err)
	}
	if first.String() != second.String() {
		t.Fatalf("timeline.json não determinístico:\n%s\n---\n%s", first.String(), second.String())
	}
}

// TestRenderDeclaraBaselineAusenteEmSnapshotLegado evita fabricar janelas que o
// registro antigo nunca gravou.
func TestRenderDeclaraBaselineAusenteEmSnapshotLegado(t *testing.T) {
	t.Parallel()

	legacy := completeDiagnosis()
	legacy.BaselineStart = nil
	legacy.BaselineEnd = nil
	legacy.BaselineSignalCount = nil

	var buffer bytes.Buffer
	if err := Render(&buffer, legacy); err != nil {
		t.Fatalf("Render() erro = %v", err)
	}
	content := buffer.String()
	if bytes.Contains(buffer.Bytes(), []byte("baseline_window_start")) {
		t.Fatalf("timeline inventou janela baseline inexistente:\n%s", content)
	}
	if !bytes.Contains(buffer.Bytes(), []byte("Metadados de baseline ausentes")) {
		t.Fatalf("timeline não declarou a baseline ausente:\n%s", content)
	}
}

func completeDiagnosis() application.PersistedDiagnosis {
	incidentStart := time.Date(2026, time.August, 6, 10, 3, 0, 0, time.UTC)
	incidentEnd := incidentStart.Add(2 * time.Minute)
	baselineStart := incidentStart.Add(-time.Hour)
	baselineEnd := incidentStart
	baselineCount := 16
	incidentCount := 16

	return application.PersistedDiagnosis{
		Incident: application.IncidentSummary{
			ID:            "inc_001",
			ServiceName:   "checkout-service",
			Environment:   "demo",
			Status:        "diagnosed",
			IncidentStart: incidentStart,
			IncidentEnd:   incidentEnd,
		},
		BaselineStart:       &baselineStart,
		BaselineEnd:         &baselineEnd,
		BaselineSignalCount: &baselineCount,
		IncidentSignalCount: &incidentCount,
		Findings: []detection.Finding{
			{
				Rule: detection.RuleErrorRateDelta, ServiceName: "checkout-service",
				Score: 1, Confidence: detection.ConfidenceHigh,
				Evidence: []detection.Evidence{{
					Summary: "Taxa de erro: 0,00% → 100,00%", SignalIDs: []string{"sig-2", "sig-1"},
				}},
				Limitations: []string{"Correlação não comprova causalidade."},
			},
			{
				Rule: detection.RuleDeploymentProximity, ServiceName: "checkout-service",
				Score: 0.98, Confidence: detection.ConfidenceHigh,
				Evidence: []detection.Evidence{{
					Summary: "O deployment ocorreu 1 minuto antes do incidente.", ChangeIDs: []string{"deploy-9"},
				}},
			},
		},
		Suspects: []ranking.Suspect{{
			ID: "checkout-service", Label: "checkout-service", Score: 0.54,
			Confidence: detection.ConfidenceHigh,
		}},
	}
}
