package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const initialSchemaVersion = 1

// Migrate aplica o schema SQLite exigido pelo primeiro MVP do Faultmap.
func Migrate(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	var applied bool
	if err := database.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)`,
		initialSchemaVersion,
	).Scan(&applied); err != nil {
		return fmt.Errorf("check migration version: %w", err)
	}
	if applied {
		return nil
	}

	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin initial migration: %w", err)
	}
	defer func() { _ = transaction.Rollback() }()

	for _, statement := range initialSchemaStatements {
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply initial migration: %w", err)
		}
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, initialSchemaVersion); err != nil {
		return fmt.Errorf("record initial migration: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit initial migration: %w", err)
	}

	return nil
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
