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
	assertMigrationVersion(t, database, signalsByServiceTimestampIndexVersion, 1)
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

// TestMigrateUpgradesVersionOneDatabaseWithSignalLookupIndex garante que bancos já
// instalados recebem o índice da consulta de sinais sem recriar sua estrutura.
func TestMigrateUpgradesVersionOneDatabaseWithSignalLookupIndex(t *testing.T) {
	t.Parallel()

	database := openMigrationTestDatabase(t)
	ctx := context.Background()
	if err := createVersionOneSchema(ctx, database); err != nil {
		t.Fatalf("create version one schema: %v", err)
	}

	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	assertMigrationVersion(t, database, initialSchemaVersion, 1)
	assertMigrationVersion(t, database, signalsByServiceTimestampIndexVersion, 1)
	assertIndexExists(t, database, "idx_signals_service_name_timestamp_id")
}

// TestMigrateIsIdempotentWhenSchemaIsAlreadyApplied garante que reiniciar o processo não reaplica migrations concluídas.
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
	assertMigrationVersion(t, database, signalsByServiceTimestampIndexVersion, 1)
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

func createVersionOneSchema(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}

	for _, statement := range initialSchemaStatements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	_, err := database.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, initialSchemaVersion)
	return err
}

func assertIndexExists(t *testing.T, database *sql.DB, index string) {
	t.Helper()

	var name string
	err := database.QueryRowContext(
		context.Background(),
		"SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?",
		index,
	).Scan(&name)
	if err == sql.ErrNoRows {
		t.Fatalf("index %q does not exist", index)
	}
	if err != nil {
		t.Fatalf("query index %q: %v", index, err)
	}
}
