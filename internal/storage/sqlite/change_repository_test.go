package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
)

// TestChangeRepositorySaveChangesIsAtomicAndIdempotent garante que um retry
// não substitua nem duplique commits e deployments já importados.
func TestChangeRepositorySaveChangesIsAtomicAndIdempotent(t *testing.T) {
	t.Parallel()

	database, repository := openChangeRepository(t)
	snapshot := changeSnapshot()
	result, err := repository.SaveChanges(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("SaveChanges() erro = %v", err)
	}
	if result.CommitsPersisted != 1 || result.DeploymentsPersisted != 1 {
		t.Fatalf("resultado = %#v", result)
	}
	retry, err := repository.SaveChanges(context.Background(), snapshot)
	if err != nil {
		t.Fatalf("SaveChanges() retry erro = %v", err)
	}
	if retry.CommitsPersisted != 0 || retry.DeploymentsPersisted != 0 {
		t.Fatalf("resultado do retry = %#v", retry)
	}

	var metadata string
	if err := database.QueryRowContext(context.Background(), `SELECT metadata_json FROM deployments WHERE id = ?`, snapshot.Deployments[0].ID).Scan(&metadata); err != nil {
		t.Fatalf("ler metadata: %v", err)
	}
	if metadata != `{"ref":"main","task":"deploy","state":"unknown"}` {
		t.Fatalf("metadata = %s", metadata)
	}
}

// TestChangeRepositoryListDeploymentsFiltersOrdersAndLimits garante leitura
// projetada por serviço, ambiente e janela sem coleção ilimitada.
func TestChangeRepositoryListDeploymentsFiltersOrdersAndLimits(t *testing.T) {
	t.Parallel()

	_, repository := openChangeRepository(t)
	base := time.Date(2025, time.December, 1, 9, 0, 0, 0, time.UTC)
	snapshot := changedomain.Snapshot{Deployments: []changedomain.Deployment{
		deploymentAt("dep-b", "checkout-service", "staging", base.Add(time.Minute)),
		deploymentAt("dep-a", "checkout-service", "staging", base.Add(time.Minute)),
		deploymentAt("dep-new", "checkout-service", "staging", base.Add(2*time.Minute)),
		deploymentAt("dep-payment", "payment-service", "staging", base.Add(time.Minute)),
		deploymentAt("dep-prod", "checkout-service", "production", base.Add(time.Minute)),
	}}
	if _, err := repository.SaveChanges(context.Background(), snapshot); err != nil {
		t.Fatalf("SaveChanges() erro = %v", err)
	}
	got, err := repository.ListDeployments(context.Background(), "checkout-service", "staging", base, base.Add(3*time.Minute), 2)
	if err != nil {
		t.Fatalf("ListDeployments() erro = %v", err)
	}
	want := []changedomain.Deployment{snapshot.Deployments[2], snapshot.Deployments[1]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListDeployments() = %#v, esperado %#v", got, want)
	}
}

// TestChangeRepositoryValidatesBeforeDatabaseAndRespectsContext protege o pool
// compartilhado contra entradas inválidas e canceladas.
func TestChangeRepositoryValidatesBeforeDatabaseAndRespectsContext(t *testing.T) {
	t.Parallel()

	if _, err := NewChangeRepository(nil).SaveChanges(context.Background(), changedomain.Snapshot{Commits: []changedomain.Commit{{}}}); err == nil {
		t.Fatal("SaveChanges() erro = nil para commit inválido")
	}
	if _, err := NewChangeRepository(nil).ListDeployments(context.Background(), "", "staging", time.Now(), time.Now().Add(time.Hour), 1); err == nil {
		t.Fatal("ListDeployments() erro = nil para serviço vazio")
	}

	_, repository := openChangeRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repository.SaveChanges(ctx, changeSnapshot()); !errors.Is(err, context.Canceled) {
		t.Fatalf("SaveChanges() erro = %v, esperado context.Canceled", err)
	}
}

func openChangeRepository(t *testing.T) (*sql.DB, *ChangeRepository) {
	t.Helper()
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "faultmap.db"))
	if err != nil {
		t.Fatalf("Open() erro = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar banco: %v", closeErr)
		}
	})
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() erro = %v", err)
	}
	return database, NewChangeRepository(database)
}

func changeSnapshot() changedomain.Snapshot {
	deployedAt := time.Date(2025, time.December, 1, 9, 55, 0, 0, time.UTC)
	return changedomain.Snapshot{
		Commits: []changedomain.Commit{{
			SHA: "abc123", Repository: "acme/checkout", Author: "alex", Message: "Reduce pool",
			CommittedAt: deployedAt.Add(-time.Minute), Files: []string{},
		}},
		Deployments: []changedomain.Deployment{{
			ID: "github:acme/checkout:deployment:42", Repository: "acme/checkout",
			Environment: "staging", ServiceName: "checkout-service", CommitSHA: "abc123",
			Ref: "main", Task: "deploy", State: "unknown", DeployedAt: deployedAt,
		}},
	}
}

func deploymentAt(id, service, environment string, deployedAt time.Time) changedomain.Deployment {
	return changedomain.Deployment{
		ID: id, Repository: "acme/checkout", Environment: environment,
		ServiceName: service, CommitSHA: "abc123", Ref: "main", Task: "deploy",
		State: "unknown", DeployedAt: deployedAt,
	}
}
