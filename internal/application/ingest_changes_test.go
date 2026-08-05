package application

import (
	"context"
	"errors"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
)

// TestIngestChangesFetchesAndPersistsOneBoundedSnapshot garante que rede e
// banco permaneçam separados e que retries possam informar somente itens novos.
func TestIngestChangesFetchesAndPersistsOneBoundedSnapshot(t *testing.T) {
	t.Parallel()

	request := validChangesRequest()
	snapshot := changedomain.Snapshot{
		Commits: []changedomain.Commit{{SHA: "abc", Repository: request.Repository, CommittedAt: request.Since}},
		Deployments: []changedomain.Deployment{{
			ID: "dep", Repository: request.Repository, Environment: request.Environment,
			ServiceName: request.ServiceName, CommitSHA: "abc", DeployedAt: request.Since,
		}},
	}
	source := &changeSourceFake{snapshot: snapshot}
	writer := &changeWriterFake{result: ChangeImportResult{CommitsPersisted: 1, DeploymentsPersisted: 1}}

	result, err := IngestChanges(context.Background(), request, source, writer)
	if err != nil {
		t.Fatalf("IngestChanges() erro = %v", err)
	}
	if result.CommitsFetched != 1 || result.DeploymentsFetched != 1 || result.CommitsPersisted != 1 || result.DeploymentsPersisted != 1 {
		t.Fatalf("resultado = %#v", result)
	}
	if source.calls != 1 || writer.calls != 1 {
		t.Fatalf("chamadas = source %d, writer %d", source.calls, writer.calls)
	}
}

// TestIngestChangesStopsBeforePersistenceWhenSourceFails preserva a causa e
// não inicia escrita após falha externa.
func TestIngestChangesStopsBeforePersistenceWhenSourceFails(t *testing.T) {
	t.Parallel()

	sourceErr := errors.New("GitHub indisponível")
	source := &changeSourceFake{err: sourceErr}
	writer := &changeWriterFake{}
	_, err := IngestChanges(context.Background(), validChangesRequest(), source, writer)
	if !errors.Is(err, sourceErr) {
		t.Fatalf("IngestChanges() erro = %v", err)
	}
	if writer.calls != 0 {
		t.Fatalf("writer.calls = %d, esperado 0", writer.calls)
	}
}

func validChangesRequest() changedomain.ImportRequest {
	until := time.Date(2025, time.December, 1, 10, 0, 0, 0, time.UTC)
	return changedomain.ImportRequest{
		Repository: "acme/checkout", Environment: "staging", ServiceName: "checkout-service",
		Since: until.Add(-time.Hour), Until: until, Limit: 20,
		IncludeCommits: true, IncludeDeployments: true,
	}
}

type changeSourceFake struct {
	snapshot changedomain.Snapshot
	err      error
	calls    int
}

func (source *changeSourceFake) Fetch(_ context.Context, _ changedomain.ImportRequest) (changedomain.Snapshot, error) {
	source.calls++
	return source.snapshot, source.err
}

type changeWriterFake struct {
	result ChangeImportResult
	err    error
	calls  int
}

func (writer *changeWriterFake) SaveChanges(_ context.Context, _ changedomain.Snapshot) (ChangeImportResult, error) {
	writer.calls++
	return writer.result, writer.err
}
