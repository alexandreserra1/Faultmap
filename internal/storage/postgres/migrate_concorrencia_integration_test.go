package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/faultmap/faultmap/internal/storage/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestMigrateIntegracaoSuportaProcessosSimultaneos cobre o cenário que o
// PostgreSQL torna normal e o SQLite tornava improvável.
//
// O banco é compartilhado por definição — é o motivo de a ADR 0016 existir — e
// os comandos da CLI migram ao iniciar. Dois processos subindo juntos contra um
// banco sem a migration N passariam ambos pela verificação e executariam o mesmo
// DDL; um aborta com "relation already exists" ou com chave duplicada em
// schema_migrations, falhando um comando que deveria ter funcionado.
//
// Cada conexão aqui é independente, como processos distintos seriam: um pool
// compartilhado esconderia a corrida.
func TestMigrateIntegracaoSuportaProcessosSimultaneos(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("FAULTMAP_TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("FAULTMAP_TEST_PG_DSN não definida; migração concorrente ignorada")
	}

	limpar := func() {
		limpeza, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatalf("sql.Open() erro = %v", err)
		}
		defer func() { _ = limpeza.Close() }()
		for _, tabela := range []string{
			"schema_changes", "schema_snapshots", "ranking_results", "findings",
			"evidence_edges", "evidence_nodes", "incidents", "deployments",
			"commits", "signals", "schema_migrations",
		} {
			if _, err := limpeza.ExecContext(context.Background(),
				"DROP TABLE IF EXISTS "+tabela+" CASCADE"); err != nil {
				t.Fatalf("limpar %s: %v", tabela, err)
			}
		}
	}
	limpar()
	t.Cleanup(limpar)

	const processos = 6
	var grupo sync.WaitGroup
	erros := make(chan error, processos)
	for range processos {
		grupo.Add(1)
		go func() {
			defer grupo.Done()
			conexao, err := sql.Open("pgx", dsn)
			if err != nil {
				erros <- err
				return
			}
			defer func() { _ = conexao.Close() }()
			if err := postgres.Migrate(context.Background(), conexao); err != nil {
				erros <- err
			}
		}()
	}
	grupo.Wait()
	close(erros)
	for err := range erros {
		t.Fatalf("Migrate() simultâneo erro = %v", err)
	}

	// E o resultado precisa ser um schema migrado uma vez só.
	conferencia, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open() erro = %v", err)
	}
	defer func() { _ = conferencia.Close() }()
	var distintas, linhas int
	if err := conferencia.QueryRowContext(context.Background(),
		`SELECT COUNT(DISTINCT version), COUNT(*) FROM schema_migrations`).Scan(&distintas, &linhas); err != nil {
		t.Fatalf("contar migrations: %v", err)
	}
	if distintas != linhas {
		t.Fatalf("schema_migrations tem %d linhas para %d versões distintas: migration aplicada duas vezes",
			linhas, distintas)
	}
	if distintas == 0 {
		t.Fatal("nenhuma migration registrada; o teste não exercitou nada")
	}
}
