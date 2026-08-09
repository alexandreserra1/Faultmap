package terminal_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/reporting/terminal"
)

func explicacaoDeExemplo() application.SuspectExplanation {
	return application.SuspectExplanation{
		IncidentID:   "inc-1",
		SuspectID:    "checkout-service",
		SuspectLabel: "checkout-service",
		Score:        0.66,
		Confidence:   detection.ConfidenceLow,
		Contributions: []application.SuspectContribution{
			{
				RuleID:    detection.RuleErrorRateDelta,
				Value:     0.32,
				Reason:    "score 0.80 × peso 0.40 = 0.32",
				Supported: true,
				Findings: []detection.Finding{
					{
						Rule:        detection.RuleErrorRateDelta,
						ServiceName: "checkout-service",
						Score:       0.8,
						Confidence:  detection.ConfidenceHigh,
						Evidence: []detection.Evidence{{
							Summary:       "erros subiram",
							SignalIDs:     []string{"sig-1"},
							ChangeIDs:     []string{"chg-1"},
							BaselineValue: 0.01,
							IncidentValue: 0.30,
						}},
						Limitations: []string{"janela curta"},
					},
				},
			},
			{
				RuleID:    detection.RuleDeploymentProximity,
				Value:     0.20,
				Reason:    "score 1.00 × peso 0.20 = 0.20",
				Supported: false,
			},
		},
		UnscoredFindings: []detection.Finding{{
			Rule:        detection.RuleLatencyDelta,
			ServiceName: "checkout-service",
			Score:       0.4,
			Confidence:  detection.ConfidenceLow,
		}},
		Limitations: []string{"janela curta"},
	}
}

func TestRenderizarExplicacaoMostraProvenienciaELimitacoes(t *testing.T) {
	var buffer bytes.Buffer
	if err := terminal.RenderSuspectExplanation(&buffer, explicacaoDeExemplo()); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	saida := buffer.String()
	obrigatorios := []string{
		"checkout-service",
		"inc-1",
		"Score agregado: 0.66",
		// A proveniência é apresentada com a contagem à frente, para que uma
		// evidência sustentada por dezenas de spans continue legível.
		"1 sinal: sig-1",
		"1 mudança: chg-1",
		"janela curta",
		"Sem evidência registrada",
		detection.RuleLatencyDelta,
	}
	for _, trecho := range obrigatorios {
		if !strings.Contains(saida, trecho) {
			t.Fatalf("saída não contém %q:\n%s", trecho, saida)
		}
	}
	// Honestidade: a explicação não pode afirmar causalidade.
	for _, proibido := range []string{"causou", "causa raiz", "root cause"} {
		if strings.Contains(strings.ToLower(saida), proibido) {
			t.Fatalf("saída afirma causalidade (%q):\n%s", proibido, saida)
		}
	}
}

func TestRenderizarExplicacaoSemContribuicoesEhExplicita(t *testing.T) {
	var buffer bytes.Buffer
	explicacao := application.SuspectExplanation{
		IncidentID:   "inc-2",
		SuspectID:    "payments-service",
		SuspectLabel: "payments-service",
	}
	if err := terminal.RenderSuspectExplanation(&buffer, explicacao); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
	if !strings.Contains(buffer.String(), "Nenhuma contribuição") {
		t.Fatalf("esperava declaração explícita:\n%s", buffer.String())
	}
}

// TestRenderSuspectExplanationResumeProveniênciaLonga cobre um problema de
// leitura observado com telemetria real: uma evidência sustentada por dezenas de
// spans despejava todos os identificadores em uma única linha, tornando a
// explicação ilegível justamente onde ela deveria ajudar.
//
// A proveniência não é descartada — ela continua completa em ranking.json e no
// relatório JSON. Aqui apresentamos uma amostra e a contagem total.
func TestRenderSuspectExplanationResumeProveniênciaLonga(t *testing.T) {
	t.Parallel()

	sinais := make([]string, 0, 20)
	for indice := 0; indice < 20; indice++ {
		sinais = append(sinais, fmt.Sprintf("trace-%02d:span-%02d", indice, indice))
	}

	var buffer bytes.Buffer
	err := terminal.RenderSuspectExplanation(&buffer, application.SuspectExplanation{
		IncidentID:   "inc_1",
		SuspectID:    "payment-service",
		SuspectLabel: "payment-service",
		Score:        0.35,
		Confidence:   detection.ConfidenceHigh,
		Contributions: []application.SuspectContribution{{
			RuleID: "error_rate_delta", Value: 0.25, Reason: "erros", Supported: true,
			Findings: []detection.Finding{{
				Rule: "error_rate_delta", ServiceName: "payment-service", Score: 1,
				Confidence: detection.ConfidenceHigh,
				Evidence:   []detection.Evidence{{Summary: "erros", SignalIDs: sinais}},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("RenderSuspectExplanation() erro = %v", err)
	}

	saida := buffer.String()
	if !strings.Contains(saida, "20 sinais") {
		t.Fatalf("a contagem total de sinais precisa aparecer:\n%s", saida)
	}
	if strings.Contains(saida, sinais[19]) {
		t.Fatalf("todos os identificadores foram despejados na saída:\n%s", saida)
	}
	if !strings.Contains(saida, sinais[0]) {
		t.Fatalf("nenhuma amostra de proveniência foi apresentada:\n%s", saida)
	}
	for _, linha := range strings.Split(saida, "\n") {
		if len(linha) > 200 {
			t.Fatalf("linha longa demais para leitura (%d caracteres): %q", len(linha), linha)
		}
	}
}
