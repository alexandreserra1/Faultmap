package ranking_test

import (
	"math"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

func TestRankAgregaFindingsPorServicoComContribuicoesAuditaveis(t *testing.T) {
	t.Parallel()

	findings := []detection.Finding{
		{
			Rule:        detection.RuleErrorRateDelta,
			ServiceName: "checkout-service",
			Score:       0.80,
			Confidence:  detection.ConfidenceHigh,
			Limitations: []string{"Correlação não comprova causalidade."},
		},
		{
			Rule:        detection.RuleLatencyDelta,
			ServiceName: "checkout-service",
			Score:       0.50,
			Confidence:  detection.ConfidenceHigh,
			Limitations: []string{"Correlação não comprova causalidade.", "Amostra representa somente a janela consultada."},
		},
	}

	suspects, err := ranking.Rank(findings, ranking.Config{
		Weights: ranking.Weights{
			ErrorRateDelta: 0.25,
			LatencyDelta:   0.10,
		},
		TopN: 3,
	})
	if err != nil {
		t.Fatalf("Rank() erro = %v", err)
	}
	if len(suspects) != 1 {
		t.Fatalf("quantidade de suspeitos = %d, esperava 1", len(suspects))
	}

	suspect := suspects[0]
	if suspect.ID != "checkout-service" {
		t.Errorf("ID = %q, esperava checkout-service", suspect.ID)
	}
	if suspect.Label != "checkout-service" {
		t.Errorf("Label = %q, esperava checkout-service", suspect.Label)
	}
	assertFloat(t, "score", suspect.Score, 0.25)
	if suspect.Confidence != detection.ConfidenceHigh {
		t.Errorf("confiança = %q, esperava %q", suspect.Confidence, detection.ConfidenceHigh)
	}

	if len(suspect.Contributions) != 2 {
		t.Fatalf("quantidade de contribuições = %d, esperava 2", len(suspect.Contributions))
	}
	assertContribution(t, suspect.Contributions, detection.RuleErrorRateDelta, 0.20)
	assertContribution(t, suspect.Contributions, detection.RuleLatencyDelta, 0.05)

	if len(suspect.Limitations) != 2 {
		t.Fatalf("limitações = %#v, esperava duas limitações sem duplicidade", suspect.Limitations)
	}
	assertContains(t, suspect.Limitations, "Correlação não comprova causalidade.")
	assertContains(t, suspect.Limitations, "Amostra representa somente a janela consultada.")
}

func TestRankMapeiaCadaRegraAoPesoCorreto(t *testing.T) {
	t.Parallel()

	findings := []detection.Finding{
		{Rule: detection.RuleErrorRateDelta, ServiceName: "api", Score: 1, Confidence: detection.ConfidenceHigh},
		{Rule: detection.RuleLatencyDelta, ServiceName: "api", Score: 1, Confidence: detection.ConfidenceHigh},
		{Rule: detection.RuleDatabaseTimeout, ServiceName: "api", Score: 1, Confidence: detection.ConfidenceHigh},
		{Rule: detection.RuleTraceCorrelation, ServiceName: "api", Score: 1, Confidence: detection.ConfidenceHigh},
		{Rule: detection.RuleDeploymentProximity, ServiceName: "api", Score: 1, Confidence: detection.ConfidenceHigh},
	}
	weights := ranking.Weights{
		ErrorRateDelta:      0.10,
		LatencyDelta:        0.20,
		DatabaseEvidence:    0.30,
		GraphProximity:      0.40,
		DeploymentProximity: 0.50,
	}

	suspects, err := ranking.Rank(findings, ranking.Config{Weights: weights, TopN: 1})
	if err != nil {
		t.Fatalf("Rank() erro = %v", err)
	}
	if len(suspects) != 1 {
		t.Fatalf("quantidade de suspeitos = %d, esperava 1", len(suspects))
	}
	assertFloat(t, "score", suspects[0].Score, 1)
	assertContribution(t, suspects[0].Contributions, detection.RuleErrorRateDelta, weights.ErrorRateDelta)
	assertContribution(t, suspects[0].Contributions, detection.RuleLatencyDelta, weights.LatencyDelta)
	assertContribution(t, suspects[0].Contributions, detection.RuleDatabaseTimeout, weights.DatabaseEvidence)
	assertContribution(t, suspects[0].Contributions, detection.RuleTraceCorrelation, weights.GraphProximity)
	assertContribution(t, suspects[0].Contributions, detection.RuleDeploymentProximity, weights.DeploymentProximity)
}

// TestRankMapeiaRetryStormParaProximidadeDoGrafo garante que a repetição
// estrutural no mesmo trace reutilize o peso existente sem ampliar o contrato YAML.
func TestRankMapeiaRetryStormParaProximidadeDoGrafo(t *testing.T) {
	t.Parallel()

	findings := []detection.Finding{{
		Rule:        detection.RuleRetryStorm,
		ServiceName: "checkout-service",
		Score:       0.80,
		Confidence:  detection.ConfidenceHigh,
		Evidence: []detection.Evidence{{
			Summary: "A média aumentou de 1,00 para 4,00 tentativas por trace.",
		}},
	}}

	suspects, err := ranking.Rank(findings, ranking.Config{
		Weights: ranking.Weights{GraphProximity: 0.15},
		TopN:    1,
	})
	if err != nil {
		t.Fatalf("Rank() erro = %v", err)
	}
	if len(suspects) != 1 {
		t.Fatalf("quantidade de suspeitos = %d, esperava 1", len(suspects))
	}
	assertFloat(t, "score", suspects[0].Score, 0.12)
	assertContribution(t, suspects[0].Contributions, detection.RuleRetryStorm, 0.12)
	if !strings.Contains(suspects[0].Contributions[0].Reason, "score 0.80 × peso 0.15 = 0.12") {
		t.Errorf("motivo não apresenta a fórmula auditável: %q", suspects[0].Contributions[0].Reason)
	}
}

func TestRankLimitaScoreAoIntervaloDeZeroAUm(t *testing.T) {
	t.Parallel()

	findings := []detection.Finding{
		{Rule: detection.RuleErrorRateDelta, ServiceName: "api", Score: 1, Confidence: detection.ConfidenceHigh},
		{Rule: detection.RuleLatencyDelta, ServiceName: "api", Score: 1, Confidence: detection.ConfidenceHigh},
	}

	suspects, err := ranking.Rank(findings, ranking.Config{
		Weights: ranking.Weights{ErrorRateDelta: 0.75, LatencyDelta: 0.75},
		TopN:    1,
	})
	if err != nil {
		t.Fatalf("Rank() erro = %v", err)
	}
	assertFloat(t, "score limitado", suspects[0].Score, 1)
}

func TestRankReduzConfiancaQuandoQualquerContribuinteTemConfiancaBaixa(t *testing.T) {
	t.Parallel()

	findings := []detection.Finding{
		{Rule: detection.RuleErrorRateDelta, ServiceName: "api", Score: 0.50, Confidence: detection.ConfidenceHigh},
		{Rule: detection.RuleLatencyDelta, ServiceName: "api", Score: 0.50, Confidence: detection.ConfidenceLow},
	}

	suspects, err := ranking.Rank(findings, ranking.Config{
		Weights: ranking.Weights{ErrorRateDelta: 0.50, LatencyDelta: 0.50},
		TopN:    1,
	})
	if err != nil {
		t.Fatalf("Rank() erro = %v", err)
	}
	if suspects[0].Confidence != detection.ConfidenceLow {
		t.Errorf("confiança = %q, esperava %q", suspects[0].Confidence, detection.ConfidenceLow)
	}
}

func TestRankIgnoraRegraDesconhecida(t *testing.T) {
	t.Parallel()

	findings := []detection.Finding{
		{Rule: "regra_futura", ServiceName: "somente-desconhecida", Score: 1, Confidence: detection.ConfidenceLow},
		{Rule: detection.RuleErrorRateDelta, ServiceName: "api", Score: 0.50, Confidence: detection.ConfidenceHigh},
		{Rule: "regra_futura", ServiceName: "api", Score: 1, Confidence: detection.ConfidenceLow},
	}

	suspects, err := ranking.Rank(findings, ranking.Config{
		Weights: ranking.Weights{ErrorRateDelta: 0.40},
		TopN:    3,
	})
	if err != nil {
		t.Fatalf("Rank() erro = %v", err)
	}
	if len(suspects) != 1 {
		t.Fatalf("suspeitos = %#v, esperava somente o serviço com regra conhecida", suspects)
	}
	assertFloat(t, "score", suspects[0].Score, 0.20)
	if len(suspects[0].Contributions) != 1 {
		t.Fatalf("contribuições = %#v, esperava somente a regra conhecida", suspects[0].Contributions)
	}
	if suspects[0].Confidence != detection.ConfidenceHigh {
		t.Errorf("regra desconhecida alterou a confiança para %q", suspects[0].Confidence)
	}
}

func TestRankOrdenaPorScoreDecrescenteEDesempataPorID(t *testing.T) {
	t.Parallel()

	findings := []detection.Finding{
		{Rule: detection.RuleErrorRateDelta, ServiceName: "zeta", Score: 0.50, Confidence: detection.ConfidenceHigh},
		{Rule: detection.RuleErrorRateDelta, ServiceName: "alpha", Score: 0.50, Confidence: detection.ConfidenceHigh},
		{Rule: detection.RuleErrorRateDelta, ServiceName: "maior", Score: 0.90, Confidence: detection.ConfidenceHigh},
	}

	suspects, err := ranking.Rank(findings, ranking.Config{
		Weights: ranking.Weights{ErrorRateDelta: 1},
		TopN:    3,
	})
	if err != nil {
		t.Fatalf("Rank() erro = %v", err)
	}
	want := []string{"maior", "alpha", "zeta"}
	for index, id := range want {
		if suspects[index].ID != id {
			t.Errorf("suspeito[%d].ID = %q, esperava %q", index, suspects[index].ID, id)
		}
	}
}

func TestRankAplicaTopNDepoisDaOrdenacao(t *testing.T) {
	t.Parallel()

	findings := []detection.Finding{
		{Rule: detection.RuleErrorRateDelta, ServiceName: "baixo", Score: 0.10, Confidence: detection.ConfidenceHigh},
		{Rule: detection.RuleErrorRateDelta, ServiceName: "alto", Score: 0.90, Confidence: detection.ConfidenceHigh},
		{Rule: detection.RuleErrorRateDelta, ServiceName: "medio", Score: 0.50, Confidence: detection.ConfidenceHigh},
	}

	suspects, err := ranking.Rank(findings, ranking.Config{
		Weights: ranking.Weights{ErrorRateDelta: 1},
		TopN:    2,
	})
	if err != nil {
		t.Fatalf("Rank() erro = %v", err)
	}
	if len(suspects) != 2 {
		t.Fatalf("quantidade de suspeitos = %d, esperava 2", len(suspects))
	}
	if suspects[0].ID != "alto" || suspects[1].ID != "medio" {
		t.Errorf("top 2 = %q, %q; esperava alto, medio", suspects[0].ID, suspects[1].ID)
	}
}

func TestRankRejeitaConfiguracaoInvalida(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config ranking.Config
	}{
		{name: "top N zero", config: ranking.Config{TopN: 0}},
		{name: "top N negativo", config: ranking.Config{TopN: -1}},
		{name: "peso negativo", config: ranking.Config{TopN: 1, Weights: ranking.Weights{ErrorRateDelta: -0.01}}},
		{name: "peso acima de um", config: ranking.Config{TopN: 1, Weights: ranking.Weights{LatencyDelta: 1.01}}},
		{name: "peso NaN", config: ranking.Config{TopN: 1, Weights: ranking.Weights{DatabaseEvidence: math.NaN()}}},
		{name: "peso infinito", config: ranking.Config{TopN: 1, Weights: ranking.Weights{GraphProximity: math.Inf(1)}}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			suspects, err := ranking.Rank(nil, test.config)
			if err == nil {
				t.Fatalf("Rank() erro = nil, esperava configuração inválida; suspeitos = %#v", suspects)
			}
		})
	}
}

func assertContribution(t *testing.T, contributions []ranking.ScoreContribution, ruleID string, value float64) {
	t.Helper()
	for _, contribution := range contributions {
		if contribution.RuleID != ruleID {
			continue
		}
		assertFloat(t, "contribuição de "+ruleID, contribution.Value, value)
		if strings.TrimSpace(contribution.Reason) == "" {
			t.Errorf("motivo da contribuição %q está vazio", ruleID)
		}
		return
	}
	t.Errorf("contribuição %q não encontrada em %#v", ruleID, contributions)
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Errorf("%q não encontrado em %#v", want, values)
}

func assertFloat(t *testing.T, label string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %.12f, esperava %.12f", label, got, want)
	}
}
