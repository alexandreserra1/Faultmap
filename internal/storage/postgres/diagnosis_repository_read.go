package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

const (
	maxFindingsPerIncident  = 1_000
	maxIncidentHistoryLimit = 1_000
)

// List devolve os diagnósticos mais recentes com limite e ordenação estável,
// sem carregar findings ou ranking que não serão usados no resumo.
func (repository *DiagnosisRepository) List(ctx context.Context, limit int) ([]application.IncidentSummary, error) {
	if limit <= 0 || limit > maxIncidentHistoryLimit {
		return nil, fmt.Errorf("incident list limit must be between 1 and %d", maxIncidentHistoryLimit)
	}
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, service_name, status, started_at, ended_at
		FROM incidents
		WHERE status = $1 AND ended_at IS NOT NULL
		ORDER BY started_at DESC, id ASC
		LIMIT $2
	`, "diagnosed", limit)
	if err != nil {
		return nil, fmt.Errorf("list persisted incidents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	incidents := make([]application.IncidentSummary, 0)
	for rows.Next() {
		var incident application.IncidentSummary
		if err := rows.Scan(
			&incident.ID, &incident.ServiceName, &incident.Status,
			&incident.IncidentStart, &incident.IncidentEnd,
		); err != nil {
			return nil, fmt.Errorf("scan persisted incident: %w", err)
		}
		incident.IncidentStart = incident.IncidentStart.UTC()
		incident.IncidentEnd = incident.IncidentEnd.UTC()
		incidents = append(incidents, incident)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate persisted incidents: %w", err)
	}
	return incidents, nil
}

// Get recupera um snapshot consistente em uma única transação de leitura curta.
//
// As três consultas ficam na mesma transação porque um snapshot lido pela
// metade não é um diagnóstico: o relatório mostraria findings de uma
// investigação e o ranking de outra.
func (repository *DiagnosisRepository) Get(ctx context.Context, id string) (application.PersistedDiagnosis, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return application.PersistedDiagnosis{}, fmt.Errorf("incident ID is required")
	}
	transaction, err := repository.database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return application.PersistedDiagnosis{}, fmt.Errorf("begin read incident transaction: %w", err)
	}

	diagnosis, err := readPersistedIncident(ctx, transaction, id)
	if err != nil {
		return application.PersistedDiagnosis{}, rollbackReadTransaction(transaction, err)
	}
	findings, err := readPersistedFindings(ctx, transaction, id)
	if err != nil {
		return application.PersistedDiagnosis{}, rollbackReadTransaction(transaction, err)
	}
	suspects, err := readPersistedRanking(ctx, transaction, id)
	if err != nil {
		return application.PersistedDiagnosis{}, rollbackReadTransaction(transaction, err)
	}
	diagnosis.Findings = findings
	diagnosis.Suspects = suspects
	if err := transaction.Commit(); err != nil {
		return application.PersistedDiagnosis{}, fmt.Errorf("commit read incident %q: %w", id, err)
	}
	return diagnosis, nil
}

func readPersistedIncident(ctx context.Context, transaction *sql.Tx, id string) (application.PersistedDiagnosis, error) {
	var diagnosis application.PersistedDiagnosis
	var baselineStart, baselineEnd sql.NullTime
	var baselineCount, incidentCount sql.NullInt64
	err := transaction.QueryRowContext(ctx, `
		SELECT
			id, service_name, environment, status, started_at, ended_at,
			baseline_start, baseline_end, baseline_signal_count, incident_signal_count
		FROM incidents
		WHERE id = $1 AND status = $2 AND ended_at IS NOT NULL
	`, id, "diagnosed").Scan(
		&diagnosis.Incident.ID,
		&diagnosis.Incident.ServiceName,
		&diagnosis.Incident.Environment,
		&diagnosis.Incident.Status,
		&diagnosis.Incident.IncidentStart,
		&diagnosis.Incident.IncidentEnd,
		&baselineStart,
		&baselineEnd,
		&baselineCount,
		&incidentCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return application.PersistedDiagnosis{}, application.ErrIncidentNotFound
	}
	if err != nil {
		return application.PersistedDiagnosis{}, fmt.Errorf("read persisted incident %q: %w", id, err)
	}
	diagnosis.Incident.IncidentStart = diagnosis.Incident.IncidentStart.UTC()
	diagnosis.Incident.IncidentEnd = diagnosis.Incident.IncidentEnd.UTC()
	diagnosis.BaselineStart = nullTimePointer(baselineStart)
	diagnosis.BaselineEnd = nullTimePointer(baselineEnd)
	diagnosis.BaselineSignalCount = nullIntPointer(baselineCount)
	diagnosis.IncidentSignalCount = nullIntPointer(incidentCount)
	return diagnosis, nil
}

func readPersistedFindings(ctx context.Context, transaction *sql.Tx, incidentID string) ([]detection.Finding, error) {
	rows, err := transaction.QueryContext(ctx, `
		SELECT rule_id, subject_id, score, confidence, evidence_json, limitations_json
		FROM findings
		WHERE incident_id = $1
		ORDER BY rule_id ASC, id ASC
		LIMIT $2
	`, incidentID, maxFindingsPerIncident+1)
	if err != nil {
		return nil, fmt.Errorf("read findings for incident %q: %w", incidentID, err)
	}
	defer func() { _ = rows.Close() }()

	findings := make([]detection.Finding, 0)
	for rows.Next() {
		if len(findings) == maxFindingsPerIncident {
			return nil, fmt.Errorf("incident %q exceeds maximum of %d findings", incidentID, maxFindingsPerIncident)
		}
		var finding detection.Finding
		var confidence, evidenceJSON, limitationsJSON string
		if err := rows.Scan(
			&finding.Rule,
			&finding.ServiceName,
			&finding.Score,
			&confidence,
			&evidenceJSON,
			&limitationsJSON,
		); err != nil {
			return nil, fmt.Errorf("scan finding for incident %q: %w", incidentID, err)
		}
		finding.Confidence = detection.Confidence(confidence)
		if err := json.Unmarshal([]byte(evidenceJSON), &finding.Evidence); err != nil {
			return nil, fmt.Errorf("decode finding %q evidence: %w", finding.Rule, err)
		}
		if err := json.Unmarshal([]byte(limitationsJSON), &finding.Limitations); err != nil {
			return nil, fmt.Errorf("decode finding %q limitations: %w", finding.Rule, err)
		}
		findings = append(findings, finding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate findings for incident %q: %w", incidentID, err)
	}
	return findings, nil
}

func readPersistedRanking(ctx context.Context, transaction *sql.Tx, incidentID string) ([]ranking.Suspect, error) {
	var suspectsJSON string
	err := transaction.QueryRowContext(ctx, `
		SELECT suspects_json
		FROM ranking_results
		WHERE incident_id = $1
	`, incidentID).Scan(&suspectsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return []ranking.Suspect{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ranking for incident %q: %w", incidentID, err)
	}
	var suspects []ranking.Suspect
	if err := json.Unmarshal([]byte(suspectsJSON), &suspects); err != nil {
		return nil, fmt.Errorf("decode ranking for incident %q: %w", incidentID, err)
	}
	return suspects, nil
}

func nullTimePointer(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time.UTC()
	return &result
}

func nullIntPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}

func rollbackReadTransaction(transaction *sql.Tx, cause error) error {
	if rollbackErr := transaction.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return fmt.Errorf("read incident failed: %w; rollback read transaction: %v", cause, rollbackErr)
	}
	return cause
}
