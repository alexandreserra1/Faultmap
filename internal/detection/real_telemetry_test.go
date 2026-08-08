package detection

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
	"github.com/faultmap/faultmap/internal/telemetry/normalizer"
)

// Estes testes carregam telemetria produzida por instrumentação de terceiros,
// passando pelo mesmo normalizador da ingestão real. Eles existem porque a
// telemetria escrita por nós usa os mesmos nomes de atributo que o código sob
// teste espera — e por isso jamais poderia revelar um desencontro de convenção.
// Uma release chegou a ser publicada cega para aplicações reais sem que nenhum
// teste falhasse.

// TestTelemetriaRealDeBancoÉEnxergada garante que spans de banco produzidos por
// instrumentação oficial sejam contados. Zero sinais aqui significa cegueira
// total: os detectores de banco nunca disparam, sem qualquer aviso.
func TestTelemetriaRealDeBancoÉEnxergada(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		fixture         string
		sistemaEsperado string
	}{
		{fixture: "postgres-psycopg2.json", sistemaEsperado: "PostgreSQL"},
		{fixture: "sqlite3.json", sistemaEsperado: "sqlite"},
		// Capturado da aplicação de terceiros em uso, com autenticação real.
		{fixture: "duckdb-strideredge.json", sistemaEsperado: "duckdb"},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.fixture, func(t *testing.T) {
			t.Parallel()

			signals := carregarFixtureReal(t, testCase.fixture)
			bancos := filterDatabaseSignals(signals)
			if len(bancos) == 0 {
				t.Fatalf("nenhum span de banco reconhecido em %s: o Faultmap está cego para esta instrumentação", testCase.fixture)
			}

			sistemas := databaseSystems(bancos)
			if len(sistemas) == 0 {
				t.Fatalf("sistema de banco não identificado em %s", testCase.fixture)
			}
			encontrado := false
			for _, sistema := range sistemas {
				if sistema == testCase.sistemaEsperado {
					encontrado = true
				}
			}
			if !encontrado {
				t.Fatalf("sistemas identificados = %v, esperado conter %q", sistemas, testCase.sistemaEsperado)
			}
		})
	}
}

// TestFalhaRealDeBancoÉReconhecida cobre o segundo desencontro: instrumentações
// reais sinalizam falha pelo status do span e por evento de exceção, não por um
// atributo error.type escrito à mão como na nossa demo.
func TestFalhaRealDeBancoÉReconhecida(t *testing.T) {
	t.Parallel()

	for _, fixture := range []string{"postgres-psycopg2.json", "sqlite3.json"} {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			bancos := filterDatabaseSignals(carregarFixtureReal(t, fixture))
			falhas := databaseFailures(bancos)
			if len(falhas) == 0 {
				t.Fatalf("nenhuma falha reconhecida em %s, embora a captura contenha erros reais", fixture)
			}
		})
	}
}

// TestDetectorDeBancoDisparaComTelemetriaReal fecha o ciclo: janela saudável
// contra janela com falha, ambas vindas da captura real, precisam produzir
// finding. É o teste que teria impedido a release cega.
func TestDetectorDeBancoDisparaComTelemetriaReal(t *testing.T) {
	t.Parallel()

	signals := carregarFixtureReal(t, "postgres-psycopg2.json")
	saudaveis, comFalha := separarPorFalha(signals)
	if len(saudaveis) == 0 || len(comFalha) == 0 {
		t.Fatalf("captura sem as duas janelas: saudáveis=%d comFalha=%d", len(saudaveis), len(comFalha))
	}

	finding, found := DetectDatabaseTimeout(Input{
		ServiceName: "captura-postgres", Baseline: saudaveis, Incident: comFalha,
	})
	if !found {
		t.Fatal("detector de banco não disparou com telemetria real de falha")
	}
	if len(finding.Evidence) == 0 || finding.Evidence[0].SignalIDs == nil {
		t.Fatal("finding sem proveniência: evidência precisa citar os sinais de origem")
	}
}

// TestTelemetriaRealHTTPÉEnxergada protege a correção da v0.1.1 contra regressão,
// agora com a telemetria real da aplicação de terceiros e não com uma imitação.
func TestTelemetriaRealHTTPÉEnxergada(t *testing.T) {
	t.Parallel()

	signals := carregarFixtureReal(t, "fastapi-strideredge.json")
	requisicoes := filterHTTPSignals(signals)
	if len(requisicoes) == 0 {
		t.Fatal("nenhuma requisição HTTP reconhecida: o Faultmap está cego para esta instrumentação")
	}
	// A instrumentação ASGI emite spans internos que repetem o código de
	// resposta. Contá-los inflaria o denominador de toda taxa de erro.
	for _, sinal := range requisicoes {
		if sinal.Attributes["span.kind"] == spanKindInternal {
			t.Fatalf("span interno %q contado como requisição", sinal.Attributes["span.name"])
		}
	}
}

// separarPorFalha divide a captura em uma janela sem falhas e outra com falhas,
// preservando os sinais exatamente como foram recebidos.
func separarPorFalha(signals []domain.Signal) (saudaveis []domain.Signal, comFalha []domain.Signal) {
	for _, sinal := range signals {
		if sinal.Severity == "error" {
			comFalha = append(comFalha, sinal)
			continue
		}
		saudaveis = append(saudaveis, sinal)
	}
	return saudaveis, comFalha
}

func carregarFixtureReal(t *testing.T, nome string) []domain.Signal {
	t.Helper()

	caminho := filepath.Join("..", "..", "fixtures", "otel", "real", nome)
	arquivo, err := os.Open(caminho)
	if err != nil {
		t.Fatalf("abrir fixture %s: %v", nome, err)
	}
	t.Cleanup(func() {
		if closeErr := arquivo.Close(); closeErr != nil {
			t.Errorf("fechar fixture %s: %v", nome, closeErr)
		}
	})

	signals, err := normalizer.ParseOTLPJSON(context.Background(), arquivo)
	if err != nil {
		t.Fatalf("normalizar fixture %s: %v", nome, err)
	}
	if len(signals) == 0 {
		t.Fatalf("fixture %s não produziu sinais", nome)
	}
	return signals
}
