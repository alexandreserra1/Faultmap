package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

const emptyJSONObject = "{}"

// SignalRepository persiste e consulta sinais usando o pool SQLite compartilhado
// pelo processo. Ele não cria conexões nem mantém estado de requisições.
type SignalRepository struct {
	database *sql.DB
}

// NewSignalRepository cria o repositório ligado ao pool SQLite criado durante o bootstrap.
func NewSignalRepository(database *sql.DB) *SignalRepository {
	return &SignalRepository{database: database}
}

// Save grava sinais de forma atômica e idempotente pelo ID. Em caso de retry, um
// ID existente é ignorado e não altera a versão de telemetria originalmente salva.
func (repository *SignalRepository) Save(ctx context.Context, signals []domain.Signal) (inserted int, err error) {
	if len(signals) == 0 {
		return 0, nil
	}

	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin save signals transaction: %w", err)
	}

	for _, signal := range signals {
		attributesJSON, err := marshalStringMap(signal.Attributes)
		if err != nil {
			return 0, rollbackSignalTransaction(transaction, fmt.Errorf("marshal signal %q attributes: %w", signal.ID, err))
		}
		measurementsJSON, err := marshalFloatMap(signal.Measurements)
		if err != nil {
			return 0, rollbackSignalTransaction(transaction, fmt.Errorf("marshal signal %q measurements: %w", signal.ID, err))
		}

		result, err := transaction.ExecContext(ctx, `
			INSERT INTO signals (
				id, signal_type, service_name, timestamp, trace_id, span_id, severity,
				attributes_json, measurements_json
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO NOTHING
		`,
			signal.ID,
			string(signal.Type),
			signal.ServiceName,
			signal.Timestamp.UTC(),
			signal.TraceID,
			signal.SpanID,
			signal.Severity,
			attributesJSON,
			measurementsJSON,
		)
		if err != nil {
			return 0, rollbackSignalTransaction(transaction, fmt.Errorf("insert signal %q: %w", signal.ID, err))
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return 0, rollbackSignalTransaction(transaction, fmt.Errorf("count inserted signal %q: %w", signal.ID, err))
		}
		inserted += int(rowsAffected)
	}

	if err := transaction.Commit(); err != nil {
		return 0, fmt.Errorf("commit save signals transaction: %w", err)
	}
	return inserted, nil
}

// ListByServiceAndWindow devolve no máximo limit sinais de um serviço no
// intervalo [start, end), ordenados de forma estável por timestamp e ID.
func (repository *SignalRepository) ListByServiceAndWindow(
	ctx context.Context,
	serviceName string,
	start time.Time,
	end time.Time,
	limit int,
) ([]domain.Signal, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("signal list limit must be greater than zero")
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("signal window start must be before end")
	}

	rows, err := repository.database.QueryContext(ctx, `
		SELECT
			id, signal_type, service_name, timestamp, trace_id, span_id, severity,
			attributes_json, measurements_json
		FROM signals
		WHERE service_name = ? AND timestamp >= ? AND timestamp < ?
		ORDER BY timestamp ASC, id ASC
		LIMIT ?
	`, serviceName, start.UTC(), end.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("list signals for service %q: %w", serviceName, err)
	}
	defer rows.Close()

	signals := make([]domain.Signal, 0)
	for rows.Next() {
		signal, err := scanSignal(rows)
		if err != nil {
			return nil, err
		}
		signals = append(signals, signal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate signals for service %q: %w", serviceName, err)
	}
	return signals, nil
}

// ListByTraceID devolve no máximo limit sinais do trace exato, ordenados por
// timestamp e ID. A consulta usa o pool compartilhado e não carrega outros traces.
func (repository *SignalRepository) ListByTraceID(ctx context.Context, traceID string, limit int) ([]domain.Signal, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return nil, fmt.Errorf("signal trace ID is required")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("signal trace list limit must be greater than zero")
	}

	rows, err := repository.database.QueryContext(ctx, `
		SELECT
			id, signal_type, service_name, timestamp, trace_id, span_id, severity,
			attributes_json, measurements_json
		FROM signals
		WHERE trace_id = ?
		ORDER BY timestamp ASC, id ASC
		LIMIT ?
	`, traceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list signals for trace %q: %w", traceID, err)
	}
	defer rows.Close()

	signals := make([]domain.Signal, 0)
	for rows.Next() {
		signal, err := scanSignal(rows)
		if err != nil {
			return nil, err
		}
		signals = append(signals, signal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate signals for trace %q: %w", traceID, err)
	}
	return signals, nil
}

// scanSignal converte uma linha de persistência para o tipo de domínio, sem
// permitir que detalhes de SQLite vazem para as camadas superiores.
func scanSignal(rows *sql.Rows) (domain.Signal, error) {
	var signal domain.Signal
	var signalType string
	var attributesJSON string
	var measurementsJSON string
	if err := rows.Scan(
		&signal.ID,
		&signalType,
		&signal.ServiceName,
		&signal.Timestamp,
		&signal.TraceID,
		&signal.SpanID,
		&signal.Severity,
		&attributesJSON,
		&measurementsJSON,
	); err != nil {
		return domain.Signal{}, fmt.Errorf("scan signal: %w", err)
	}
	signal.Type = domain.SignalType(signalType)
	if err := json.Unmarshal([]byte(attributesJSON), &signal.Attributes); err != nil {
		return domain.Signal{}, fmt.Errorf("unmarshal signal %q attributes: %w", signal.ID, err)
	}
	if err := json.Unmarshal([]byte(measurementsJSON), &signal.Measurements); err != nil {
		return domain.Signal{}, fmt.Errorf("unmarshal signal %q measurements: %w", signal.ID, err)
	}
	return signal, nil
}

// marshalStringMap mantém um objeto JSON mesmo quando a fonte não trouxe atributos.
func marshalStringMap(values map[string]string) (string, error) {
	if values == nil {
		return emptyJSONObject, nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// marshalFloatMap mantém um objeto JSON mesmo quando a fonte não trouxe medições.
func marshalFloatMap(values map[string]float64) (string, error) {
	if values == nil {
		return emptyJSONObject, nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// rollbackSignalTransaction encerra a transação curta ao primeiro erro e mantém
// a causa original disponível para quem iniciou a persistência.
func rollbackSignalTransaction(transaction *sql.Tx, cause error) error {
	if rollbackErr := transaction.Rollback(); rollbackErr != nil {
		return fmt.Errorf("save signals failed: %w; rollback save signals transaction: %v", cause, rollbackErr)
	}
	return fmt.Errorf("save signals failed: %w", cause)
}
