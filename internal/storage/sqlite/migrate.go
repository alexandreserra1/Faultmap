package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const (
	initialSchemaVersion                  = 1
	signalsByServiceTimestampIndexVersion = 2
)

type migration struct {
	version    int
	statements []string
}

// Migrate aplica, em ordem, as migrations versionadas exigidas pelo Faultmap.
// Cada migration é uma transação curta para impedir que o banco registre uma
// versão cujo schema não tenha sido criado integralmente.
func Migrate(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	for _, migration := range migrations {
		applied, err := isMigrationApplied(ctx, database, migration.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, database, migration); err != nil {
			return err
		}
	}

	return nil
}

// isMigrationApplied verifica se a migration já foi concluída sem inferir o
// estado a partir do schema, que pode estar incompleto após uma falha externa.
func isMigrationApplied(ctx context.Context, database *sql.DB, version int) (bool, error) {
	var applied bool
	if err := database.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)`,
		version,
	).Scan(&applied); err != nil {
		return false, fmt.Errorf("check migration version %d: %w", version, err)
	}
	return applied, nil
}

// applyMigration executa uma única migration e só registra sua versão depois
// que todos os DDLs obrigatórios foram aceitos pelo SQLite.
func applyMigration(ctx context.Context, database *sql.DB, migration migration) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", migration.version, err)
	}

	for _, statement := range migration.statements {
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			return rollbackMigration(transaction, migration.version, fmt.Errorf("apply migration: %w", err))
		}
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, migration.version); err != nil {
		return rollbackMigration(transaction, migration.version, fmt.Errorf("record migration: %w", err))
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", migration.version, err)
	}
	return nil
}

// rollbackMigration preserva a causa da falha e relata um rollback inesperado.
func rollbackMigration(transaction *sql.Tx, version int, cause error) error {
	if rollbackErr := transaction.Rollback(); rollbackErr != nil {
		return fmt.Errorf("migration %d failed: %w; rollback migration: %v", version, cause, rollbackErr)
	}
	return fmt.Errorf("migration %d failed: %w", version, cause)
}

var migrations = []migration{
	{version: initialSchemaVersion, statements: initialSchemaStatements},
	{
		version: signalsByServiceTimestampIndexVersion,
		statements: []string{
			`CREATE INDEX IF NOT EXISTS idx_signals_service_name_timestamp_id
				ON signals (service_name, timestamp, id)`,
		},
	},
}

var initialSchemaStatements = []string{
	`CREATE TABLE signals (
		id TEXT PRIMARY KEY,
		signal_type TEXT NOT NULL,
		service_name TEXT,
		timestamp DATETIME NOT NULL,
		trace_id TEXT,
		span_id TEXT,
		severity TEXT,
		attributes_json TEXT NOT NULL,
		measurements_json TEXT NOT NULL
	)`,
	`CREATE TABLE incidents (
		id TEXT PRIMARY KEY,
		service_name TEXT NOT NULL,
		environment TEXT,
		started_at DATETIME NOT NULL,
		ended_at DATETIME,
		status TEXT NOT NULL
	)`,
	`CREATE TABLE findings (
		id TEXT PRIMARY KEY,
		incident_id TEXT NOT NULL,
		rule_id TEXT NOT NULL,
		subject_id TEXT NOT NULL,
		score REAL NOT NULL,
		confidence TEXT NOT NULL,
		evidence_json TEXT NOT NULL,
		limitations_json TEXT NOT NULL
	)`,
	`CREATE TABLE evidence_nodes (
		id TEXT PRIMARY KEY,
		node_type TEXT NOT NULL,
		label TEXT NOT NULL,
		attributes_json TEXT NOT NULL
	)`,
	`CREATE TABLE evidence_edges (
		id TEXT PRIMARY KEY,
		source_id TEXT NOT NULL,
		target_id TEXT NOT NULL,
		relation TEXT NOT NULL,
		confidence REAL NOT NULL,
		evidence_ids_json TEXT NOT NULL
	)`,
	`CREATE TABLE deployments (
		id TEXT PRIMARY KEY,
		repository TEXT NOT NULL,
		environment TEXT,
		service_name TEXT,
		commit_sha TEXT,
		deployed_at DATETIME NOT NULL,
		metadata_json TEXT NOT NULL
	)`,
	`CREATE TABLE commits (
		sha TEXT PRIMARY KEY,
		repository TEXT NOT NULL,
		author TEXT,
		message TEXT,
		committed_at DATETIME NOT NULL,
		files_json TEXT NOT NULL
	)`,
	`CREATE TABLE ranking_results (
		id TEXT PRIMARY KEY,
		incident_id TEXT NOT NULL,
		generated_at DATETIME NOT NULL,
		suspects_json TEXT NOT NULL
	)`,
}
