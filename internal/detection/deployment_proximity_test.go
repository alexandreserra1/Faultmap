package detection

import (
	"strings"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestDetectDeploymentProximityRelacionaVersaoEJanelaDoIncidente garante que
// proximidade temporal fortaleça uma hipótese sem ser descrita como causa.
func TestDetectDeploymentProximityRelacionaVersaoEJanelaDoIncidente(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	input := Input{
		ServiceName: "checkout-service",
		Baseline: []domain.Signal{{
			ID: "baseline", ServiceName: "checkout-service",
			Attributes: map[string]string{"service.version": "oldsha"},
		}},
		Incident: []domain.Signal{{
			ID: "incident", ServiceName: "checkout-service", Timestamp: incidentStart,
			Attributes: map[string]string{"service.version": "newsha"},
		}},
	}
	deployment := changedomain.Deployment{
		ID: "deployment-42", Repository: "acme/checkout", Environment: "staging",
		ServiceName: "checkout-service", CommitSHA: "newsha", DeployedAt: incidentStart.Add(-6 * time.Minute),
	}

	finding, found := DetectDeploymentProximity(input, []changedomain.Deployment{deployment}, incidentStart)
	if !found {
		t.Fatal("DetectDeploymentProximity() found = false")
	}
	if finding.Rule != RuleDeploymentProximity || finding.ServiceName != "checkout-service" {
		t.Fatalf("finding = %#v", finding)
	}
	if finding.Score < 0.89 || finding.Score > 0.91 {
		t.Fatalf("score = %v, esperado aproximadamente 0.90", finding.Score)
	}
	if finding.Confidence != ConfidenceHigh {
		t.Fatalf("confiança = %q, esperado alta por correspondência de versão", finding.Confidence)
	}
	if len(finding.Evidence) != 1 || len(finding.Evidence[0].ChangeIDs) != 1 || finding.Evidence[0].ChangeIDs[0] != deployment.ID || len(finding.Evidence[0].SignalIDs) != 0 {
		t.Fatalf("evidência = %#v", finding.Evidence)
	}
	if !strings.Contains(finding.Evidence[0].Summary, "6 minuto") || !strings.Contains(finding.Evidence[0].Summary, "newsha") {
		t.Fatalf("resumo = %q", finding.Evidence[0].Summary)
	}
	if !containsLimitation(finding.Limitations, "prova") {
		t.Fatalf("limitações = %#v, esperado aviso causal", finding.Limitations)
	}
	if !containsLimitation(finding.Limitations, "status") {
		t.Fatalf("limitações = %#v, esperado status desconhecido", finding.Limitations)
	}
}

// TestDetectDeploymentProximityIgnoraDeployForaDaJanelaOuOutroServico evita
// elevar o ranking com mudanças sem relação temporal e de identidade.
func TestDetectDeploymentProximityIgnoraDeployForaDaJanelaOuOutroServico(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	input := Input{ServiceName: "checkout-service"}
	deployments := []changedomain.Deployment{
		{ID: "old", ServiceName: "checkout-service", DeployedAt: incidentStart.Add(-DeploymentLookback - time.Second)},
		{ID: "other", ServiceName: "payment-service", DeployedAt: incidentStart.Add(-time.Minute)},
		{ID: "future", ServiceName: "checkout-service", DeployedAt: incidentStart.Add(time.Second)},
	}
	if _, found := DetectDeploymentProximity(input, deployments, incidentStart); found {
		t.Fatal("DetectDeploymentProximity() encontrou deployment sem relação válida")
	}
}

func containsLimitation(limitations []string, fragment string) bool {
	for _, limitation := range limitations {
		if strings.Contains(strings.ToLower(limitation), strings.ToLower(fragment)) {
			return true
		}
	}
	return false
}
