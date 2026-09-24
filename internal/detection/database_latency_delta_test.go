package detection

import (
	"fmt"
	"strings"
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

// TestDatabaseLatencyDeltaNaoAfirmaAltaConfiancaEmMagnitudeDeRuido é a correção
// de um padrão observado quatro vezes neste projeto: com o sistema comprovadamente
// saudável, o detector acusou o banco com "confiança alta" em aumentos de poucos
// milissegundos — 1,64 para 5,20 ms, 4 para 12, 3,9 para 12,3 e 6,66 para 17,04.
//
// A causa não era o piso. Era a confiança não olhar a magnitude: ela vinha só do
// tamanho da amostra, então 120 operações bastavam para "alta" mesmo quando o
// efeito media três milissegundos. O produto afirmava com força o que mediu
// fraco.
//
// Medições em janelas comprovadamente saudáveis desta demo produziram p95 de
// incidente de até 17 ms — aqui documentadas para que o número tenha origem e
// não seja escolhido por conveniência. Abaixo disso, o aumento é indistinguível
// do que um sistema sadio produz, e o finding continua sendo emitido (silenciar
// esconderia uma degradação real e pequena) com a confiança dizendo a verdade.
func TestDatabaseLatencyDeltaNaoAfirmaAltaConfiancaEmMagnitudeDeRuido(t *testing.T) {
	t.Parallel()

	finding, found := DetectDatabaseLatencyDelta(Input{
		ServiceName: "payment-service",
		Baseline:    bancoComLatência(120, 1.64),
		Incident:    bancoComLatência(120, 5.20),
	})
	if !found {
		t.Fatal("detector silenciou: a correção é sobre confiança, não sobre suprimir o finding")
	}
	if finding.Confidence != ConfidenceLow {
		t.Fatalf("confiança = %q para um aumento de 3,6 ms, esperado baixa", finding.Confidence)
	}
	// A limitação precisa nomear a magnitude. Dizer "amostra pequena" seria falso
	// e mandaria a pessoa coletar mais dados para um problema que não é esse.
	juntas := strings.Join(finding.Limitations, " ")
	if strings.Contains(juntas, "Amostra pequena") {
		t.Fatalf("limitação culpou a amostra, que tem 120 sinais por janela: %q", juntas)
	}
	if !strings.Contains(juntas, "ms") {
		t.Fatalf("limitação não diz que a magnitude é pequena: %q", juntas)
	}
}

// TestDatabaseLatencyDeltaMantemAltaConfiancaEmDegradacaoReal é o contrapeso: a
// calibragem da confiança não pode enfraquecer o caso que o detector existe para
// pegar. Os números vêm de uma medição real do piloto contra database-slow.
func TestDatabaseLatencyDeltaMantemAltaConfiancaEmDegradacaoReal(t *testing.T) {
	t.Parallel()

	finding, found := DetectDatabaseLatencyDelta(Input{
		ServiceName: "payment-service",
		Baseline:    bancoComLatência(120, 4.57),
		Incident:    bancoComLatência(120, 762.80),
	})
	if !found {
		t.Fatal("detector silenciou uma degradação de 167 vezes")
	}
	if finding.Confidence != ConfidenceHigh {
		t.Fatalf("confiança = %q para 4,57 ms virando 762,80 ms, esperado alta", finding.Confidence)
	}
}

// TestDatabaseLatencyDeltaSomaAsDuasRessalvasQuandoCabem garante que calibrar
// pela magnitude não apague a ressalva de amostra: as duas fraquezas são
// independentes e uma janela pode ter as duas.
func TestDatabaseLatencyDeltaSomaAsDuasRessalvasQuandoCabem(t *testing.T) {
	t.Parallel()

	finding, found := DetectDatabaseLatencyDelta(Input{
		ServiceName: "payment-service",
		Baseline:    bancoComLatência(3, 1.64),
		Incident:    bancoComLatência(3, 5.20),
	})
	if !found {
		t.Fatal("detector silenciou")
	}
	juntas := strings.Join(finding.Limitations, " ")
	if !strings.Contains(juntas, "Amostra pequena") {
		t.Fatalf("com 3 sinais por janela, a ressalva de amostra sumiu: %q", juntas)
	}
	if !strings.Contains(juntas, "ms") {
		t.Fatalf("a ressalva de magnitude sumiu: %q", juntas)
	}
}
