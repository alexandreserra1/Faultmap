package detection

import (
	"fmt"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestDatabaseLatencyDeltaAcusaBancoQueDegradouSemFalhar cobre o caso que
// motivou este detector, observado em uma aplicação real sob carga: o banco
// ficou catorze vezes mais lento e não falhou nenhuma vez.
//
// As duas regras de banco existentes procuram falha — timeout e erro — então
// nenhuma tinha o que dizer, e o diagnóstico só mostrava a latência HTTP. Quem
// investigava via "a API ficou lenta" sem ver "o banco ficou lento", que é a
// informação que aponta onde mexer.
func TestDatabaseLatencyDeltaAcusaBancoQueDegradouSemFalhar(t *testing.T) {
	t.Parallel()

	finding, found := DetectDatabaseLatencyDelta(Input{
		ServiceName: "strideredge-api",
		Baseline:    bancoComLatência(60, 0.40),
		Incident:    bancoComLatência(300, 5.67),
	})
	if !found {
		t.Fatal("detector silenciou um banco que ficou catorze vezes mais lento")
	}
	if finding.Rule != RuleDatabaseLatencyDelta {
		t.Fatalf("regra = %q", finding.Rule)
	}
	if len(finding.Evidence) == 0 || len(finding.Evidence[0].SignalIDs) == 0 {
		t.Fatal("finding sem proveniência: a evidência precisa citar os sinais de origem")
	}
}

// TestDatabaseLatencyDeltaIgnoraOscilaçãoNormal impede que o detector repita o
// falso positivo que a latência HTTP já produziu: variação de fração de
// milissegundo entre duas janelas é o comportamento normal de um banco local.
func TestDatabaseLatencyDeltaIgnoraOscilaçãoNormal(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		nome               string
		baseline, incident float64
	}{
		{nome: "fração de milissegundo", baseline: 0.32, incident: 0.41},
		{nome: "dobrou mas segue irrelevante", baseline: 0.30, incident: 0.70},
		{nome: "cresceu pouco em banco lento", baseline: 40, incident: 44},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.nome, func(t *testing.T) {
			t.Parallel()

			if finding, found := DetectDatabaseLatencyDelta(Input{
				ServiceName: "payment-service",
				Baseline:    bancoComLatência(60, testCase.baseline),
				Incident:    bancoComLatência(60, testCase.incident),
			}); found {
				t.Fatalf("detector acusou oscilação normal: %s", finding.Evidence[0].Summary)
			}
		})
	}
}

// TestDatabaseLatencyDeltaExigeDuasJanelas garante que o detector não invente
// conclusão a partir de uma janela só.
func TestDatabaseLatencyDeltaExigeDuasJanelas(t *testing.T) {
	t.Parallel()

	if _, found := DetectDatabaseLatencyDelta(Input{
		ServiceName: "payment-service",
		Baseline:    nil,
		Incident:    bancoComLatência(60, 90),
	}); found {
		t.Fatal("detector concluiu sem baseline para comparar")
	}
}

// bancoComLatência monta operações de banco com a duração informada.
func bancoComLatência(total int, duraçãoMS float64) []domain.Signal {
	signals := make([]domain.Signal, 0, total)
	for index := 0; index < total; index++ {
		signals = append(signals, domain.Signal{
			ID:          fmt.Sprintf("db-lat-%03d", index),
			ServiceName: "strideredge-api",
			Attributes: map[string]string{
				"db.system":    "duckdb",
				"db.operation": "SELECT",
			},
			Measurements: map[string]float64{"duration_ms": duraçãoMS},
		})
	}
	return signals
}

// TestDatabaseLatencyDeltaIgnoraOperaçõesQueFalharam mantém o detector fiel ao
// que ele promete: medir a degradação de operações que concluíram.
//
// Uma operação que estourou o tempo é, por definição, lenta — contá-la aqui
// repetiria o que database_timeout já explica e encheria a saída com duas
// hipóteses para o mesmo fato. Quando todas as operações do incidente falharam,
// não sobra nada para medir e o detector se cala.
func TestDatabaseLatencyDeltaIgnoraOperaçõesQueFalharam(t *testing.T) {
	t.Parallel()

	saudáveis := bancoComLatência(30, 0.5)
	comTimeout := bancoComLatência(30, 900)
	for indice := range comTimeout {
		comTimeout[indice].Severity = "error"
		comTimeout[indice].Attributes["error.type"] = "timeout"
	}

	if finding, found := DetectDatabaseLatencyDelta(Input{
		ServiceName: "payment-service", Baseline: saudáveis, Incident: comTimeout,
	}); found {
		t.Fatalf("detector duplicou o que o timeout já explica: %s", finding.Evidence[0].Summary)
	}
}

// TestDatabaseLatencyDeltaMedeAsOperaçõesQueConcluíram garante que a exclusão
// das falhas não silencie a degradação real que convive com algumas falhas.
func TestDatabaseLatencyDeltaMedeAsOperaçõesQueConcluíram(t *testing.T) {
	t.Parallel()

	incidente := bancoComLatência(30, 8)
	// Algumas operações falharam; a maioria concluiu, porém devagar.
	for indice := 0; indice < 5; indice++ {
		incidente[indice].Severity = "error"
		incidente[indice].Attributes["error.type"] = "timeout"
		incidente[indice].Measurements["duration_ms"] = 5000
	}

	finding, found := DetectDatabaseLatencyDelta(Input{
		ServiceName: "payment-service", Baseline: bancoComLatência(30, 0.5), Incident: incidente,
	})
	if !found {
		t.Fatal("detector silenciou a degradação das operações que concluíram")
	}
	// O p95 precisa refletir as operações concluídas, e não os 5000 ms das que falharam.
	if finding.Evidence[0].IncidentValue > 100 {
		t.Fatalf("p95 = %.2f ms, contaminado pelas operações que falharam", finding.Evidence[0].IncidentValue)
	}
}
