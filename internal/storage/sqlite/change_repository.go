package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
)

const (
	maxChangesPerImport    = 100
	maxDeploymentsPerQuery = 1_000
)

// ChangeRepository reutiliza o pool do processo para commits e deployments.
type ChangeRepository struct {
	database *sql.DB
}

// NewChangeRepository injeta o pool compartilhado sem abrir conexões próprias.
func NewChangeRepository(database *sql.DB) *ChangeRepository {
	return &ChangeRepository{database: database}
}

// SaveChanges grava commits e deployments em uma transação curta e idempotente.
func (repository *ChangeRepository) SaveChanges(ctx context.Context, snapshot changedomain.Snapshot) (application.ChangeImportResult, error) {
	prepared, err := prepareChanges(snapshot)
	if err != nil {
		return application.ChangeImportResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return application.ChangeImportResult{}, fmt.Errorf("salvar mudanças: contexto cancelado: %w", err)
	}
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return application.ChangeImportResult{}, fmt.Errorf("iniciar transação de mudanças: %w", err)
	}
	result := application.ChangeImportResult{}
	for _, commit := range prepared.commits {
		execution, err := transaction.ExecContext(ctx, `
			INSERT INTO commits (sha, repository, author, message, committed_at, files_json)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(sha) DO NOTHING
		`, commit.change.SHA, commit.change.Repository, commit.change.Author, commit.change.Message, commit.change.CommittedAt.UTC(), commit.filesJSON)
		if err != nil {
			return application.ChangeImportResult{}, rollbackChangeTransaction(transaction, fmt.Errorf("inserir commit %q: %w", commit.change.SHA, err))
		}
		rowsAffected, err := execution.RowsAffected()
		if err != nil {
			return application.ChangeImportResult{}, rollbackChangeTransaction(transaction, fmt.Errorf("contar commit %q inserido: %w", commit.change.SHA, err))
		}
		result.CommitsPersisted += int(rowsAffected)
	}
	for _, deployment := range prepared.deployments {
		execution, err := transaction.ExecContext(ctx, `
			INSERT INTO deployments (
				id, repository, environment, service_name, commit_sha, deployed_at, metadata_json
			)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO NOTHING
		`, deployment.change.ID, deployment.change.Repository, deployment.change.Environment,
			deployment.change.ServiceName, deployment.change.CommitSHA, deployment.change.DeployedAt.UTC(), deployment.metadataJSON)
		if err != nil {
			return application.ChangeImportResult{}, rollbackChangeTransaction(transaction, fmt.Errorf("inserir deployment %q: %w", deployment.change.ID, err))
		}
		rowsAffected, err := execution.RowsAffected()
		if err != nil {
			return application.ChangeImportResult{}, rollbackChangeTransaction(transaction, fmt.Errorf("contar deployment %q inserido: %w", deployment.change.ID, err))
		}
		result.DeploymentsPersisted += int(rowsAffected)
	}
	if err := transaction.Commit(); err != nil {
		return application.ChangeImportResult{}, fmt.Errorf("confirmar mudanças: %w", err)
	}
	return result, nil
}

// ListDeployments lê uma janela estável e limitada para o detector de proximidade.
func (repository *ChangeRepository) ListDeployments(
	ctx context.Context,
	serviceName string,
	environment string,
	start time.Time,
	end time.Time,
	limit int,
) ([]changedomain.Deployment, error) {
	serviceName = strings.TrimSpace(serviceName)
	environment = strings.TrimSpace(environment)
	if serviceName == "" || environment == "" {
		return nil, fmt.Errorf("listar deployments: serviço e ambiente são obrigatórios")
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("listar deployments: janela inválida")
	}
	if limit <= 0 || limit > maxDeploymentsPerQuery {
		return nil, fmt.Errorf("listar deployments: limite deve estar entre 1 e %d", maxDeploymentsPerQuery)
	}
	rows, err := repository.database.QueryContext(ctx, `
		SELECT id, repository, environment, service_name, commit_sha, deployed_at, metadata_json
		FROM deployments
		WHERE service_name = ? AND environment = ? AND deployed_at >= ? AND deployed_at < ?
		ORDER BY deployed_at DESC, id ASC
		LIMIT ?
	`, serviceName, environment, start.UTC(), end.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("listar deployments de %q: %w", serviceName, err)
	}
	defer rows.Close()
	deployments := make([]changedomain.Deployment, 0)
	for rows.Next() {
		var deployment changedomain.Deployment
		var metadataJSON string
		if err := rows.Scan(
			&deployment.ID, &deployment.Repository, &deployment.Environment,
			&deployment.ServiceName, &deployment.CommitSHA, &deployment.DeployedAt, &metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("ler deployment: %w", err)
		}
		var metadata deploymentMetadata
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			return nil, fmt.Errorf("interpretar metadata do deployment %q: %w", deployment.ID, err)
		}
		deployment.Ref = metadata.Ref
		deployment.Task = metadata.Task
		deployment.State = metadata.State
		deployment.DeployedAt = deployment.DeployedAt.UTC()
		deployments = append(deployments, deployment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("percorrer deployments de %q: %w", serviceName, err)
	}
	return deployments, nil
}

type preparedChanges struct {
	commits     []preparedCommit
	deployments []preparedDeployment
}

type preparedCommit struct {
	change    changedomain.Commit
	filesJSON string
}

type preparedDeployment struct {
	change       changedomain.Deployment
	metadataJSON string
}

type deploymentMetadata struct {
	Ref   string `json:"ref"`
	Task  string `json:"task"`
	State string `json:"state"`
}

func prepareChanges(snapshot changedomain.Snapshot) (preparedChanges, error) {
	if len(snapshot.Commits) > maxChangesPerImport || len(snapshot.Deployments) > maxChangesPerImport {
		return preparedChanges{}, fmt.Errorf("preparar mudanças: máximo de %d itens por recurso", maxChangesPerImport)
	}
	prepared := preparedChanges{
		commits:     make([]preparedCommit, 0, len(snapshot.Commits)),
		deployments: make([]preparedDeployment, 0, len(snapshot.Deployments)),
	}
	for _, commit := range snapshot.Commits {
		if err := commit.Validate(); err != nil {
			return preparedChanges{}, err
		}
		filesJSON, err := json.Marshal(commit.Files)
		if err != nil {
			return preparedChanges{}, fmt.Errorf("serializar arquivos do commit %q: %w", commit.SHA, err)
		}
		prepared.commits = append(prepared.commits, preparedCommit{change: commit, filesJSON: string(filesJSON)})
	}
	for _, deployment := range snapshot.Deployments {
		if err := deployment.Validate(); err != nil {
			return preparedChanges{}, err
		}
		metadataJSON, err := json.Marshal(deploymentMetadata{Ref: deployment.Ref, Task: deployment.Task, State: deployment.State})
		if err != nil {
			return preparedChanges{}, fmt.Errorf("serializar deployment %q: %w", deployment.ID, err)
		}
		prepared.deployments = append(prepared.deployments, preparedDeployment{change: deployment, metadataJSON: string(metadataJSON)})
	}
	return prepared, nil
}

func rollbackChangeTransaction(transaction *sql.Tx, cause error) error {
	if rollbackErr := transaction.Rollback(); rollbackErr != nil {
		return fmt.Errorf("salvar mudanças falhou: %w; rollback: %v", cause, rollbackErr)
	}
	return cause
}
