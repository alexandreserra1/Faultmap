package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/storage/postgres"
	"github.com/faultmap/faultmap/internal/storage/storagetest"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestConformidadeIntegracaoPostgres roda a bateria compartilhada contra um
// PostgreSQL de verdade.
//
// Ela precisa de um servidor porque é justamente o que um mock não prova: que a
// tradução do dialeto produz SQL válido. Metade das diferenças entre os dois
// backends — o apelido obrigatório da subconsulta no FROM, a numeração dos
// parâmetros, o fuso em que o TIMESTAMPTZ volta — passariam por qualquer mock
// e falhariam no primeiro uso real.
//
// Sem FAULTMAP_TEST_PG_DSN o teste se ignora, o que mantém `make verify` verde
// em uma máquina sem Docker.
func TestConformidadeIntegracaoPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("FAULTMAP_TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("FAULTMAP_TEST_PG_DSN não definido; conformidade PostgreSQL ignorada")
	}

	storagetest.RodarConformidade(t, func(t *testing.T) storagetest.Backend {
		database := abrirEmSchemaIsolado(t, dsn)
		if err := postgres.Migrate(context.Background(), database); err != nil {
			t.Fatalf("Migrate() erro = %v", err)
		}
		return storagetest.Backend{
			Sinais:      postgres.NewSignalRepository(database),
			Escopo:      postgres.NewScopeRepository(database),
			Mudancas:    postgres.NewChangeRepository(database),
			Catalogo:    postgres.NewSchemaRepository(database),
			Diagnostico: postgres.NewDiagnosisRepository(database),
			// Leva a base ao estado que a política impede, para exercitar a
			// defesa em profundidade. É SQL direto porque não existe caminho
			// legítimo até este estado, e abrir um método de produção só para o
			// teste colocaria em produção código que só o teste usa.
			LiberarTodosOsCatalogos: func(ctx context.Context, databaseName string) error {
				_, err := database.ExecContext(ctx,
					`UPDATE schema_snapshots SET objects_json = '' WHERE database_name = $1`,
					databaseName)
				return err
			},
			Retencao: postgres.NewRetentionRepository(database),
		}
	})
}

// abrirEmSchemaIsolado dá a cada caso um schema próprio dentro da mesma base.
//
// A bateria assume um backend vazio a cada chamada da fábrica, e criar uma base
// por caso custaria caro. Um schema por caso entrega o mesmo isolamento: o
// search_path faz `CREATE TABLE signals` cair no schema do teste, e o DROP
// CASCADE no fim leva tudo embora.
func abrirEmSchemaIsolado(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	nome := fmt.Sprintf("faultmap_conf_%d", time.Now().UnixNano())
	administracao, err := postgres.Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("Open() administrativo erro = %v", err)
	}
	defer func() {
		if closeErr := administracao.Close(); closeErr != nil {
			t.Errorf("fechar conexão administrativa: %v", closeErr)
		}
	}()
	if _, err := administracao.ExecContext(context.Background(), "CREATE SCHEMA "+nome); err != nil {
		t.Fatalf("criar schema %q: %v", nome, err)
	}

	database, err := postgres.Open(context.Background(), comSearchPath(dsn, nome))
	if err != nil {
		t.Fatalf("Open() erro = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar banco: %v", closeErr)
		}
		limpeza, err := postgres.Open(context.Background(), dsn)
		if err != nil {
			t.Errorf("abrir conexão de limpeza: %v", err)
			return
		}
		defer func() { _ = limpeza.Close() }()
		if _, err := limpeza.ExecContext(context.Background(), "DROP SCHEMA "+nome+" CASCADE"); err != nil {
			t.Errorf("remover schema %q: %v", nome, err)
		}
	})
	return database
}

// comSearchPath acrescenta o search_path à DSN. O pgx repassa parâmetros
// desconhecidos da URL como parâmetros de runtime da sessão, então o schema
// isolado vale para toda conexão do pool.
func comSearchPath(dsn, schema string) string {
	separador := "?"
	if strings.Contains(dsn, "?") {
		separador = "&"
	}
	return dsn + separador + "search_path=" + schema
}
