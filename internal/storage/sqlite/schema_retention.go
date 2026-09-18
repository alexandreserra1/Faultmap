package sqlite

import (
	"context"
	"fmt"
	"time"
)

// PruneSchemaCatalogsBefore libera o conteúdo dos catálogos coletados antes de
// cutoff, preservando a linha da coleta e todas as mudanças derivadas dela.
//
// O que cresce é o JSON do catálogo: numa base com 5.000 objetos ele passa de
// 600 KB por coleta, e a pontuação pessimista da proximidade cobra coleta
// frequente — de 5 em 5 minutos isso são dezenas de gigabytes por ano, num
// produto que se distribui como SQLite local. As mudanças derivadas são
// minúsculas e sustentam evidência de diagnósticos já gravados: apagá-las
// tiraria do relatório aquilo que ele afirma.
//
// Por isso a limpeza esvazia `objects_json` em vez de remover a linha. A chave
// estrangeira de `schema_changes` aponta para a coleta com `ON DELETE CASCADE`,
// então remover a linha levaria a evidência junto — a armadilha ficou comentada
// na migration que a criou. Esvaziar recupera praticamente todo o espaço e
// mantém o registro de que a coleta aconteceu.
//
// A proteção casa a linha exata pelo id, e não o instante pelo MAX: duas
// coletas da mesma base com captured_at idêntico — dois coletores, ou um
// carimbo truncado ao segundo — comparariam ambas iguais ao máximo e ficariam
// protegidas para sempre, quebrando a invariante de "exatamente uma íntegra".
//
// A coleta mais recente de cada base nunca é esvaziada, qualquer que seja a
// idade dela. Ela é a linha de base da próxima comparação: esvaziá-la faria o
// diff seguinte enxergar um catálogo vazio e reportar todo objeto da base como
// recém-criado — uma migração inventada em cada tabela, no próximo incidente.
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

	// A subconsulta ordenada torna o lote determinístico e começa pelas coletas
	// mais antigas. `captured_at <> (máximo da base)` é o que protege a linha de
	// base; `objects_json <> ''` é o que torna a operação idempotente, porque um
	// catálogo já esvaziado não entra no lote seguinte.
	result, err := transaction.ExecContext(ctx, `
		UPDATE schema_snapshots
		SET objects_json = ''
		WHERE id IN (
			SELECT s.id FROM schema_snapshots s
			WHERE s.captured_at < ?
			  AND s.objects_json <> ''
			  AND s.id <> (
				SELECT u.id FROM schema_snapshots u
				WHERE u.database_name = s.database_name
				ORDER BY u.captured_at DESC, u.id DESC
				LIMIT 1
			  )
			ORDER BY s.captured_at ASC, s.id ASC
			LIMIT ?
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
// Existe para que a limpeza possa ser verificada pelo que ela promete não
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
		`SELECT COUNT(*) FROM schema_snapshots WHERE database_name = ? AND objects_json <> ''`,
		databaseName).Scan(&total); err != nil {
		return 0, fmt.Errorf("count intact catalogs: %w", err)
	}
	return total, nil
}
