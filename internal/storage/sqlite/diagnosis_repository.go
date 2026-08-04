package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
)

// DiagnosisRepository persiste snapshots completos usando o pool SQLite compartilhado.
type DiagnosisRepository struct {
	database *sql.DB
}

// NewDiagnosisRepository liga o repositório ao pool criado durante o bootstrap.
func NewDiagnosisRepository(database *sql.DB) *DiagnosisRepository {
	return &DiagnosisRepository{database: database}
}

// Save grava incidente, findings e ranking em uma transação curta. O mesmo ID
// é imutável: retries não substituem nem duplicam o snapshot originalmente salvo.
func (repository *DiagnosisRepository) Save(ctx context.Context, diagnosis application.Diagnosis) (bool, error) {
	prepared, err := prepareDiagnosis(diagnosis)
	if err != nil {
		return false, err
	}
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin save diagnosis transaction: %w", err)
	}

	result, err := transaction.ExecContext(ctx, `
		INSERT INTO incidents (id, service_name, environment, started_at, ended_at, status)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`, diagnosis.ID, diagnosis.ServiceName, "", diagnosis.Windows.Incident.Start.UTC(), diagnosis.Windows.Incident.End.UTC(), "diagnosed")
	if err != nil {
		return false, rollbackDiagnosisTransaction(transaction, fmt.Errorf("insert incident %q: %w", diagnosis.ID, err))
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, rollbackDiagnosisTransaction(transaction, fmt.Errorf("count inserted incident %q: %w", diagnosis.ID, err))
	}
	if rowsAffected == 0 {
		if err := transaction.Rollback(); err != nil && err != sql.ErrTxDone {
			return false, fmt.Errorf("rollback idempotent diagnosis %q: %w", diagnosis.ID, err)
		}
		return false, nil
	}

	for _, finding := range prepared.findings {
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO findings (
				id, incident_id, rule_id, subject_id, score, confidence,
				evidence_json, limitations_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`,
			finding.id,
			diagnosis.ID,
			finding.rule,
			finding.subjectID,
			finding.score,
			finding.confidence,
			finding.evidenceJSON,
			finding.limitationsJSON,
		); err != nil {
			return false, rollbackDiagnosisTransaction(transaction, fmt.Errorf("insert finding %q: %w", finding.id, err))
		}
	}
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO ranking_results (id, incident_id, generated_at, suspects_json)
		VALUES (?, ?, ?, ?)
	`, "ranking:"+diagnosis.ID, diagnosis.ID, diagnosis.Windows.Incident.End.UTC(), prepared.suspectsJSON); err != nil {
		return false, rollbackDiagnosisTransaction(transaction, fmt.Errorf("insert ranking for incident %q: %w", diagnosis.ID, err))
	}

	if err := transaction.Commit(); err != nil {
		return false, fmt.Errorf("commit diagnosis %q: %w", diagnosis.ID, err)
	}
	return true, nil
}

type preparedDiagnosis struct {
	findings     []preparedFinding
	suspectsJSON string
}

type preparedFinding struct {
	id              string
	rule            string
	subjectID       string
	score           float64
	confidence      string
	evidenceJSON    string
	limitationsJSON string
}

// prepareDiagnosis serializa e ordena o snapshot antes de abrir a transação,
// reduzindo o tempo em que o escritor exclusivo do SQLite fica ocupado.
func prepareDiagnosis(diagnosis application.Diagnosis) (preparedDiagnosis, error) {
	if strings.TrimSpace(diagnosis.ID) == "" {
		return preparedDiagnosis{}, fmt.Errorf("prepare diagnosis: ID is required")
	}
	prepared := preparedDiagnosis{findings: make([]preparedFinding, 0, len(diagnosis.Findings))}
	for _, finding := range diagnosis.Findings {
		evidenceJSON, err := json.Marshal(finding.Evidence)
		if err != nil {
			return preparedDiagnosis{}, fmt.Errorf("marshal finding %q evidence: %w", finding.Rule, err)
		}
		limitationsJSON, err := json.Marshal(finding.Limitations)
		if err != nil {
			return preparedDiagnosis{}, fmt.Errorf("marshal finding %q limitations: %w", finding.Rule, err)
		}
		prepared.findings = append(prepared.findings, preparedFinding{
			id:              findingID(diagnosis.ID, finding),
			rule:            finding.Rule,
			subjectID:       finding.ServiceName,
			score:           finding.Score,
			confidence:      string(finding.Confidence),
			evidenceJSON:    string(evidenceJSON),
			limitationsJSON: string(limitationsJSON),
		})
	}
	sort.Slice(prepared.findings, func(first, second int) bool { return prepared.findings[first].id < prepared.findings[second].id })
	suspectsJSON, err := json.Marshal(diagnosis.Suspects)
	if err != nil {
		return preparedDiagnosis{}, fmt.Errorf("marshal diagnosis suspects: %w", err)
	}
	prepared.suspectsJSON = string(suspectsJSON)
	return prepared, nil
}

func findingID(incidentID string, finding detection.Finding) string {
	canonical := strings.Join([]string{incidentID, finding.Rule, finding.ServiceName}, "\x00")
	digest := sha256.Sum256([]byte(canonical))
	return "finding:" + hex.EncodeToString(digest[:12])
}

// rollbackDiagnosisTransaction preserva a causa obrigatória e relata uma falha adicional de rollback.
func rollbackDiagnosisTransaction(transaction *sql.Tx, cause error) error {
	if rollbackErr := transaction.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
		return fmt.Errorf("save diagnosis failed: %w; rollback diagnosis: %v", cause, rollbackErr)
	}
	return cause
}
