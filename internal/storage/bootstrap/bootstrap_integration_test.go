package bootstrap_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/storage/bootstrap"
	"github.com/faultmap/faultmap/internal/storage/postgres"
	"github.com/faultmap/faultmap/internal/storage/storagetest"
)

// TestIntegracaoBootstrapEntregaBackendPostgresCompleto fecha o circuito entre
// a configuração e o banco.
//
// Os testes de unidade deste pacote provam a escolha do driver, e a bateria em
// internal/storage/postgres prova o comportamento do adaptador — mas nenhum dos
// dois prova que o caminho que a CLI percorre de fato chega lá. Um construtor
// que devolvesse o repositório SQLite mesmo com driver postgres passaria em
// ambos e falharia aqui.
func TestIntegracaoBootstrapEntregaBackendPostgresCompleto(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("FAULTMAP_TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("FAULTMAP_TEST_PG_DSN não definido; integração do bootstrap ignorada")
	}

	storagetest.RodarConformidade(t, func(t *testing.T) storagetest.Backend {
		schema := fmt.Sprintf("faultmap_boot_%d", time.Now().UnixNano())
		criarSchema(t, dsn, schema)

		// A DSN entra pelo ambiente, exatamente como em produção: é a única porta
		// por onde ela pode entrar, já que o faultmap.yaml não tem campo para
		// credenciais.
		t.Setenv(bootstrap.VariavelDSN, comSearchPath(dsn, schema))

		// O caminho do SQLite é passado e ignorado, como acontece na CLI: o
		// bootstrap recebe sempre os dois e decide pelo driver.
		conexao, err := bootstrap.Open(context.Background(), "postgres", "/caminho/ignorado.db")
		if err != nil {
			t.Fatalf("Open() erro = %v", err)
		}
		if conexao.Driver() != bootstrap.DriverPostgres {
			t.Fatalf("driver = %q, esperado %q", conexao.Driver(), bootstrap.DriverPostgres)
		}
		t.Cleanup(func() {
			if closeErr := conexao.Close(); closeErr != nil {
				t.Errorf("fechar conexão: %v", closeErr)
			}
			removerSchema(t, dsn, schema)
		})
		if err := bootstrap.Migrate(context.Background(), conexao); err != nil {
			t.Fatalf("Migrate() erro = %v", err)
		}

		return storagetest.Backend{
			Sinais:      bootstrap.NewSignalRepository(conexao),
			Escopo:      bootstrap.NewScopeRepository(conexao),
			Mudancas:    bootstrap.NewChangeRepository(conexao),
			Catalogo:    bootstrap.NewSchemaRepository(conexao),
			Diagnostico: bootstrap.NewDiagnosisRepository(conexao),
			Retencao:    bootstrap.NewRetentionRepository(conexao),
		}
	})
}

func criarSchema(t *testing.T, dsn, schema string) {
	t.Helper()
	executarAdministrativo(t, dsn, "CREATE SCHEMA "+schema)
}

func removerSchema(t *testing.T, dsn, schema string) {
	t.Helper()
	executarAdministrativo(t, dsn, "DROP SCHEMA "+schema+" CASCADE")
}

// executarAdministrativo abre uma conexão na DSN base — fora do schema do teste
// — para criar e remover o schema isolado que dá estado limpo a cada caso.
func executarAdministrativo(t *testing.T, dsn, comando string) {
	t.Helper()

	database, err := postgres.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("abrir conexão administrativa: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar conexão administrativa: %v", closeErr)
		}
	}()
	if _, err := database.ExecContext(context.Background(), comando); err != nil {
		t.Fatalf("%s: %v", comando, err)
	}
}

func comSearchPath(dsn, schema string) string {
	separador := "?"
	if strings.Contains(dsn, "?") {
		separador = "&"
	}
	return dsn + separador + "search_path=" + schema
}
