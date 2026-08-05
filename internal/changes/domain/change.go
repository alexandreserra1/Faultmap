// Package domain define mudanças de código e deployments sem depender da API GitHub ou do SQLite.
package domain

import (
	"fmt"
	"strings"
	"time"
)

// Commit representa a parte auditável de uma mudança de código importada.
type Commit struct {
	SHA         string
	Repository  string
	Author      string
	Message     string
	CommittedAt time.Time
	Files       []string
}

// Validate rejeita commits que não podem ser identificados ou ordenados com segurança.
func (commit Commit) Validate() error {
	if strings.TrimSpace(commit.SHA) == "" {
		return fmt.Errorf("validar commit: SHA é obrigatório")
	}
	if strings.TrimSpace(commit.Repository) == "" {
		return fmt.Errorf("validar commit %q: repositório é obrigatório", commit.SHA)
	}
	if commit.CommittedAt.IsZero() {
		return fmt.Errorf("validar commit %q: instante é obrigatório", commit.SHA)
	}
	return nil
}

// Deployment representa uma versão disponibilizada para um serviço e ambiente específicos.
type Deployment struct {
	ID          string
	Repository  string
	Environment string
	ServiceName string
	CommitSHA   string
	Ref         string
	Task        string
	State       string
	DeployedAt  time.Time
}

// ImportRequest delimita uma coleta de mudanças de uma origem externa.
type ImportRequest struct {
	Repository         string
	Environment        string
	ServiceName        string
	Since              time.Time
	Until              time.Time
	Limit              int
	IncludeCommits     bool
	IncludeDeployments bool
}

// Snapshot agrupa os recursos normalizados antes da persistência atômica.
type Snapshot struct {
	Commits     []Commit
	Deployments []Deployment
}

// Validate protege os campos necessários à correlação deployment → commit → serviço.
func (deployment Deployment) Validate() error {
	if strings.TrimSpace(deployment.ID) == "" {
		return fmt.Errorf("validar deployment: ID é obrigatório")
	}
	if strings.TrimSpace(deployment.Repository) == "" {
		return fmt.Errorf("validar deployment %q: repositório é obrigatório", deployment.ID)
	}
	if strings.TrimSpace(deployment.Environment) == "" {
		return fmt.Errorf("validar deployment %q: ambiente é obrigatório", deployment.ID)
	}
	if strings.TrimSpace(deployment.ServiceName) == "" {
		return fmt.Errorf("validar deployment %q: serviço é obrigatório", deployment.ID)
	}
	if strings.TrimSpace(deployment.CommitSHA) == "" {
		return fmt.Errorf("validar deployment %q: commit SHA é obrigatório", deployment.ID)
	}
	if deployment.DeployedAt.IsZero() {
		return fmt.Errorf("validar deployment %q: instante é obrigatório", deployment.ID)
	}
	return nil
}
