package terminal

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

// TestRenderIncidentListOrdenaEExibeResumo garante saída determinística mesmo
// quando outro leitor fornecer resumos fora da ordem esperada.
func TestRenderIncidentListOrdenaEExibeResumo(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	incidents := []application.IncidentSummary{
		{ID: "inc-old", ServiceName: "checkout", Status: "diagnosed", IncidentStart: start, IncidentEnd: start.Add(time.Minute)},
		{ID: "inc-new", ServiceName: "payment", Status: "diagnosed", IncidentStart: start.Add(time.Hour), IncidentEnd: start.Add(time.Hour + time.Minute)},
	}
	var output bytes.Buffer
	if err := RenderIncidentList(&output, incidents); err != nil {
		t.Fatalf("RenderIncidentList() erro = %v", err)
	}
	result := output.String()
	if strings.Index(result, "inc-new") > strings.Index(result, "inc-old") {
		t.Fatalf("listagem não ordenou mais recente primeiro:\n%s", result)
	}
	for _, expected := range []string{"Incidentes persistidos — 2", "payment", "Status: diagnosed"} {
		if !strings.Contains(result, expected) {
			t.Errorf("saída não contém %q:\n%s", expected, result)
		}
	}
}

// TestRenderPersistedDiagnosisExplicaMetadataLegada evita apresentar zeros
// inventados quando uma linha anterior à migration não possui baseline.
func TestRenderPersistedDiagnosisExplicaMetadataLegada(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	diagnosis := application.PersistedDiagnosis{
		Incident: application.IncidentSummary{
			ID: "inc-legacy", ServiceName: "checkout-service", Status: "diagnosed",
			IncidentStart: start, IncidentEnd: start.Add(time.Minute),
		},
		Findings: []detection.Finding{{
			Rule: detection.RuleDatabaseTimeout, ServiceName: "checkout-service",
			Score: 0.3, Confidence: detection.ConfidenceHigh,
			Evidence: []detection.Evidence{{Summary: "Timeout PostgreSQL observado."}},
		}},
		Suspects: []ranking.Suspect{{ID: "checkout-service", Label: "checkout-service", Score: 0.3, Confidence: detection.ConfidenceHigh}},
	}
	var output bytes.Buffer
	if err := RenderPersistedDiagnosis(&output, diagnosis); err != nil {
		t.Fatalf("RenderPersistedDiagnosis() erro = %v", err)
	}
	result := output.String()
	for _, expected := range []string{
		"Incidente persistido — inc-legacy",
		"Serviço: checkout-service",
		"Status: diagnosed",
		"Metadata de baseline e contagens indisponível para este snapshot legado.",
		"Timeout PostgreSQL observado.",
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("saída não contém %q:\n%s", expected, result)
		}
	}
}
