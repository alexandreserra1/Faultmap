package markdown

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

// TestRenderApresentaSnapshotAuditavelComoMarkdown garante que o artefato
// humano preserve janelas, ranking, evidências e limitações.
func TestRenderApresentaSnapshotAuditavelComoMarkdown(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if err := Render(&output, markdownDiagnosis()); err != nil {
		t.Fatalf("Render() erro = %v", err)
	}
	for _, expected := range []string{
		"# Diagnóstico do incidente `inc_1`",
		"**Serviço:** checkout-service",
		"**Baseline:** 40 sinais",
		"## Ranking de suspeitos",
		"### 1. checkout-service",
		"`database_timeout`",
		"## Evidências",
		"Timeout observado.",
		"## Limitações",
		"Correlação não comprova causalidade.",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("Markdown não contém %q:\n%s", expected, output.String())
		}
	}
}

func markdownDiagnosis() application.PersistedDiagnosis {
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
			Evidence:    []detection.Evidence{{Summary: "Timeout observado."}},
			Limitations: []string{"Correlação não comprova causalidade."},
		}},
		Suspects: []ranking.Suspect{{
			ID: "checkout-service", Label: "checkout-service", Score: 0.06,
			Confidence: detection.ConfidenceHigh,
			Contributions: []ranking.ScoreContribution{{
				RuleID: detection.RuleDatabaseTimeout, Value: 0.06,
				Reason: "score 0.30 × peso 0.20 = 0.06",
			}},
		}},
	}
}
