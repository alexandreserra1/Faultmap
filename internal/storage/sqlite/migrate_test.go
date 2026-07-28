package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigrateAppliesInitialSchemaOnCleanDatabase garante que uma instalação nova recebe todas as tabelas essenciais.
func TestMigrateAppliesInitialSchemaOnCleanDatabase(t *testing.T) {
	t.Parallel()

	database := openMigrationTestDatabase(t)
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	assertMigrationVersion(t, database, initialSchemaVersion, 1)
	for _, table := range []string{
		"signals",
		"incidents",
		"findings",
		"evidence_nodes",
		"evidence_edges",
		"deployments",
		"commits",
		"ranking_results",
	} {
		assertTableExists(t, database, table)
	}
}

// TestMigrateIsIdempotentWhenSchemaIsAlreadyApplied garante que reiniciar o processo não reaplica a migration inicial.
func TestMigrateIsIdempotentWhenSchemaIsAlreadyApplied(t *testing.T) {
	t.Parallel()

	database := openMigrationTestDatabase(t)
	ctx := context.Background()
	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	assertMigrationVersion(t, database, initialSchemaVersion, 1)
	assertTableExists(t, database, "signals")
}

func openMigrationTestDatabase(t *testing.T) *sql.DB {
	t.Helper()

	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "faultmap.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close database: %v", closeErr)
		}
	})
	return database
}

func assertMigrationVersion(t *testing.T, database *sql.DB, version, wantCount int) {
	t.Helper()

	var count int
	if err := database.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM schema_migrations WHERE version = ?",
		version,
	).Scan(&count); err != nil {
		t.Fatalf("count migration version: %v", err)
	}
	if count != wantCount {
		t.Fatalf("migration version %d count = %d, want %d", version, count, wantCount)
	}
}

func assertTableExists(t *testing.T, database *sql.DB, table string) {
	t.Helper()

	var name string
	err := database.QueryRowContext(
		context.Background(),
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&name)
	if err == sql.ErrNoRows {
		t.Fatalf("table %q does not exist", table)
	}
	if err != nil {
		t.Fatalf("query table %q: %v", table, err)
	}
}
