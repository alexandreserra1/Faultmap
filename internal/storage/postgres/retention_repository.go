package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// RetentionRepository aplica a política de retenção sobre a telemetria bruta.
//
// DeleteSignalsBefore alcança somente a tabela signals. Snapshots de
// diagnóstico (incidents, findings e ranking_results) são deliberadamente
// preservados: eles são o registro auditável de uma investigação já publicada e
// precisam continuar legíveis por incident show mesmo depois que os sinais que
// os originaram expiram (ADR 0003). Os IDs de sinais mantidos como proveniência
// podem, a partir daí, apontar para telemetria ausente.
//
// O catálogo de schema tem política própria, na ADR 0015, aplicada por
// PruneSchemaCatalogsBefore.
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
// segurar um DELETE ilimitado.
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
	// telemetria mais antiga seja sempre a primeira a sair do banco. A consulta
	// é a mesma do SQLite, sem SKIP LOCKED: `retention apply` é um comando
	// explícito de operação, não uma rotina concorrente, e fazer os dois
	// backends escolherem lotes por critérios diferentes tiraria o sentido de
	// compará-los.
	result, err := transaction.ExecContext(ctx, `
		DELETE FROM signals
		WHERE id IN (
			SELECT id FROM signals
			WHERE timestamp < $1
			ORDER BY timestamp ASC, id ASC
			LIMIT $2
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

// PruneSchemaCatalogsBefore libera o conteúdo dos catálogos coletados antes de
// cutoff, preservando a linha da coleta e todas as mudanças derivadas dela.
//
// É a mesma política do backend SQLite, e a bateria de conformidade prova que as
// duas se comportam igual. A consulta é idêntica em intenção; a única diferença
// é o dialeto, com placeholders numerados.
//
// A limpeza esvazia objects_json em vez de remover a linha porque a chave
// estrangeira de schema_changes aponta para a coleta com ON DELETE CASCADE:
// remover a linha levaria junto a evidência que diagnósticos já gravados citam.
//
// A coleta mais recente de cada base nunca é esvaziada, qualquer que seja a
// idade. Ela é a linha de base da próxima comparação — esvaziá-la faria o diff
// seguinte enxergar catálogo vazio e reportar todo objeto da base como
// recém-criado. Ver ADR 0015.
func (repository *RetentionRepository) PruneSchemaCatalogsBefore(
	ctx context.Context,
	cutoff time.Time,
	limit int,
) (int, error) {
	if limit <= 0 {
		return 0, fmt.Errorf("schema catalog prune limit must be greater than zero")
	}

	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin schema retention transaction: %w", err)
	}

	result, err := transaction.ExecContext(ctx, `
		UPDATE schema_snapshots
		SET objects_json = ''
		WHERE id IN (
			SELECT s.id FROM schema_snapshots s
			WHERE s.captured_at < $1
			  AND s.objects_json <> ''
			  AND s.captured_at <> (
				SELECT MAX(u.captured_at) FROM schema_snapshots u
				WHERE u.database_name = s.database_name
			  )
			ORDER BY s.captured_at ASC, s.id ASC
			LIMIT $2
		)
	`, cutoff.UTC(), limit)
	if err != nil {
		return 0, rollbackRetentionTransaction(transaction, fmt.Errorf("prune schema catalogs: %w", err))
	}
	pruned, err := result.RowsAffected()
	if err != nil {
		return 0, rollbackRetentionTransaction(transaction, fmt.Errorf("count pruned schema catalogs: %w", err))
	}
	if err := transaction.Commit(); err != nil {
		return 0, fmt.Errorf("commit schema retention transaction: %w", err)
	}
	return int(pruned), nil
}

// CountSchemaChanges devolve quantas mudanças de catálogo estão gravadas.
//
// Existe para que a liberação possa ser verificada pelo que ela promete não
// tocar: o número precisa ser idêntico antes e depois.
func (repository *RetentionRepository) CountSchemaChanges(ctx context.Context) (int, error) {
	var total int
	if err := repository.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_changes`).Scan(&total); err != nil {
		return 0, fmt.Errorf("count schema changes: %w", err)
	}
	return total, nil
}

// CountIntactCatalogs devolve quantas coletas de uma base ainda têm conteúdo.
func (repository *RetentionRepository) CountIntactCatalogs(ctx context.Context, databaseName string) (int, error) {
	var total int
	if err := repository.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_snapshots WHERE database_name = $1 AND objects_json <> ''`,
		databaseName).Scan(&total); err != nil {
		return 0, fmt.Errorf("count intact catalogs: %w", err)
	}
	return total, nil
}
