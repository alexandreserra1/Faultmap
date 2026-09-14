package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// As versões são as mesmas do backend SQLite, e a igualdade é deliberada: a
// tabela schema_migrations de um banco e de outro precisa poder ser comparada
// ao investigar uma divergência. Uma migration aplicada aqui e não lá teria o
// mesmo número em ambos ou nenhum dos dois.
const (
	initialSchemaVersion                  = 1
	signalsByServiceTimestampIndexVersion = 2
	signalsByTraceTimestampIndexVersion   = 3
	diagnosisForeignKeysVersion           = 4
	diagnosisSnapshotMetadataVersion      = 5
	diagnosisReadIndexesVersion           = 6
	deploymentsLookupIndexVersion         = 7
	compactDeploymentsLookupIndexVersion  = 8
	schemaCatalogVersion                  = 9
	schemaChangesCarryTableVersion        = 10
)

type migration struct {
	version    int
	statements []string
}

// Migrate aplica, em ordem, as migrations versionadas exigidas pelo Faultmap.
// Cada migration é uma transação curta para impedir que o banco registre uma
// versão que não chegou a ser aplicada por inteiro.
func Migrate(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("criar tabela de migrations: %w", err)
	}

	for _, migracao := range migrations {
		aplicada, err := isMigrationApplied(ctx, database, migracao.version)
		if err != nil {
			return err
		}
		if aplicada {
			continue
		}
		if err := applyMigration(ctx, database, migracao); err != nil {
			return err
		}
	}
	return nil
}

// isMigrationApplied verifica se a migration já foi concluída sem inferir o
// estado a partir da existência das tabelas: só a versão registrada prova que a
// transação inteira foi confirmada.
func isMigrationApplied(ctx context.Context, database *sql.DB, version int) (bool, error) {
	var aplicada bool
	if err := database.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`,
		version,
	).Scan(&aplicada); err != nil {
		return false, fmt.Errorf("verificar migration %d: %w", version, err)
	}
	return aplicada, nil
}

// applyMigration executa uma única migration e só registra sua versão depois
// que todos os comandos dela passaram.
func applyMigration(ctx context.Context, database *sql.DB, migracao migration) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar migration %d: %w", migracao.version, err)
	}
	for _, comando := range migracao.statements {
		if _, err := transaction.ExecContext(ctx, comando); err != nil {
			return rollbackMigration(transaction, migracao.version, fmt.Errorf("aplicar migration: %w", err))
		}
	}
	if _, err := transaction.ExecContext(
		ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, migracao.version,
	); err != nil {
		return rollbackMigration(transaction, migracao.version, fmt.Errorf("registrar migration: %w", err))
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("confirmar migration %d: %w", migracao.version, err)
	}
	return nil
}

// rollbackMigration encerra a migration ao primeiro erro preservando a causa.
func rollbackMigration(transaction *sql.Tx, version int, causa error) error {
	if rollbackErr := transaction.Rollback(); rollbackErr != nil {
		return fmt.Errorf("migration %d falhou: %w; desfazer migration: %v", version, causa, rollbackErr)
	}
	return fmt.Errorf("migration %d falhou: %w", version, causa)
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
	{
		version: signalsByTraceTimestampIndexVersion,
		statements: []string{
			`CREATE INDEX IF NOT EXISTS idx_signals_trace_id_timestamp_id
				ON signals (trace_id, timestamp, id)`,
		},
	},
	{
		// No SQLite esta migration reescreve findings e ranking_results inteiras,
		// porque aquele banco não sabe adicionar uma foreign key a uma tabela
		// existente. Aqui um ALTER basta. O resultado observável é o mesmo — e é
		// o resultado, não o caminho, que a bateria de conformidade compara.
		version: diagnosisForeignKeysVersion,
		statements: []string{
			`ALTER TABLE findings
				ADD CONSTRAINT fk_findings_incident
				FOREIGN KEY (incident_id) REFERENCES incidents(id) ON DELETE CASCADE`,
			`ALTER TABLE ranking_results
				ADD CONSTRAINT fk_ranking_results_incident
				FOREIGN KEY (incident_id) REFERENCES incidents(id) ON DELETE CASCADE`,
		},
	},
	{
		version: diagnosisSnapshotMetadataVersion,
		statements: []string{
			`ALTER TABLE incidents ADD COLUMN baseline_start TIMESTAMPTZ`,
			`ALTER TABLE incidents ADD COLUMN baseline_end TIMESTAMPTZ`,
			`ALTER TABLE incidents ADD COLUMN baseline_signal_count INTEGER`,
			`ALTER TABLE incidents ADD COLUMN incident_signal_count INTEGER`,
		},
	},
	{
		version: diagnosisReadIndexesVersion,
		statements: []string{
			`CREATE INDEX idx_incidents_status_started_at_id
				ON incidents (status, started_at DESC, id ASC, service_name, ended_at)`,
			`CREATE INDEX idx_findings_incident_id_rule_id_id
				ON findings (incident_id, rule_id ASC, id ASC)`,
			`CREATE UNIQUE INDEX idx_ranking_results_incident_id
				ON ranking_results (incident_id)`,
		},
	},
	{
		version: deploymentsLookupIndexVersion,
		statements: []string{
			`CREATE INDEX idx_deployments_service_environment_time_id
				ON deployments (service_name, environment, deployed_at DESC, id ASC, repository, commit_sha, metadata_json)`,
		},
	},
	{
		version: compactDeploymentsLookupIndexVersion,
		statements: []string{
			`DROP INDEX idx_deployments_service_environment_time_id`,
			`CREATE INDEX idx_deployments_service_environment_time_id
				ON deployments (service_name, environment, deployed_at DESC, id ASC)`,
		},
	},
	{
		version: schemaCatalogVersion,
		statements: []string{
			`CREATE TABLE schema_snapshots (
				id TEXT PRIMARY KEY,
				database_name TEXT NOT NULL,
				captured_at TIMESTAMPTZ NOT NULL,
				objects_json TEXT NOT NULL
			)`,
			// A leitura mais quente é "a coleta anterior desta base", feita a cada
			// nova coleta. O índice descendente a resolve com uma única linha.
			`CREATE INDEX idx_schema_snapshots_database_captured
				ON schema_snapshots (database_name, captured_at DESC, id ASC)`,
			`CREATE TABLE schema_changes (
				id TEXT PRIMARY KEY,
				database_name TEXT NOT NULL,
				object_kind TEXT NOT NULL,
				object_name TEXT NOT NULL,
				change_kind TEXT NOT NULL,
				detail TEXT NOT NULL,
				observed_after TIMESTAMPTZ NOT NULL,
				observed_before TIMESTAMPTZ NOT NULL,
				snapshot_id TEXT NOT NULL,
				-- O CASCADE é um alerta para quem for mexer na retenção: apagar uma
				-- coleta apaga as mudanças que ela revelou. Por isso schema_snapshots
				-- fica fora da política de retenção, que remove telemetria e preserva
				-- o que sustenta um diagnóstico (ADR 0003). O volume também não pede
				-- limpeza: é uma linha por coleta, não uma por span.
				FOREIGN KEY (snapshot_id) REFERENCES schema_snapshots(id) ON DELETE CASCADE
			)`,
			`CREATE INDEX idx_schema_changes_database_observed
				ON schema_changes (database_name, observed_before DESC, id ASC)`,
		},
	},
	{
		// A tabela permite ligar uma migração ao serviço quando a telemetria não
		// nomeia a base — o caso comum: medindo uma aplicação instrumentada,
		// todos os spans de banco traziam db.collection.name e nenhum trazia
		// db.namespace.
		version: schemaChangesCarryTableVersion,
		statements: []string{
			`ALTER TABLE schema_changes ADD COLUMN table_name TEXT NOT NULL DEFAULT ''`,
			`CREATE INDEX idx_schema_changes_table_observed
				ON schema_changes (table_name, observed_before DESC, id ASC)`,
		},
	},
}

var initialSchemaStatements = []string{
	`CREATE TABLE signals (
		id TEXT PRIMARY KEY,
		signal_type TEXT NOT NULL,
		service_name TEXT,
		timestamp TIMESTAMPTZ NOT NULL,
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
		started_at TIMESTAMPTZ NOT NULL,
		ended_at TIMESTAMPTZ,
		status TEXT NOT NULL
	)`,
	// score é DOUBLE PRECISION, e não REAL como no SQLite. Os dois nomes
	// enganam: REAL no SQLite já é ponto flutuante de 8 bytes, enquanto no
	// PostgreSQL são 4 — traduzir o nome ao pé da letra truncaria o score de
	// cada finding e faria dois backends ranquearem os mesmos suspeitos em
	// ordens diferentes.
	`CREATE TABLE findings (
		id TEXT PRIMARY KEY,
		incident_id TEXT NOT NULL,
		rule_id TEXT NOT NULL,
		subject_id TEXT NOT NULL,
		score DOUBLE PRECISION NOT NULL,
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
		confidence DOUBLE PRECISION NOT NULL,
		evidence_ids_json TEXT NOT NULL
	)`,
	`CREATE TABLE deployments (
		id TEXT PRIMARY KEY,
		repository TEXT NOT NULL,
		environment TEXT,
		service_name TEXT,
		commit_sha TEXT,
		deployed_at TIMESTAMPTZ NOT NULL,
		metadata_json TEXT NOT NULL
	)`,
	`CREATE TABLE commits (
		sha TEXT PRIMARY KEY,
		repository TEXT NOT NULL,
		author TEXT,
		message TEXT,
		committed_at TIMESTAMPTZ NOT NULL,
		files_json TEXT NOT NULL
	)`,
	`CREATE TABLE ranking_results (
		id TEXT PRIMARY KEY,
		incident_id TEXT NOT NULL,
		generated_at TIMESTAMPTZ NOT NULL,
		suspects_json TEXT NOT NULL
	)`,
}
