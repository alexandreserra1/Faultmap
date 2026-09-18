package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/faultmap/faultmap/internal/storage/sqlite"
	"github.com/faultmap/faultmap/internal/storage/storagetest"
)

// TestConformidadeSQLite roda a bateria compartilhada contra o backend padrão.
//
// Ela é redundante com os testes desta pasta de propósito: o SQLite é a
// referência de comportamento, então a bateria só serve para cobrar o
// PostgreSQL depois de ter passado aqui. Uma falha neste teste significa que a
// bateria descreve algo que o produto nunca fez — e não que o SQLite regrediu.
func TestConformidadeSQLite(t *testing.T) {
	t.Parallel()

	storagetest.RodarConformidade(t, func(t *testing.T) storagetest.Backend {
		database, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "faultmap.db"))
		if err != nil {
			t.Fatalf("Open() erro = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := database.Close(); closeErr != nil {
				t.Errorf("fechar banco: %v", closeErr)
			}
		})
		if err := sqlite.Migrate(context.Background(), database); err != nil {
			t.Fatalf("Migrate() erro = %v", err)
		}
		return storagetest.Backend{
			Sinais:      sqlite.NewSignalRepository(database),
			Escopo:      sqlite.NewScopeRepository(database),
			Mudancas:    sqlite.NewChangeRepository(database),
			Catalogo:    sqlite.NewSchemaRepository(database),
			Diagnostico: sqlite.NewDiagnosisRepository(database),
			// Leva a base ao estado que a política impede, para exercitar a
			// defesa em profundidade. É SQL direto porque não existe caminho
			// legítimo até este estado, e abrir um método de produção só para o
			// teste colocaria em produção código que só o teste usa.
			LiberarTodosOsCatalogos: func(ctx context.Context, databaseName string) error {
				_, err := database.ExecContext(ctx,
					`UPDATE schema_snapshots SET objects_json = '' WHERE database_name = ?`,
					databaseName)
				return err
			},
			Retencao: sqlite.NewRetentionRepository(database),
		}
	})
}
