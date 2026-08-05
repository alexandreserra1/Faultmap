package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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
	assertMigrationVersion(t, database, signalsByTraceTimestampIndexVersion, 1)
	assertMigrationVersion(t, database, diagnosisForeignKeysVersion, 1)
	assertMigrationVersion(t, database, diagnosisSnapshotMetadataVersion, 1)
	assertMigrationVersion(t, database, diagnosisReadIndexesVersion, 1)
	assertMigrationVersion(t, database, deploymentsLookupIndexVersion, 1)
	assertMigrationVersion(t, database, compactDeploymentsLookupIndexVersion, 1)
	assertIndexExists(t, database, "idx_signals_trace_id_timestamp_id")
	assertIndexExists(t, database, "idx_incidents_status_started_at_id")
	assertIndexExists(t, database, "idx_findings_incident_id_rule_id_id")
	assertIndexExists(t, database, "idx_ranking_results_incident_id")
	assertIndexExists(t, database, "idx_deployments_service_environment_time_id")
	assertIndexColumns(t, database, "idx_deployments_service_environment_time_id", []string{"service_name", "environment", "deployed_at", "id"})
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
	if _, err := database.ExecContext(ctx, `
		INSERT INTO signals (
			id, signal_type, service_name, timestamp, trace_id, span_id, severity,
			attributes_json, measurements_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "existing-signal", "span", "checkout-service", time.Unix(1, 0).UTC(), "existing-trace", "existing-span", "info", `{}`, `{}`); err != nil {
		t.Fatalf("insert populated signal before upgrade: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO incidents (id, service_name, environment, started_at, ended_at, status)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "existing-incident", "checkout-service", "", time.Unix(1, 0).UTC(), time.Unix(2, 0).UTC(), "diagnosed"); err != nil {
		t.Fatalf("insert populated incident before upgrade: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO findings (
			id, incident_id, rule_id, subject_id, score, confidence,
			evidence_json, limitations_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "existing-finding", "existing-incident", "rule", "checkout-service", 0.5, "alta", `[]`, `[]`); err != nil {
		t.Fatalf("insert populated finding before upgrade: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO ranking_results (id, incident_id, generated_at, suspects_json)
		VALUES (?, ?, ?, ?)
	`, "existing-ranking", "existing-incident", time.Unix(2, 0).UTC(), `[]`); err != nil {
		t.Fatalf("insert populated ranking before upgrade: %v", err)
	}

	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	assertMigrationVersion(t, database, initialSchemaVersion, 1)
	assertMigrationVersion(t, database, signalsByServiceTimestampIndexVersion, 1)
	assertMigrationVersion(t, database, signalsByTraceTimestampIndexVersion, 1)
	assertMigrationVersion(t, database, diagnosisForeignKeysVersion, 1)
	assertMigrationVersion(t, database, diagnosisSnapshotMetadataVersion, 1)
	assertMigrationVersion(t, database, diagnosisReadIndexesVersion, 1)
	assertMigrationVersion(t, database, deploymentsLookupIndexVersion, 1)
	assertMigrationVersion(t, database, compactDeploymentsLookupIndexVersion, 1)
	assertIndexExists(t, database, "idx_signals_service_name_timestamp_id")
	assertIndexExists(t, database, "idx_signals_trace_id_timestamp_id")
	assertIndexExists(t, database, "idx_incidents_status_started_at_id")
	assertIndexExists(t, database, "idx_findings_incident_id_rule_id_id")
	assertIndexExists(t, database, "idx_ranking_results_incident_id")
	assertIndexExists(t, database, "idx_deployments_service_environment_time_id")
	var preservedTraceID string
	if err := database.QueryRowContext(ctx, `SELECT trace_id FROM signals WHERE id = ?`, "existing-signal").Scan(&preservedTraceID); err != nil {
		t.Fatalf("read signal after upgrade: %v", err)
	}
	if preservedTraceID != "existing-trace" {
		t.Fatalf("trace after upgrade = %q, want existing-trace", preservedTraceID)
	}
	if got := migrationRowCount(t, database, "findings"); got != 1 {
		t.Fatalf("findings after upgrade = %d, want 1", got)
	}
	if got := migrationRowCount(t, database, "ranking_results"); got != 1 {
		t.Fatalf("ranking_results after upgrade = %d, want 1", got)
	}
}

func migrationRowCount(t *testing.T, database *sql.DB, table string) int {
	t.Helper()
	allowed := map[string]struct{}{"findings": {}, "ranking_results": {}}
	if _, ok := allowed[table]; !ok {
		t.Fatalf("table %q is not allowed", table)
	}
	var count int
	if err := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
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
	assertMigrationVersion(t, database, signalsByTraceTimestampIndexVersion, 1)
	assertMigrationVersion(t, database, diagnosisForeignKeysVersion, 1)
	assertMigrationVersion(t, database, diagnosisSnapshotMetadataVersion, 1)
	assertMigrationVersion(t, database, diagnosisReadIndexesVersion, 1)
	assertMigrationVersion(t, database, deploymentsLookupIndexVersion, 1)
	assertMigrationVersion(t, database, compactDeploymentsLookupIndexVersion, 1)
	assertTableExists(t, database, "signals")
	assertIndexExists(t, database, "idx_signals_trace_id_timestamp_id")
	assertIndexExists(t, database, "idx_incidents_status_started_at_id")
	assertIndexExists(t, database, "idx_findings_incident_id_rule_id_id")
	assertIndexExists(t, database, "idx_ranking_results_incident_id")
	assertIndexExists(t, database, "idx_deployments_service_environment_time_id")
}

// TestDiagnosisReadQueriesUsePurposeBuiltIndexes garante que as consultas do
// histórico não façam full scan nem ordenação temporária conforme o volume cresce.
func TestDiagnosisReadQueriesUsePurposeBuiltIndexes(t *testing.T) {
	t.Parallel()

	database := openMigrationTestDatabase(t)
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	tests := []struct {
		name      string
		query     string
		arguments []any
		wantIndex string
	}{
		{
			name: "list incidents",
			query: `
				SELECT id, service_name, status, started_at, ended_at
				FROM incidents
				WHERE status = ? AND ended_at IS NOT NULL
				ORDER BY started_at DESC, id ASC
				LIMIT ?
			`,
			arguments: []any{"diagnosed", 20},
			wantIndex: "idx_incidents_status_started_at_id",
		},
		{
			name: "get incident",
			query: `
				SELECT id, service_name, status, started_at, ended_at,
					baseline_start, baseline_end, baseline_signal_count, incident_signal_count
				FROM incidents
				WHERE id = ? AND status = ? AND ended_at IS NOT NULL
			`,
			arguments: []any{"incident-id", "diagnosed"},
			wantIndex: "sqlite_autoindex_incidents_1",
		},
		{
			name: "get findings",
			query: `
				SELECT rule_id, subject_id, score, confidence, evidence_json, limitations_json
				FROM findings
				WHERE incident_id = ?
				ORDER BY rule_id ASC, id ASC
				LIMIT ?
			`,
			arguments: []any{"incident-id", maxFindingsPerIncident + 1},
			wantIndex: "idx_findings_incident_id_rule_id_id",
		},
		{
			name: "get ranking",
			query: `
				SELECT suspects_json
				FROM ranking_results
				WHERE incident_id = ?
			`,
			arguments: []any{"incident-id"},
			wantIndex: "idx_ranking_results_incident_id",
		},
		{
			name: "list deployments",
			query: `
				SELECT id, repository, environment, service_name, commit_sha, deployed_at, metadata_json
				FROM deployments
				WHERE service_name = ? AND environment = ? AND deployed_at >= ? AND deployed_at < ?
				ORDER BY deployed_at DESC, id ASC
				LIMIT ?
			`,
			arguments: []any{
				"checkout-service", "staging",
				time.Date(2025, time.December, 1, 9, 0, 0, 0, time.UTC),
				time.Date(2025, time.December, 1, 10, 0, 0, 0, time.UTC),
				100,
			},
			wantIndex: "idx_deployments_service_environment_time_id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := queryPlan(t, database, test.query, test.arguments...)
			if !strings.Contains(plan, "USING ") || !strings.Contains(plan, test.wantIndex) {
				t.Fatalf("query plan = %q, want index %q", plan, test.wantIndex)
			}
			if strings.Contains(plan, "USE TEMP B-TREE") {
				t.Fatalf("query plan uses temporary sorting: %q", plan)
			}
			if strings.Contains(plan, "SCAN ") {
				t.Fatalf("query plan performs full scan: %q", plan)
			}
		})
	}
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

func assertIndexColumns(t *testing.T, database *sql.DB, index string, expected []string) {
	t.Helper()
	rows, err := database.QueryContext(context.Background(), `SELECT name FROM pragma_index_info(?) ORDER BY seqno`, index)
	if err != nil {
		t.Fatalf("ler colunas do índice %q: %v", index, err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatalf("ler coluna do índice %q: %v", index, err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("percorrer colunas do índice %q: %v", index, err)
	}
	if !reflect.DeepEqual(columns, expected) {
		t.Fatalf("colunas do índice %q = %#v, esperado %#v", index, columns, expected)
	}
}

func queryPlan(t *testing.T, database *sql.DB, query string, arguments ...any) string {
	t.Helper()

	rows, err := database.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, arguments...)
	if err != nil {
		t.Fatalf("explain query plan: %v", err)
	}
	defer rows.Close()

	var details []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan query plan: %v", err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate query plan: %v", err)
	}
	return strings.Join(details, "; ")
}
