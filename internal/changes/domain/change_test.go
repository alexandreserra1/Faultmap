package domain

import (
	"testing"
	"time"
)

// TestCommitValidateRejectsIncompleteChange garante que dados externos não
// cheguem à persistência sem identidade, repositório e instante confiáveis.
func TestCommitValidateRejectsIncompleteChange(t *testing.T) {
	t.Parallel()

	valid := Commit{SHA: "abc123", Repository: "acme/checkout", CommittedAt: time.Now().UTC()}
	for _, change := range []Commit{
		{},
		{Repository: valid.Repository, CommittedAt: valid.CommittedAt},
		{SHA: valid.SHA, CommittedAt: valid.CommittedAt},
		{SHA: valid.SHA, Repository: valid.Repository},
	} {
		if err := change.Validate(); err == nil {
			t.Fatalf("Commit.Validate() erro = nil para %#v", change)
		}
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Commit.Validate() erro = %v", err)
	}
}

// TestDeploymentValidateRequiresCorrelationFields protege a relação explícita
// deployment → commit → serviço usada pelo detector.
func TestDeploymentValidateRequiresCorrelationFields(t *testing.T) {
	t.Parallel()

	valid := Deployment{
		ID: "github:acme/checkout:deployment:42", Repository: "acme/checkout",
		Environment: "staging", ServiceName: "checkout-service", CommitSHA: "abc123",
		DeployedAt: time.Now().UTC(),
	}
	for _, change := range []Deployment{
		{},
		{Repository: valid.Repository, Environment: valid.Environment, ServiceName: valid.ServiceName, CommitSHA: valid.CommitSHA, DeployedAt: valid.DeployedAt},
		{ID: valid.ID, Environment: valid.Environment, ServiceName: valid.ServiceName, CommitSHA: valid.CommitSHA, DeployedAt: valid.DeployedAt},
		{ID: valid.ID, Repository: valid.Repository, ServiceName: valid.ServiceName, CommitSHA: valid.CommitSHA, DeployedAt: valid.DeployedAt},
		{ID: valid.ID, Repository: valid.Repository, Environment: valid.Environment, CommitSHA: valid.CommitSHA, DeployedAt: valid.DeployedAt},
		{ID: valid.ID, Repository: valid.Repository, Environment: valid.Environment, ServiceName: valid.ServiceName, DeployedAt: valid.DeployedAt},
	} {
		if err := change.Validate(); err == nil {
			t.Fatalf("Deployment.Validate() erro = nil para %#v", change)
		}
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Deployment.Validate() erro = %v", err)
	}
}
