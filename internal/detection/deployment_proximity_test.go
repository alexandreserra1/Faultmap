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

	finding, found := detectDeploymentProximity(input, []changedomain.Deployment{deployment}, incidentStart)
	if !found {
		t.Fatal("detectDeploymentProximity() found = false")
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
	if _, found := detectDeploymentProximity(input, deployments, incidentStart); found {
		t.Fatal("detectDeploymentProximity() encontrou deployment sem relação válida")
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

// TestDeploymentProximityAcusaOCommitComoSuspeito cobre a mudança que coloca o
// commit no ranking por direito próprio.
//
// Antes, o commit aparecia apenas dentro da evidência do serviço: quem lia via
// "o checkout está suspeito" e precisava caçar, no texto, qual mudança tinha
// entrado. A informação mais acionável de um incidente causado por deploy ficava
// escondida no meio da explicação.
func TestDeploymentProximityAcusaOCommitComoSuspeito(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.UTC)
	deployment := changedomain.Deployment{
		ID: "deploy-1", Repository: "acme/checkout", Environment: "producao",
		ServiceName: "checkout-service", CommitSHA: "abc123def4567890",
		State: "success", DeployedAt: incidentStart.Add(-6 * time.Minute),
	}
	input := Input{
		ServiceName: "checkout-service",
		Baseline:    spansComVersao("baseline", 10, "1.0.0"),
		Incident:    spansComVersao("incident", 10, "abc123def4567890"),
	}

	findings := DetectDeploymentProximityFindings(
		input,
		[]changedomain.Deployment{deployment},
		map[string]string{"abc123def4567890": "Reduce payment timeout"},
		incidentStart,
	)
	if len(findings) != 2 {
		t.Fatalf("findings = %d, esperado o serviço e o commit", len(findings))
	}

	var commitFinding *Finding
	for indice := range findings {
		if kind, _, _ := findings[indice].Subject(); kind == SubjectCommit {
			commitFinding = &findings[indice]
		}
	}
	if commitFinding == nil {
		t.Fatal("nenhum finding acusou o commit")
	}
	_, identificador, rotulo := commitFinding.Subject()
	if identificador != "abc123def4567890" {
		t.Fatalf("identificador do commit = %q", identificador)
	}
	// O rótulo precisa ser legível: quem investiga não decora SHA.
	if !strings.Contains(rotulo, "Reduce payment timeout") {
		t.Fatalf("rótulo do commit = %q, esperado conter a mensagem", rotulo)
	}
	// O serviço continua sendo registrado, para que a evidência do commit possa
	// ser lida no contexto de onde ele foi implantado.
	if commitFinding.ServiceName != "checkout-service" {
		t.Fatalf("serviço do finding do commit = %q", commitFinding.ServiceName)
	}
}

// TestDeploymentProximitySemMensagemUsaOIdentificador garante que a ausência da
// mensagem não impeça o commit de ser acusado.
func TestDeploymentProximitySemMensagemUsaOIdentificador(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.UTC)
	findings := DetectDeploymentProximityFindings(
		Input{
			ServiceName: "checkout-service",
			Baseline:    spansComVersao("baseline", 10, "1.0.0"),
			Incident:    spansComVersao("incident", 10, "abc123def4567890"),
		},
		[]changedomain.Deployment{{
			ID: "deploy-1", ServiceName: "checkout-service", CommitSHA: "abc123def4567890",
			State: "success", DeployedAt: incidentStart.Add(-time.Minute),
		}},
		nil,
		incidentStart,
	)
	if len(findings) != 2 {
		t.Fatalf("findings = %d, esperado o serviço e o commit mesmo sem mensagem", len(findings))
	}
}

// versionedHTTPSignals monta spans HTTP declarando a versão observada.
func spansComVersao(prefixo string, total int, versao string) []domain.Signal {
	signals := make([]domain.Signal, 0, total)
	for indice := 0; indice < total; indice++ {
		signals = append(signals, domain.Signal{
			ID:          prefixo + "-" + versao + "-" + string(rune('a'+indice%26)),
			ServiceName: "checkout-service",
			Attributes: map[string]string{
				"http.response.status_code": "200",
				"service.version":           versao,
			},
			Measurements: map[string]float64{"duration_ms": 10},
		})
	}
	return signals
}
