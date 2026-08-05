package application

import (
	"context"
	"fmt"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
)

// ChangeImportResult diferencia itens coletados de itens realmente inseridos em uma execução idempotente.
type ChangeImportResult struct {
	CommitsFetched       int
	CommitsPersisted     int
	DeploymentsFetched   int
	DeploymentsPersisted int
}

// ChangeSource abstrai uma coleta externa limitada sem expor payloads do provedor.
type ChangeSource interface {
	Fetch(ctx context.Context, request changedomain.ImportRequest) (changedomain.Snapshot, error)
}

// ChangeWriter persiste o snapshot como uma unidade curta e idempotente.
type ChangeWriter interface {
	SaveChanges(ctx context.Context, snapshot changedomain.Snapshot) (ChangeImportResult, error)
}

// IngestChanges coleta primeiro e só então inicia a persistência, evitando rede dentro da transação.
func IngestChanges(
	ctx context.Context,
	request changedomain.ImportRequest,
	source ChangeSource,
	writer ChangeWriter,
) (ChangeImportResult, error) {
	snapshot, err := source.Fetch(ctx, request)
	if err != nil {
		return ChangeImportResult{}, fmt.Errorf("ingerir mudanças: coletar origem: %w", err)
	}
	result, err := writer.SaveChanges(ctx, snapshot)
	if err != nil {
		return ChangeImportResult{}, fmt.Errorf("ingerir mudanças: persistir snapshot: %w", err)
	}
	result.CommitsFetched = len(snapshot.Commits)
	result.DeploymentsFetched = len(snapshot.Deployments)
	return result, nil
}
