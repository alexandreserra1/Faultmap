package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// RetentionRepository aplica a política de retenção sobre a telemetria bruta
// usando o pool SQLite compartilhado pelo processo.
//
// A limpeza alcança somente a tabela signals. Snapshots de diagnóstico
// (incidents, findings e ranking_results) são deliberadamente preservados: eles
// são o registro auditável de uma investigação já publicada e precisam
// continuar legíveis por incident show mesmo depois que os sinais que os
// originaram expiram. Os IDs de sinais mantidos como proveniência podem, a
// partir daí, apontar para telemetria ausente.
type RetentionRepository struct {
	database *sql.DB
}

// NewRetentionRepository liga o repositório ao pool criado durante o bootstrap.
func NewRetentionRepository(database *sql.DB) *RetentionRepository {
	return &RetentionRepository{database: database}
}

// DeleteSignalsBefore remove, em uma única transação curta, no máximo limit
// sinais anteriores a cutoff, começando pelos mais antigos. O limite mantém a
// transação previsível e permite que o caso de uso avance por lotes em vez de
// bloquear o único escritor do SQLite com um DELETE ilimitado.
//
// A operação é idempotente por natureza: repetir a chamada apenas remove o
// lote seguinte e, quando não há mais sinais expirados, devolve zero.
func (repository *RetentionRepository) DeleteSignalsBefore(
	ctx context.Context,
	cutoff time.Time,
	limit int,
) (int, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("retention delete limit must be greater than zero")
	}

	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin retention transaction: %w", err)
	}

	// A subconsulta ordenada torna o lote determinístico e garante que a
	// telemetria mais antiga seja sempre a primeira a sair do banco.
	result, err := transaction.ExecContext(ctx, `
		DELETE FROM signals
		WHERE id IN (
			SELECT id FROM signals
			WHERE timestamp < ?
			ORDER BY timestamp ASC, id ASC
			LIMIT ?
		)
	`, cutoff.UTC(), limit)
	if err != nil {
		return 0, rollbackRetentionTransaction(transaction, fmt.Errorf("delete expired signals: %w", err))
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, rollbackRetentionTransaction(transaction, fmt.Errorf("count expired signals: %w", err))
	}
	if err := transaction.Commit(); err != nil {
		return 0, fmt.Errorf("commit retention transaction: %w", err)
	}
	return int(removed), nil
}

// rollbackRetentionTransaction encerra o lote ao primeiro erro preservando a causa.
func rollbackRetentionTransaction(transaction *sql.Tx, cause error) error {
	if rollbackErr := transaction.Rollback(); rollbackErr != nil {
		return fmt.Errorf("apply retention failed: %w; rollback retention transaction: %v", cause, rollbackErr)
	}
	return fmt.Errorf("apply retention failed: %w", cause)
}
