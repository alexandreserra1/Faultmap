package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
	"github.com/faultmap/faultmap/internal/ranking"
)

// TestDiagnosisRepositorySavePersistsCompleteDiagnosis garante que um diagnóstico
// seja armazenado atomicamente no pool injetado, preservando seus dados auditáveis.
func TestDiagnosisRepositorySavePersistsCompleteDiagnosis(t *testing.T) {
	t.Parallel()

	database, repository := openDiagnosisRepository(t)
	diagnosis := testDiagnosis(t, "incident-checkout-1")

	created, err := repository.Save(context.Background(), diagnosis)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !created {
		t.Fatal("Save() created = false, want true")
	}

	assertStoredIncident(t, database, diagnosis)
	assertStoredFindings(t, database, diagnosis.ID, diagnosis.Findings)
	assertStoredRanking(t, database, diagnosis.ID, diagnosis.Suspects)
}

// TestDiagnosisRepositorySaveIsIdempotent garante que retries do mesmo
// diagnóstico não dupliquem nem substituam o conjunto persistido inicialmente.
func TestDiagnosisRepositorySaveIsIdempotent(t *testing.T) {
	t.Parallel()

	database, repository := openDiagnosisRepository(t)
	diagnosis := testDiagnosis(t, "incident-checkout-retry")

	created, err := repository.Save(context.Background(), diagnosis)
	if err != nil || !created {
		t.Fatalf("first Save() = (%v, %v), want (true, nil)", created, err)
	}
	firstFindingIDs := storedIDs(t, database, "findings", "incident_id", diagnosis.ID)
	firstRankingIDs := storedIDs(t, database, "ranking_results", "incident_id", diagnosis.ID)

	changed := diagnosis
	changed.ServiceName = "valor-que-nao-deve-substituir-o-original"
	created, err = repository.Save(context.Background(), changed)
	if err != nil {
		t.Fatalf("retry Save() error = %v", err)
	}
	if created {
		t.Fatal("retry Save() created = true, want false")
	}

	if got := rowCount(t, database, "incidents"); got != 1 {
		t.Fatalf("incidents count = %d, want 1", got)
	}
	if got := rowCount(t, database, "findings"); got != len(diagnosis.Findings) {
		t.Fatalf("findings count = %d, want %d", got, len(diagnosis.Findings))
	}
	if got := rowCount(t, database, "ranking_results"); got != 1 {
		t.Fatalf("ranking_results count = %d, want 1", got)
	}
	if got := storedIDs(t, database, "findings", "incident_id", diagnosis.ID); !reflect.DeepEqual(got, firstFindingIDs) {
		t.Fatalf("finding IDs after retry = %v, want %v", got, firstFindingIDs)
	}
	if got := storedIDs(t, database, "ranking_results", "incident_id", diagnosis.ID); !reflect.DeepEqual(got, firstRankingIDs) {
		t.Fatalf("ranking IDs after retry = %v, want %v", got, firstRankingIDs)
	}
}

// TestDiagnosisRepositorySaveGeneratesStableFindingIDs garante que a identidade
// de cada finding não dependa da ordem recebida pelo repositório.
func TestDiagnosisRepositorySaveGeneratesStableFindingIDs(t *testing.T) {
	t.Parallel()

	firstDatabase, firstRepository := openDiagnosisRepository(t)
	secondDatabase, secondRepository := openDiagnosisRepository(t)
	first := testDiagnosis(t, "incident-stable-ids")
	second := testDiagnosis(t, "incident-stable-ids")
	second.Findings[0], second.Findings[1] = second.Findings[1], second.Findings[0]

	if created, err := firstRepository.Save(context.Background(), first); err != nil || !created {
		t.Fatalf("first Save() = (%v, %v), want (true, nil)", created, err)
	}
	if created, err := secondRepository.Save(context.Background(), second); err != nil || !created {
		t.Fatalf("second Save() = (%v, %v), want (true, nil)", created, err)
	}

	firstIDs := storedFindingIDsByRule(t, firstDatabase, first.ID)
	secondIDs := storedFindingIDsByRule(t, secondDatabase, second.ID)
	if !reflect.DeepEqual(firstIDs, secondIDs) {
		t.Fatalf("finding IDs depend on input order: first = %v, second = %v", firstIDs, secondIDs)
	}
}

// TestDiagnosisRepositorySaveRollsBackAllRowsOnMandatoryRecordFailure garante
// que um erro depois do incidente não deixe um diagnóstico parcial no banco.
func TestDiagnosisRepositorySaveRollsBackAllRowsOnMandatoryRecordFailure(t *testing.T) {
	t.Parallel()

	database, repository := openDiagnosisRepository(t)
	if _, err := database.ExecContext(context.Background(), `
		CREATE TRIGGER reject_diagnosis_finding
		BEFORE INSERT ON findings
		BEGIN
			SELECT RAISE(ABORT, 'finding obrigatório rejeitado pelo teste');
		END
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	created, err := repository.Save(context.Background(), testDiagnosis(t, "incident-rollback"))
	if err == nil {
		t.Fatal("Save() error = nil, want mandatory finding error")
	}
	if created {
		t.Fatal("Save() created = true after rollback, want false")
	}
	for _, table := range []string{"incidents", "findings", "ranking_results"} {
		if got := rowCount(t, database, table); got != 0 {
			t.Fatalf("%s count after rollback = %d, want 0", table, got)
		}
	}
}

// TestDiagnosisRepositorySaveRespectsCanceledContext garante que cancelamento
// seja propagado sem iniciar ou persistir uma transação parcial.
func TestDiagnosisRepositorySaveRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	database, repository := openDiagnosisRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	created, err := repository.Save(ctx, testDiagnosis(t, "incident-canceled"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Save() error = %v, want context.Canceled", err)
	}
	if created {
		t.Fatal("Save() created = true, want false")
	}
	if got := rowCount(t, database, "incidents"); got != 0 {
		t.Fatalf("incidents count = %d, want 0", got)
	}
}

// TestDiagnosisTablesEnforceIncidentForeignKeys protege a integridade mesmo
// quando uma escrita inválida tenta contornar o repositório.
func TestDiagnosisTablesEnforceIncidentForeignKeys(t *testing.T) {
	t.Parallel()

	database, _ := openDiagnosisRepository(t)
	ctx := context.Background()
	_, findingErr := database.ExecContext(ctx, `
		INSERT INTO findings (
			id, incident_id, rule_id, subject_id, score, confidence,
			evidence_json, limitations_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, "orphan-finding", "missing-incident", "rule", "checkout", 0.5, "alta", `[]`, `[]`)
	if findingErr == nil {
		t.Fatal("insert orphan finding error = nil, want foreign key violation")
	}

	_, rankingErr := database.ExecContext(ctx, `
		INSERT INTO ranking_results (id, incident_id, generated_at, suspects_json)
		VALUES (?, ?, ?, ?)
	`, "orphan-ranking", "missing-incident", time.Now().UTC(), `[]`)
	if rankingErr == nil {
		t.Fatal("insert orphan ranking error = nil, want foreign key violation")
	}
}

func openDiagnosisRepository(t *testing.T) (*sql.DB, *DiagnosisRepository) {
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
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return database, NewDiagnosisRepository(database)
}

func testDiagnosis(t *testing.T, id string) application.Diagnosis {
	t.Helper()

	incidentStart := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(
		incidentStart,
		incidentStart.Add(time.Minute),
		time.Minute,
	)
	if err != nil {
		t.Fatalf("create investigation window: %v", err)
	}
	findings := []detection.Finding{
		{
			Rule:        detection.RuleDatabaseTimeout,
			ServiceName: "checkout-service",
			Score:       0.3,
			Confidence:  detection.ConfidenceHigh,
			Evidence: []detection.Evidence{{
				Summary:       "6 de 20 operações PostgreSQL tiveram timeout.",
				SignalIDs:     []string{"signal-db-2", "signal-db-1"},
				BaselineValue: 0,
				IncidentValue: 0.3,
			}},
			Limitations: []string{"Correlação não comprova causalidade."},
		},
		{
			Rule:        detection.RuleErrorRateDelta,
			ServiceName: "checkout-service",
			Score:       0.4,
			Confidence:  detection.ConfidenceHigh,
			Evidence: []detection.Evidence{{
				Summary:       "Taxa de erro aumentou de 0% para 40%.",
				SignalIDs:     []string{"signal-http-1"},
				BaselineValue: 0,
				IncidentValue: 0.4,
			}},
		},
	}
	return application.Diagnosis{
		ID:                  id,
		ServiceName:         "checkout-service",
		Windows:             windows,
		BaselineSignalCount: 40,
		IncidentSignalCount: 40,
		Findings:            findings,
		Suspects: []ranking.Suspect{{
			ID:         "checkout-service",
			Label:      "checkout-service",
			Score:      0.4,
			Confidence: detection.ConfidenceHigh,
			Contributions: []ranking.ScoreContribution{
				{RuleID: detection.RuleDatabaseTimeout, Value: 0.06, Reason: "score 0.30 × peso 0.20 = 0.06"},
				{RuleID: detection.RuleErrorRateDelta, Value: 0.10, Reason: "score 0.40 × peso 0.25 = 0.10"},
			},
			Limitations: []string{"Correlação não comprova causalidade."},
		}},
	}
}

func assertStoredIncident(t *testing.T, database *sql.DB, diagnosis application.Diagnosis) {
	t.Helper()

	var serviceName, status string
	var startedAt, endedAt time.Time
	err := database.QueryRowContext(context.Background(), `
		SELECT service_name, started_at, ended_at, status
		FROM incidents
		WHERE id = ?
	`, diagnosis.ID).Scan(&serviceName, &startedAt, &endedAt, &status)
	if err != nil {
		t.Fatalf("read incident: %v", err)
	}
	if serviceName != diagnosis.ServiceName || status != "diagnosed" {
		t.Fatalf("stored incident = (%q, %q), want (%q, diagnosed)", serviceName, status, diagnosis.ServiceName)
	}
	if !startedAt.Equal(diagnosis.Windows.Incident.Start) || !endedAt.Equal(diagnosis.Windows.Incident.End) {
		t.Fatalf("stored incident window = [%s, %s], want [%s, %s]", startedAt, endedAt, diagnosis.Windows.Incident.Start, diagnosis.Windows.Incident.End)
	}
}

func assertStoredFindings(t *testing.T, database *sql.DB, incidentID string, want []detection.Finding) {
	t.Helper()

	rows, err := database.QueryContext(context.Background(), `
		SELECT rule_id, subject_id, score, confidence, evidence_json, limitations_json
		FROM findings
		WHERE incident_id = ?
		ORDER BY rule_id, id
	`, incidentID)
	if err != nil {
		t.Fatalf("query findings: %v", err)
	}
	defer rows.Close()

	wantByRule := make(map[string]detection.Finding, len(want))
	for _, finding := range want {
		wantByRule[finding.Rule] = finding
	}
	found := 0
	for rows.Next() {
		var ruleID, subjectID, confidence, evidenceJSON, limitationsJSON string
		var score float64
		if err := rows.Scan(&ruleID, &subjectID, &score, &confidence, &evidenceJSON, &limitationsJSON); err != nil {
			t.Fatalf("scan finding: %v", err)
		}
		expected, ok := wantByRule[ruleID]
		if !ok {
			t.Fatalf("unexpected stored rule %q", ruleID)
		}
		var evidence []detection.Evidence
		var limitations []string
		if err := json.Unmarshal([]byte(evidenceJSON), &evidence); err != nil {
			t.Fatalf("decode evidence for %s: %v", ruleID, err)
		}
		if err := json.Unmarshal([]byte(limitationsJSON), &limitations); err != nil {
			t.Fatalf("decode limitations for %s: %v", ruleID, err)
		}
		if subjectID != expected.ServiceName || score != expected.Score || confidence != string(expected.Confidence) {
			t.Fatalf("stored finding %s scalar fields differ from %#v", ruleID, expected)
		}
		if !reflect.DeepEqual(evidence, expected.Evidence) || !reflect.DeepEqual(limitations, expected.Limitations) {
			t.Fatalf("stored finding %s JSON = (%#v, %#v), want (%#v, %#v)", ruleID, evidence, limitations, expected.Evidence, expected.Limitations)
		}
		found++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate findings: %v", err)
	}
	if found != len(want) {
		t.Fatalf("stored findings = %d, want %d", found, len(want))
	}
}

func assertStoredRanking(t *testing.T, database *sql.DB, incidentID string, want []ranking.Suspect) {
	t.Helper()

	var id, suspectsJSON string
	if err := database.QueryRowContext(context.Background(), `
		SELECT id, suspects_json
		FROM ranking_results
		WHERE incident_id = ?
	`, incidentID).Scan(&id, &suspectsJSON); err != nil {
		t.Fatalf("read ranking: %v", err)
	}
	if id != "ranking:"+incidentID {
		t.Fatalf("ranking id = %q, want %q", id, "ranking:"+incidentID)
	}
	var suspects []ranking.Suspect
	if err := json.Unmarshal([]byte(suspectsJSON), &suspects); err != nil {
		t.Fatalf("decode suspects: %v", err)
	}
	if !reflect.DeepEqual(suspects, want) {
		t.Fatalf("stored suspects = %#v, want %#v", suspects, want)
	}
}

func storedIDs(t *testing.T, database *sql.DB, table, filterColumn, incidentID string) []string {
	t.Helper()

	// Os nomes são constantes definidos pelos próprios testes, nunca derivados de entrada externa.
	query := "SELECT id FROM " + table + " WHERE " + filterColumn + " = ? ORDER BY id"
	rows, err := database.QueryContext(context.Background(), query, incidentID)
	if err != nil {
		t.Fatalf("query IDs from %s: %v", table, err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan ID from %s: %v", table, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate IDs from %s: %v", table, err)
	}
	sort.Strings(ids)
	return ids
}

func storedFindingIDsByRule(t *testing.T, database *sql.DB, incidentID string) map[string]string {
	t.Helper()

	rows, err := database.QueryContext(context.Background(), `
		SELECT rule_id, id
		FROM findings
		WHERE incident_id = ?
		ORDER BY rule_id, id
	`, incidentID)
	if err != nil {
		t.Fatalf("query finding IDs: %v", err)
	}
	defer rows.Close()
	ids := make(map[string]string)
	for rows.Next() {
		var ruleID, id string
		if err := rows.Scan(&ruleID, &id); err != nil {
			t.Fatalf("scan finding ID: %v", err)
		}
		ids[ruleID] = id
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate finding IDs: %v", err)
	}
	return ids
}

func rowCount(t *testing.T, database *sql.DB, table string) int {
	t.Helper()

	// O nome vem de uma allowlist fixa no teste e não contém entrada do usuário.
	var count int
	if err := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}
