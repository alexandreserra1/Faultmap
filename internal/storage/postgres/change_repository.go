package postgres

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
func (repository *ChangeRepository) SaveChanges(
	ctx context.Context,
	snapshot changedomain.Snapshot,
) (application.ChangeImportResult, error) {
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
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (sha) DO NOTHING
		`, commit.change.SHA, commit.change.Repository, commit.change.Author, commit.change.Message,
			commit.change.CommittedAt.UTC(), commit.filesJSON)
		if err != nil {
			return application.ChangeImportResult{}, rollbackChangeTransaction(
				transaction, fmt.Errorf("inserir commit %q: %w", commit.change.SHA, err))
		}
		rowsAffected, err := execution.RowsAffected()
		if err != nil {
			return application.ChangeImportResult{}, rollbackChangeTransaction(
				transaction, fmt.Errorf("contar commit %q inserido: %w", commit.change.SHA, err))
		}
		result.CommitsPersisted += int(rowsAffected)
	}
	for _, deployment := range prepared.deployments {
		execution, err := transaction.ExecContext(ctx, `
			INSERT INTO deployments (
				id, repository, environment, service_name, commit_sha, deployed_at, metadata_json
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO NOTHING
		`, deployment.change.ID, deployment.change.Repository, deployment.change.Environment,
			deployment.change.ServiceName, deployment.change.CommitSHA,
			deployment.change.DeployedAt.UTC(), deployment.metadataJSON)
		if err != nil {
			return application.ChangeImportResult{}, rollbackChangeTransaction(
				transaction, fmt.Errorf("inserir deployment %q: %w", deployment.change.ID, err))
		}
		rowsAffected, err := execution.RowsAffected()
		if err != nil {
			return application.ChangeImportResult{}, rollbackChangeTransaction(
				transaction, fmt.Errorf("contar deployment %q inserido: %w", deployment.change.ID, err))
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
		WHERE service_name = $1 AND environment = $2 AND deployed_at >= $3 AND deployed_at < $4
		ORDER BY deployed_at DESC, id ASC
		LIMIT $5
	`, serviceName, environment, start.UTC(), end.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("listar deployments de %q: %w", serviceName, err)
	}
	defer func() { _ = rows.Close() }()

	deployments, err := scanDeployments(rows, "ler deployment")
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("percorrer deployments de %q: %w", serviceName, err)
	}
	return deployments, nil
}

// ListDeploymentsForServices carrega os deployments de todos os serviços do
// escopo em uma única consulta.
//
// Com a investigação comparando vários serviços, consultar um por vez seria um
// N+1 que cresce junto com o raio do incidente. Os nomes entram como parâmetros
// numerados; nada é concatenado no SQL.
func (repository *ChangeRepository) ListDeploymentsForServices(
	ctx context.Context,
	serviceNames []string,
	environment string,
	start time.Time,
	end time.Time,
	limit int,
) ([]changedomain.Deployment, error) {
	environment = strings.TrimSpace(environment)
	if len(serviceNames) == 0 || environment == "" {
		return nil, fmt.Errorf("listar deployments: escopo de serviços e ambiente são obrigatórios")
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("listar deployments: janela inválida")
	}
	if limit <= 0 || limit > maxDeploymentsPerQuery {
		return nil, fmt.Errorf("listar deployments: limite deve estar entre 1 e %d", maxDeploymentsPerQuery)
	}

	arguments := make([]any, 0, len(serviceNames)+4)
	for _, serviceName := range serviceNames {
		arguments = append(arguments, serviceName)
	}
	proximo := len(serviceNames) + 1
	arguments = append(arguments, environment, start.UTC(), end.UTC(), limit)

	query := fmt.Sprintf(`
		SELECT id, repository, environment, service_name, commit_sha, deployed_at, metadata_json
		FROM deployments
		WHERE service_name IN (%s)
			AND environment = $%d AND deployed_at >= $%d AND deployed_at < $%d
		ORDER BY deployed_at DESC, id ASC
		LIMIT $%d
	`, placeholders(len(serviceNames), 1), proximo, proximo+1, proximo+2, proximo+3)

	rows, err := repository.database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("listar deployments do escopo: %w", err)
	}
	defer func() { _ = rows.Close() }()

	deployments, err := scanDeployments(rows, "ler deployment do escopo")
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("percorrer deployments do escopo: %w", err)
	}
	return deployments, nil
}

// ListCommitMessagesBySHA devolve, em uma consulta só, a mensagem de cada
// commit solicitado.
//
// O rótulo de um commit suspeito precisa da mensagem para ser legível — quem
// investiga não decora SHA. Buscar uma por vez seria um N+1 que cresce com o
// número de serviços comparados.
func (repository *ChangeRepository) ListCommitMessagesBySHA(
	ctx context.Context,
	shas []string,
	limit int,
) (map[string]string, error) {
	if len(shas) == 0 {
		return map[string]string{}, nil
	}
	if limit <= 0 || limit > maxDeploymentsPerQuery {
		return nil, fmt.Errorf("listar commits: limite deve estar entre 1 e %d", maxDeploymentsPerQuery)
	}

	arguments := make([]any, 0, len(shas)+1)
	for _, sha := range shas {
		if trimmed := strings.TrimSpace(sha); trimmed != "" {
			arguments = append(arguments, trimmed)
		}
	}
	if len(arguments) == 0 {
		return map[string]string{}, nil
	}
	quantidade := len(arguments)
	arguments = append(arguments, limit)

	query := fmt.Sprintf(`
		SELECT sha, message
		FROM commits
		WHERE sha IN (%s)
		ORDER BY sha ASC
		LIMIT $%d
	`, placeholders(quantidade, 1), quantidade+1)

	rows, err := repository.database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("listar mensagens de commit: %w", err)
	}
	defer func() { _ = rows.Close() }()

	messages := make(map[string]string, quantidade)
	for rows.Next() {
		var sha string
		var message string
		if err := rows.Scan(&sha, &message); err != nil {
			return nil, fmt.Errorf("ler mensagem de commit: %w", err)
		}
		messages[sha] = message
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("percorrer mensagens de commit: %w", err)
	}
	return messages, nil
}

func scanDeployments(rows *sql.Rows, contexto string) ([]changedomain.Deployment, error) {
	deployments := make([]changedomain.Deployment, 0)
	for rows.Next() {
		var deployment changedomain.Deployment
		var metadataJSON string
		if err := rows.Scan(
			&deployment.ID, &deployment.Repository, &deployment.Environment,
			&deployment.ServiceName, &deployment.CommitSHA, &deployment.DeployedAt, &metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("%s: %w", contexto, err)
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

// prepareChanges valida e serializa antes de abrir a transação, mantendo curta
// a janela em que o banco fica ocupado.
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
		metadataJSON, err := json.Marshal(deploymentMetadata{
			Ref: deployment.Ref, Task: deployment.Task, State: deployment.State,
		})
		if err != nil {
			return preparedChanges{}, fmt.Errorf("serializar deployment %q: %w", deployment.ID, err)
		}
		prepared.deployments = append(prepared.deployments, preparedDeployment{
			change: deployment, metadataJSON: string(metadataJSON),
		})
	}
	return prepared, nil
}

func rollbackChangeTransaction(transaction *sql.Tx, cause error) error {
	if rollbackErr := transaction.Rollback(); rollbackErr != nil {
		return fmt.Errorf("salvar mudanças falhou: %w; rollback: %v", cause, rollbackErr)
	}
	return cause
}
