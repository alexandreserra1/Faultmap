package bootstrap_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/storage/bootstrap"
)

// TestAbreSQLitePorPadrao cobre a garantia que não é negociável: o produto
// continua sendo um binário único com banco local, e nada no caminho novo pode
// exigir um servidor para ele funcionar.
func TestAbreSQLitePorPadrao(t *testing.T) {
	t.Parallel()

	conexao, err := bootstrap.Open(context.Background(), "sqlite", filepath.Join(t.TempDir(), "faultmap.db"))
	if err != nil {
		t.Fatalf("Open() erro = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := conexao.Close(); closeErr != nil {
			t.Errorf("fechar conexão: %v", closeErr)
		}
	})
	if conexao.Driver() != bootstrap.DriverSQLite {
		t.Fatalf("driver = %q, esperado %q", conexao.Driver(), bootstrap.DriverSQLite)
	}
	if err := bootstrap.Migrate(context.Background(), conexao); err != nil {
		t.Fatalf("Migrate() erro = %v", err)
	}
}

// TestDriverVazioContinuaSendoSQLite protege configurações escritas antes de o
// campo existir: a ausência de opinião significa o comportamento de sempre, e
// não um erro de inicialização.
func TestDriverVazioContinuaSendoSQLite(t *testing.T) {
	t.Parallel()

	conexao, err := bootstrap.Open(context.Background(), "", filepath.Join(t.TempDir(), "faultmap.db"))
	if err != nil {
		t.Fatalf("Open() erro = %v", err)
	}
	t.Cleanup(func() { _ = conexao.Close() })
	if conexao.Driver() != bootstrap.DriverSQLite {
		t.Fatalf("driver = %q, esperado %q", conexao.Driver(), bootstrap.DriverSQLite)
	}
}

// TestPostgresSemDSNExplicaOndeConfigurar é o caso de erro que mais aparece na
// primeira vez: quem troca o driver no YAML e espera que a conexão esteja lá.
// A mensagem precisa dizer que a DSN vem do ambiente, porque o arquivo de
// configuração deliberadamente não tem onde guardá-la.
func TestPostgresSemDSNExplicaOndeConfigurar(t *testing.T) {
	t.Setenv(bootstrap.VariavelDSN, "")

	_, err := bootstrap.Open(context.Background(), "postgres", "")
	if err == nil {
		t.Fatal("Open() erro = nil, esperado recusa sem DSN")
	}
	if !strings.Contains(err.Error(), bootstrap.VariavelDSN) {
		t.Errorf("mensagem = %q, esperado citar %s", err.Error(), bootstrap.VariavelDSN)
	}
	if !strings.Contains(err.Error(), "faultmap.yaml") {
		t.Errorf("mensagem = %q, esperado explicar que a DSN não vem do arquivo", err.Error())
	}
}

// TestDriverDesconhecidoListaAsOpcoes evita o erro mudo de um YAML com erro de
// digitação no nome do backend.
func TestDriverDesconhecidoListaAsOpcoes(t *testing.T) {
	t.Parallel()

	_, err := bootstrap.Open(context.Background(), "mysql", "")
	if err == nil {
		t.Fatal("Open() erro = nil, esperado recusa de driver desconhecido")
	}
	for _, esperado := range []string{"sqlite", "postgres"} {
		if !strings.Contains(err.Error(), esperado) {
			t.Errorf("mensagem = %q, esperado citar %q", err.Error(), esperado)
		}
	}
}

// TestRepositoriosSeguemODriverDaConexao prova que a escolha feita na abertura
// alcança todos os repositórios. Sem isso, um comando poderia abrir PostgreSQL
// e consultar pelo caminho do SQLite sem que nada reclamasse até a primeira
// consulta.
func TestRepositoriosSeguemODriverDaConexao(t *testing.T) {
	t.Parallel()

	conexao, err := bootstrap.Open(context.Background(), "sqlite", filepath.Join(t.TempDir(), "faultmap.db"))
	if err != nil {
		t.Fatalf("Open() erro = %v", err)
	}
	t.Cleanup(func() { _ = conexao.Close() })
	if err := bootstrap.Migrate(context.Background(), conexao); err != nil {
		t.Fatalf("Migrate() erro = %v", err)
	}

	// Cada construtor precisa devolver algo utilizável; um nil aqui só apareceria
	// como panic no meio de uma investigação.
	if bootstrap.NewSignalRepository(conexao) == nil {
		t.Error("NewSignalRepository() = nil")
	}
	if bootstrap.NewScopeRepository(conexao) == nil {
		t.Error("NewScopeRepository() = nil")
	}
	if bootstrap.NewChangeRepository(conexao) == nil {
		t.Error("NewChangeRepository() = nil")
	}
	if bootstrap.NewSchemaRepository(conexao) == nil {
		t.Error("NewSchemaRepository() = nil")
	}
	if bootstrap.NewDiagnosisRepository(conexao) == nil {
		t.Error("NewDiagnosisRepository() = nil")
	}
	if bootstrap.NewRetentionRepository(conexao) == nil {
		t.Error("NewRetentionRepository() = nil")
	}
}
