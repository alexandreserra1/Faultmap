package terminal_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/platform/config"
	"github.com/faultmap/faultmap/internal/ranking"
	"github.com/faultmap/faultmap/internal/reporting/terminal"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestDiagnósticoNãoDependeDaOrdemDeChegadaDosSinais ataca a promessa de
// determinismo do produto com entrada aleatória em vez de casos escritos à mão.
//
// Sinais chegam pela rede em lotes e o banco não garante ordem estável entre
// consultas. Se qualquer etapa — detecção, ranking ou renderização — vazar
// iteração de mapa ou usar ordenação instável, o mesmo incidente produz
// relatórios diferentes a cada execução, e a pessoa perde a única coisa que
// distingue este produto de um palpite: poder conferir.
//
// Um teste com ordem fixa nunca encontraria isso, porque o defeito só aparece
// quando a ordem muda. A comparação é byte a byte contra a saída da ordem
// original, para muitas permutações.
func TestDiagnósticoNãoDependeDaOrdemDeChegadaDosSinais(t *testing.T) {
	t.Parallel()

	// A semente é fixa para que uma falha seja reproduzível: um teste aleatório
	// que não se repete transforma defeito em folclore.
	sorteio := rand.New(rand.NewSource(20260820))
	pesos := ranking.Weights{
		ErrorRateDelta:      config.Default().Ranking.Weights.ErrorRateDelta,
		DeploymentProximity: config.Default().Ranking.Weights.DeploymentProximity,
		DatabaseEvidence:    config.Default().Ranking.Weights.DatabaseEvidence,
		GraphProximity:      config.Default().Ranking.Weights.GraphProximity,
		LatencyDelta:        config.Default().Ranking.Weights.LatencyDelta,
		LogCorrelation:      config.Default().Ranking.Weights.LogCorrelation,
	}

	for rodada := 0; rodada < 40; rodada++ {
		baseline, incidente := telemetriaAleatória(sorteio)
		referência := diagnosticar(t, baseline, incidente, pesos)

		for permutação := 0; permutação < 6; permutação++ {
			obtido := diagnosticar(t, embaralhar(sorteio, baseline), embaralhar(sorteio, incidente), pesos)
			if obtido != referência {
				t.Fatalf("rodada %d, permutação %d: a ordem de chegada mudou o diagnóstico.\n--- esperado ---\n%s\n--- obtido ---\n%s",
					rodada, permutação, referência, obtido)
			}
		}
	}
}

func diagnosticar(t *testing.T, baseline, incidente []domain.Signal, pesos ranking.Weights) string {
	t.Helper()
	findings := detection.Run(detection.Input{
		ServiceName: "checkout-service", Baseline: baseline, Incident: incidente,
	})
	// Sem findings não há nada a ordenar, e o teste passaria comparando duas
	// mensagens de "nenhuma anomalia" — verde por vacuidade. Exigir mais de um
	// finding garante que a ordenação entre eles esteja sendo exercitada.
	if len(findings) < 2 {
		t.Fatalf("a telemetria gerada produziu %d findings; o teste não exercitaria ordenação", len(findings))
	}
	suspeitos, err := ranking.Rank(findings, ranking.Config{Weights: pesos, TopN: 3})
	if err != nil {
		t.Fatalf("Rank() erro = %v", err)
	}
	var saída bytes.Buffer
	terminal.RenderDiagnosis(&saída, "checkout-service", len(baseline), len(incidente), findings, suspeitos)
	return saída.String()
}

func embaralhar(sorteio *rand.Rand, original []domain.Signal) []domain.Signal {
	copiado := append([]domain.Signal(nil), original...)
	sorteio.Shuffle(len(copiado), func(primeiro, segundo int) {
		copiado[primeiro], copiado[segundo] = copiado[segundo], copiado[primeiro]
	})
	return copiado
}

// telemetriaAleatória monta duas janelas com regressão de tamanho variável.
//
// A geração precisa produzir incidentes que os detectores realmente aceitem: um
// gerador que só criasse ruído deixaria o teste passar por vacuidade, sem nunca
// exercitar ordenação de findings.
func telemetriaAleatória(sorteio *rand.Rand) (baseline, incidente []domain.Signal) {
	instante := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	total := 20 + sorteio.Intn(20)
	errosBaseline := sorteio.Intn(3)
	errosIncidente := total/2 + sorteio.Intn(total/4+1)
	latênciaBase := 20.0 + float64(sorteio.Intn(30))
	latênciaIncidente := latênciaBase * (3 + float64(sorteio.Intn(8)))

	for índice := 0; índice < total; índice++ {
		baseline = append(baseline, requisição("base", índice, instante, índice < errosBaseline, latênciaBase))
		incidente = append(incidente, requisição("inc", índice, instante.Add(time.Hour), índice < errosIncidente, latênciaIncidente))
		if sorteio.Intn(2) == 0 {
			baseline = append(baseline, consultaDeBanco("base", índice, instante, 3, false))
			incidente = append(incidente, consultaDeBanco("inc", índice, instante.Add(time.Hour),
				3+float64(sorteio.Intn(400)), sorteio.Intn(3) == 0))
		}
	}
	return baseline, incidente
}

func requisição(prefixo string, índice int, instante time.Time, falhou bool, duração float64) domain.Signal {
	status := "200"
	if falhou {
		status = "500"
	}
	return domain.Signal{
		ID:          fmt.Sprintf("span-%s-%03d", prefixo, índice),
		Type:        domain.SignalTypeSpan,
		ServiceName: "checkout-service",
		Timestamp:   instante.Add(time.Duration(índice) * time.Second),
		TraceID:     fmt.Sprintf("%s-trace-%03d", prefixo, índice),
		SpanID:      fmt.Sprintf("%s-span-%03d", prefixo, índice),
		Attributes: map[string]string{
			"http.response.status_code": status,
			"http.route":                "/checkout",
			"span.kind":                 "SPAN_KIND_SERVER",
		},
		Measurements: map[string]float64{"duration_ms": duração},
	}
}

func consultaDeBanco(prefixo string, índice int, instante time.Time, duração float64, comTimeout bool) domain.Signal {
	atributos := map[string]string{
		"db.system.name":      "postgresql",
		"db.operation.name":   "INSERT",
		"db.collection.name":  "orders",
		"span.kind":           "SPAN_KIND_CLIENT",
		"db.operation.status": "ok",
	}
	if comTimeout {
		atributos["error.type"] = "timeout"
		atributos["status.message"] = "canceling statement due to statement timeout"
	}
	return domain.Signal{
		ID:           fmt.Sprintf("db-%s-%03d", prefixo, índice),
		Type:         domain.SignalTypeSpan,
		ServiceName:  "checkout-service",
		Timestamp:    instante.Add(time.Duration(índice) * time.Second),
		TraceID:      fmt.Sprintf("%s-trace-%03d", prefixo, índice),
		SpanID:       fmt.Sprintf("%s-dbspan-%03d", prefixo, índice),
		Attributes:   atributos,
		Measurements: map[string]float64{"duration_ms": duração},
	}
}
