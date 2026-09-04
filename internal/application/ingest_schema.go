package application

import (
	"context"
	"fmt"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
)

// SchemaImportResult separa o que foi lido do catálogo do que virou mudança
// registrada. A distinção importa: uma coleta com milhares de objetos e zero
// mudanças é o resultado saudável, e não uma coleta que falhou.
type SchemaImportResult struct {
	ObjectsCollected int
	ChangesPersisted int
}

// SchemaSource abstrai a leitura do catálogo sem expor tipos do driver.
type SchemaSource interface {
	Fetch(ctx context.Context) (changedomain.SchemaSnapshot, error)
}

// SchemaWriter persiste a coleta e as diferenças em relação à anterior.
type SchemaWriter interface {
	SaveSnapshot(ctx context.Context, snapshot changedomain.SchemaSnapshot) (SchemaImportResult, error)
}

// IngestSchema lê o catálogo e só então inicia a persistência.
//
// A ordem é a mesma de IngestChanges e pelo mesmo motivo: manter a rede fora da
// transação. Uma consulta lenta ao PostgreSQL não pode segurar uma transação
// aberta no SQLite local.
func IngestSchema(
	ctx context.Context,
	source SchemaSource,
	writer SchemaWriter,
) (SchemaImportResult, error) {
	snapshot, err := source.Fetch(ctx)
	if err != nil {
		return SchemaImportResult{}, fmt.Errorf("ingerir catálogo: coletar origem: %w", err)
	}
	result, err := writer.SaveSnapshot(ctx, snapshot)
	if err != nil {
		return SchemaImportResult{}, fmt.Errorf("ingerir catálogo: persistir coleta: %w", err)
	}
	result.ObjectsCollected = len(snapshot.Objects)
	return result, nil
}
