package postgres

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

// maxScopeServices limita quantos serviços uma investigação pode carregar de
// uma vez, protegendo a consulta contra excesso de parâmetros e a memória
// contra um escopo grande demais para ser útil. O teto é o mesmo do SQLite: um
// escopo aceito por um backend e recusado pelo outro seria uma divergência de
// produto disfarçada de detalhe de persistência.
const maxScopeServices = 50

// SignalRepository persiste e consulta sinais usando o pool PostgreSQL
// compartilhado pelo processo.
type SignalRepository struct {
	database *sql.DB
}

// NewSignalRepository cria o repositório ligado ao pool criado durante o bootstrap.
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
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO NOTHING
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
		WHERE service_name = $1 AND timestamp >= $2 AND timestamp < $3
		ORDER BY timestamp ASC, id ASC
		LIMIT $4
	`, serviceName, start.UTC(), end.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("list signals for service %q: %w", serviceName, err)
	}
	defer func() { _ = rows.Close() }()

	signals, err := scanSignals(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate signals for service %q: %w", serviceName, err)
	}
	return signals, nil
}

// ListByTraceID devolve no máximo limit sinais do trace exato, ordenados por
// timestamp e ID.
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
		WHERE trace_id = $1
		ORDER BY timestamp ASC, id ASC
		LIMIT $2
	`, traceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list signals for trace %q: %w", traceID, err)
	}
	defer func() { _ = rows.Close() }()

	signals, err := scanSignals(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate signals for trace %q: %w", traceID, err)
	}
	return signals, nil
}

// ListByServicesAndWindow carrega, em uma única consulta, os sinais de todos os
// serviços do escopo dentro da janela.
//
// A alternativa — uma consulta por serviço — seria um N+1 disfarçado, que
// cresceria junto com o raio do incidente exatamente quando ele é maior. O
// limite se aplica ao total de sinais e a ordenação é estável, para que duas
// execuções da mesma investigação leiam o mesmo conjunto.
func (repository *SignalRepository) ListByServicesAndWindow(
	ctx context.Context,
	serviceNames []string,
	start time.Time,
	end time.Time,
	limit int,
) ([]domain.Signal, error) {
	if len(serviceNames) == 0 {
		return nil, fmt.Errorf("signal scope must contain at least one service")
	}
	if len(serviceNames) > maxScopeServices {
		return nil, fmt.Errorf("signal scope must contain at most %d services", maxScopeServices)
	}
	if limit <= 0 {
		return nil, fmt.Errorf("signal list limit must be greater than zero")
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("signal window start must be before end")
	}

	// Os nomes entram como parâmetros numerados; nada é concatenado no SQL.
	arguments := make([]any, 0, len(serviceNames)+3)
	for _, serviceName := range serviceNames {
		arguments = append(arguments, serviceName)
	}
	proximo := len(serviceNames) + 1
	arguments = append(arguments, start.UTC(), end.UTC(), limit)

	query := fmt.Sprintf(`
		SELECT
			id, signal_type, service_name, timestamp, trace_id, span_id, severity,
			attributes_json, measurements_json
		FROM signals
		WHERE service_name IN (%s)
			AND timestamp >= $%d AND timestamp < $%d
		ORDER BY timestamp ASC, id ASC
		LIMIT $%d
	`, placeholders(len(serviceNames), 1), proximo, proximo+1, proximo+2)

	rows, err := repository.database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list signals for service scope: %w", err)
	}
	defer func() { _ = rows.Close() }()

	signals, err := scanSignals(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate signals for service scope: %w", err)
	}
	return signals, nil
}

// scanSignals converte as linhas para o domínio, sem permitir que detalhes do
// PostgreSQL vazem para as camadas superiores.
func scanSignals(rows *sql.Rows) ([]domain.Signal, error) {
	signals := make([]domain.Signal, 0)
	for rows.Next() {
		signal, err := scanSignal(rows)
		if err != nil {
			return nil, err
		}
		signals = append(signals, signal)
	}
	return signals, nil
}

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
	// O driver devolve TIMESTAMPTZ no fuso da sessão, que é o do servidor. Sem
	// esta conversão, o mesmo sinal voltaria com um horário diferente do que o
	// SQLite devolve, e a janela do incidente mudaria de tamanho conforme onde
	// o banco está hospedado.
	signal.Timestamp = signal.Timestamp.UTC()
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
