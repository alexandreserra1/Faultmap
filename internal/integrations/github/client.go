// Package github integra a API REST do GitHub sem expor seus payloads às camadas internas.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
)

const (
	apiVersion       = "2026-03-10"
	maxGitHubPage    = 100
	maxResponseBytes = 4 << 20
)

// FetchRequest mantém compatibilidade nominal com o contrato neutro de importação.
type FetchRequest = changedomain.ImportRequest

// Snapshot mantém o retorno da integração independente dos payloads GitHub.
type Snapshot = changedomain.Snapshot

// Client executa chamadas autenticadas e limitadas contra uma API GitHub configurada.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	token      string
}

// NewClient cria um cliente que mantém o token somente em memória e nunca o inclui em erros.
func NewClient(httpClient *http.Client, baseURL, token string) (*Client, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("criar cliente GitHub: HTTP client é obrigatório")
	}
	parsedBaseURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || (parsedBaseURL.Scheme != "http" && parsedBaseURL.Scheme != "https") || parsedBaseURL.Host == "" {
		return nil, fmt.Errorf("criar cliente GitHub: URL base inválida")
	}
	if parsedBaseURL.User != nil || parsedBaseURL.RawQuery != "" || parsedBaseURL.Fragment != "" {
		return nil, fmt.Errorf("criar cliente GitHub: URL base não pode conter credenciais, query ou fragmento")
	}
	if parsedBaseURL.Scheme != "https" && !isLoopbackHost(parsedBaseURL.Hostname()) {
		return nil, fmt.Errorf("criar cliente GitHub: HTTPS é obrigatório fora do host local")
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("criar cliente GitHub: GITHUB_TOKEN é obrigatório")
	}
	return &Client{httpClient: httpClient, baseURL: parsedBaseURL, token: token}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

// Fetch consulta no máximo uma página por recurso para impedir trabalho e memória ilimitados.
func (client *Client) Fetch(ctx context.Context, request FetchRequest) (Snapshot, error) {
	owner, repository, err := validateFetchRequest(request)
	if err != nil {
		return Snapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("importar GitHub: contexto cancelado: %w", err)
	}

	snapshot := Snapshot{Commits: []changedomain.Commit{}, Deployments: []changedomain.Deployment{}}
	if request.IncludeCommits {
		commits, err := client.fetchCommits(ctx, owner, repository, request)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Commits = commits
	}
	if request.IncludeDeployments {
		deployments, err := client.fetchDeployments(ctx, owner, repository, request)
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.Deployments = deployments
	}
	return snapshot, nil
}

func (client *Client) fetchCommits(ctx context.Context, owner, repository string, request FetchRequest) ([]changedomain.Commit, error) {
	query := url.Values{
		"since":    []string{request.Since.UTC().Format(time.RFC3339)},
		"until":    []string{request.Until.UTC().Format(time.RFC3339)},
		"per_page": []string{strconv.Itoa(request.Limit)},
	}
	var payload []commitResponse
	if err := client.getJSON(ctx, repositoryPath(owner, repository, "commits"), query, &payload); err != nil {
		return nil, fmt.Errorf("importar commits GitHub: %w", err)
	}
	commits := make([]changedomain.Commit, 0, len(payload))
	for _, source := range payload {
		committedAt := source.Commit.Committer.Date
		if committedAt.IsZero() {
			committedAt = source.Commit.Author.Date
		}
		author := strings.TrimSpace(source.Author.Login)
		if author == "" {
			author = strings.TrimSpace(source.Commit.Author.Name)
		}
		commit := changedomain.Commit{
			SHA: source.SHA, Repository: request.Repository, Author: author,
			Message: source.Commit.Message, CommittedAt: committedAt.UTC(), Files: []string{},
		}
		if err := commit.Validate(); err != nil {
			return nil, fmt.Errorf("normalizar commit GitHub: %w", err)
		}
		commits = append(commits, commit)
	}
	sort.Slice(commits, func(first, second int) bool {
		if !commits[first].CommittedAt.Equal(commits[second].CommittedAt) {
			return commits[first].CommittedAt.After(commits[second].CommittedAt)
		}
		return commits[first].SHA < commits[second].SHA
	})
	return commits, nil
}

func (client *Client) fetchDeployments(ctx context.Context, owner, repository string, request FetchRequest) ([]changedomain.Deployment, error) {
	query := url.Values{
		"environment": []string{request.Environment},
		"per_page":    []string{strconv.Itoa(request.Limit)},
	}
	var payload []deploymentResponse
	if err := client.getJSON(ctx, repositoryPath(owner, repository, "deployments"), query, &payload); err != nil {
		return nil, fmt.Errorf("importar deployments GitHub: %w", err)
	}
	deployments := make([]changedomain.Deployment, 0, len(payload))
	for _, source := range payload {
		if source.CreatedAt.Before(request.Since) || source.CreatedAt.After(request.Until) {
			continue
		}
		deployment := changedomain.Deployment{
			ID:         fmt.Sprintf("github:%s:deployment:%d", request.Repository, source.ID),
			Repository: request.Repository, Environment: source.Environment,
			ServiceName: request.ServiceName, CommitSHA: source.SHA,
			Ref: source.Ref, Task: source.Task, State: "unknown", DeployedAt: source.CreatedAt.UTC(),
		}
		if err := deployment.Validate(); err != nil {
			return nil, fmt.Errorf("normalizar deployment GitHub: %w", err)
		}
		deployments = append(deployments, deployment)
	}
	sort.Slice(deployments, func(first, second int) bool {
		if !deployments[first].DeployedAt.Equal(deployments[second].DeployedAt) {
			return deployments[first].DeployedAt.After(deployments[second].DeployedAt)
		}
		return deployments[first].ID < deployments[second].ID
	})
	return deployments, nil
}

func (client *Client) getJSON(ctx context.Context, path string, query url.Values, destination any) error {
	endpoint := *client.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + path
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("criar requisição: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+client.token)
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("executar requisição: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	closeErr := response.Body.Close()
	if readErr != nil {
		return fmt.Errorf("ler resposta: %w", readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("fechar resposta: %w", closeErr)
	}
	if len(body) > maxResponseBytes {
		return fmt.Errorf("resposta excede limite de %d bytes", maxResponseBytes)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("GitHub respondeu HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("interpretar resposta JSON: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("interpretar resposta JSON: dados adicionais")
	}
	return nil
}

func validateFetchRequest(request FetchRequest) (string, string, error) {
	parts := strings.Split(request.Repository, "/")
	if len(parts) != 2 || !validRepositoryPart(parts[0]) || !validRepositoryPart(parts[1]) {
		return "", "", fmt.Errorf("importar GitHub: --repo deve usar owner/repository")
	}
	if request.Limit <= 0 || request.Limit > maxGitHubPage {
		return "", "", fmt.Errorf("importar GitHub: limite deve estar entre 1 e %d", maxGitHubPage)
	}
	if request.Since.IsZero() || request.Until.IsZero() || !request.Since.Before(request.Until) {
		return "", "", fmt.Errorf("importar GitHub: janela inválida")
	}
	if !request.IncludeCommits && !request.IncludeDeployments {
		return "", "", fmt.Errorf("importar GitHub: selecione commits ou deployments")
	}
	if request.IncludeDeployments && (strings.TrimSpace(request.Environment) == "" || strings.TrimSpace(request.ServiceName) == "") {
		return "", "", fmt.Errorf("importar GitHub: ambiente e serviço são obrigatórios para deployments")
	}
	return parts[0], parts[1], nil
}

func validRepositoryPart(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, "\\?#")
}

func repositoryPath(owner, repository, resource string) string {
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repository) + "/" + resource
}

type commitResponse struct {
	SHA    string `json:"sha"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"author"`
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

type deploymentResponse struct {
	ID          int64     `json:"id"`
	SHA         string    `json:"sha"`
	Ref         string    `json:"ref"`
	Task        string    `json:"task"`
	Environment string    `json:"environment"`
	CreatedAt   time.Time `json:"created_at"`
}
