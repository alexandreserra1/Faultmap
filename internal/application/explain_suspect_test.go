package application_test

import (
	"errors"
	"testing"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

// snapshotDeExemplo monta um snapshot persistido com um suspeito que possui uma
// contribuição sustentada por finding e outra sem finding correspondente.
func snapshotDeExemplo() application.PersistedDiagnosis {
	return application.PersistedDiagnosis{
		Incident: application.IncidentSummary{ID: "inc-1", ServiceName: "checkout-service"},
		Findings: []detection.Finding{
			{
				Rule:        detection.RuleErrorRateDelta,
				ServiceName: "checkout-service",
				Score:       0.8,
				Confidence:  detection.ConfidenceHigh,
				Evidence: []detection.Evidence{
					{Summary: "erros subiram", SignalIDs: []string{"sig-2", "sig-1"}, ChangeIDs: []string{"chg-1"}},
				},
				Limitations: []string{"janela curta"},
			},
			{
				Rule:        detection.RuleLatencyDelta,
				ServiceName: "checkout-service",
				Score:       0.4,
				Confidence:  detection.ConfidenceLow,
				Evidence:    []detection.Evidence{{Summary: "latência subiu"}},
			},
			{
				Rule:        detection.RuleErrorRateDelta,
				ServiceName: "payments-service",
				Score:       0.9,
			},
		},
		Suspects: []ranking.Suspect{
			{
				ID:         "checkout-service",
				Label:      "checkout-service",
				Score:      0.66,
				Confidence: detection.ConfidenceLow,
				Contributions: []ranking.ScoreContribution{
					{RuleID: detection.RuleErrorRateDelta, Value: 0.32, Reason: "score 0.80 × peso 0.40 = 0.32"},
					{RuleID: detection.RuleDeploymentProximity, Value: 0.20, Reason: "score 1.00 × peso 0.20 = 0.20"},
				},
				Limitations: []string{"janela curta"},
			},
			{ID: "payments-service", Label: "payments-service", Score: 0.4},
		},
	}
}

func TestExplicarSuspeitoAgrupaContribuicoesEvidenciasEProveniencia(t *testing.T) {
	explanation, err := application.ExplainSuspect(snapshotDeExemplo(), "checkout-service")
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if explanation.IncidentID != "inc-1" || explanation.SuspectID != "checkout-service" {
		t.Fatalf("identificação inesperada: %+v", explanation)
	}
	if len(explanation.Contributions) != 2 {
		t.Fatalf("esperava 2 contribuições, obtive %d", len(explanation.Contributions))
	}
	// Ordenação determinística: maior valor primeiro.
	if explanation.Contributions[0].RuleID != detection.RuleErrorRateDelta {
		t.Fatalf("ordem de contribuições inesperada: %+v", explanation.Contributions)
	}
	primeira := explanation.Contributions[0]
	if len(primeira.Findings) != 1 || len(primeira.Findings[0].Evidence) != 1 {
		t.Fatalf("esperava finding com evidência: %+v", primeira)
	}
	if primeira.Findings[0].Evidence[0].SignalIDs[0] != "sig-1" {
		t.Fatalf("IDs de sinais deveriam estar ordenados: %+v", primeira.Findings[0].Evidence[0])
	}
	// Honestidade: contribuição sem finding correspondente precisa ficar visível.
	segunda := explanation.Contributions[1]
	if segunda.RuleID != detection.RuleDeploymentProximity || segunda.Supported {
		t.Fatalf("esperava contribuição sem suporte: %+v", segunda)
	}
	// Findings do serviço que não viraram contribuição também não podem sumir.
	if len(explanation.UnscoredFindings) != 1 ||
		explanation.UnscoredFindings[0].Rule != detection.RuleLatencyDelta {
		t.Fatalf("esperava finding sem contribuição: %+v", explanation.UnscoredFindings)
	}
	if len(explanation.Limitations) != 1 || explanation.Limitations[0] != "janela curta" {
		t.Fatalf("limitações deveriam ser preservadas: %+v", explanation.Limitations)
	}
}

func TestExplicarSuspeitoInexistenteDevolveErroDeDominio(t *testing.T) {
	_, err := application.ExplainSuspect(snapshotDeExemplo(), "inventory-service")
	if !errors.Is(err, application.ErrSuspectNotFound) {
		t.Fatalf("esperava ErrSuspectNotFound, obtive %v", err)
	}
}

func TestExplicarSuspeitoExigeNomeNaoVazio(t *testing.T) {
	_, err := application.ExplainSuspect(snapshotDeExemplo(), "   ")
	if err == nil {
		t.Fatal("esperava erro para nome vazio")
	}
	if errors.Is(err, application.ErrSuspectNotFound) {
		t.Fatal("nome vazio é erro de uso, não ausência do suspeito")
	}
}

func TestExplicarSuspeitoIgnoraCaixaEEspacos(t *testing.T) {
	explanation, err := application.ExplainSuspect(snapshotDeExemplo(), "  Checkout-Service ")
	if err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	// O rótulo devolvido é o do snapshot, não o texto digitado pelo operador.
	if explanation.SuspectLabel != "checkout-service" {
		t.Fatalf("rótulo inesperado: %q", explanation.SuspectLabel)
	}
}
