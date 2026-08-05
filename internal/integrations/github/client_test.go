package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestClientFetchImportsBoundedCommitsAndDeployments garante uma chamada por
// recurso, filtros explícitos e normalização sem tipos da API nas camadas superiores.
func TestClientFetchImportsBoundedCommitsAndDeployments(t *testing.T) {
	t.Parallel()

	since := time.Date(2025, time.December, 1, 9, 0, 0, 0, time.UTC)
	until := time.Date(2025, time.December, 1, 11, 0, 0, 0, time.UTC)
	requests := make(chan *http.Request, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests <- request.Clone(context.Background())
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/repos/acme/checkout/commits":
			if _, err := writer.Write([]byte(`[{"sha":"abc123","commit":{"message":"Reduce pool","author":{"name":"Alex","date":"2025-12-01T09:50:00Z"},"committer":{"date":"2025-12-01T09:51:00Z"}},"author":{"login":"alex"}}]`)); err != nil {
				t.Errorf("escrever resposta de commits: %v", err)
			}
		case "/repos/acme/checkout/deployments":
			if _, err := writer.Write([]byte(`[{"id":42,"sha":"abc123","ref":"main","task":"deploy","environment":"staging","created_at":"2025-12-01T09:55:00Z"},{"id":43,"sha":"future","environment":"staging","created_at":"2025-12-01T12:00:00Z"}]`)); err != nil {
				t.Errorf("escrever resposta de deployments: %v", err)
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.Client(), server.URL, "secret-token")
	if err != nil {
		t.Fatalf("NewClient() erro = %v", err)
	}
	snapshot, err := client.Fetch(context.Background(), FetchRequest{
		Repository: "acme/checkout", Environment: "staging", ServiceName: "checkout-service",
		Since: since, Until: until, Limit: 20, IncludeCommits: true, IncludeDeployments: true,
	})
	if err != nil {
		t.Fatalf("Fetch() erro = %v", err)
	}
	if len(snapshot.Commits) != 1 || snapshot.Commits[0].SHA != "abc123" || snapshot.Commits[0].Author != "alex" {
		t.Fatalf("commits = %#v", snapshot.Commits)
	}
	if !snapshot.Commits[0].CommittedAt.Equal(time.Date(2025, time.December, 1, 9, 51, 0, 0, time.UTC)) {
		t.Fatalf("CommittedAt = %s", snapshot.Commits[0].CommittedAt)
	}
	if len(snapshot.Deployments) != 1 {
		t.Fatalf("deployments = %#v, esperado somente item dentro da janela", snapshot.Deployments)
	}
	deployment := snapshot.Deployments[0]
	if deployment.ID != "github:acme/checkout:deployment:42" || deployment.CommitSHA != "abc123" || deployment.ServiceName != "checkout-service" {
		t.Fatalf("deployment = %#v", deployment)
	}

	for index := 0; index < 2; index++ {
		request := <-requests
		if request.Header.Get("Authorization") != "Bearer secret-token" {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("Accept") != "application/vnd.github+json" || request.Header.Get("X-GitHub-Api-Version") == "" {
			t.Errorf("headers GitHub ausentes: %#v", request.Header)
		}
		assertQueryValue(t, request.URL.Query(), "per_page", "20")
		if strings.HasSuffix(request.URL.Path, "/commits") {
			assertQueryValue(t, request.URL.Query(), "since", since.Format(time.RFC3339))
			assertQueryValue(t, request.URL.Query(), "until", until.Format(time.RFC3339))
		} else {
			assertQueryValue(t, request.URL.Query(), "environment", "staging")
		}
	}
}

// TestClientFetchRejectsInvalidInputBeforeNetwork impede URLs manipuladas e
// coleções acima do limite aceito pela API.
func TestClientFetchRejectsInvalidInputBeforeNetwork(t *testing.T) {
	t.Parallel()

	client, err := NewClient(http.DefaultClient, "https://api.github.test", "token")
	if err != nil {
		t.Fatalf("NewClient() erro = %v", err)
	}
	valid := FetchRequest{
		Repository: "acme/checkout", Environment: "staging", ServiceName: "checkout-service",
		Since: time.Now().Add(-time.Hour), Until: time.Now(), Limit: 20, IncludeDeployments: true,
	}
	for _, mutate := range []func(*FetchRequest){
		func(request *FetchRequest) { request.Repository = "acme" },
		func(request *FetchRequest) { request.Repository = "../secrets" },
		func(request *FetchRequest) { request.Limit = 0 },
		func(request *FetchRequest) { request.Limit = 101 },
		func(request *FetchRequest) { request.Until = request.Since },
		func(request *FetchRequest) { request.IncludeDeployments = false },
	} {
		request := valid
		mutate(&request)
		if _, err := client.Fetch(context.Background(), request); err == nil {
			t.Fatalf("Fetch() erro = nil para %#v", request)
		}
	}
}

// TestClientFetchPreservesCancellationAndDoesNotLeakToken protege timeout e credenciais.
func TestClientFetchPreservesCancellationAndDoesNotLeakToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()
	client, err := NewClient(server.Client(), server.URL, "highly-secret")
	if err != nil {
		t.Fatalf("NewClient() erro = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.Fetch(ctx, FetchRequest{
		Repository: "acme/checkout", Environment: "staging", ServiceName: "checkout-service",
		Since: time.Now().Add(-time.Hour), Until: time.Now(), Limit: 1, IncludeCommits: true,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch() erro = %v, esperado context.Canceled", err)
	}
	if strings.Contains(err.Error(), "highly-secret") {
		t.Fatalf("erro expôs token: %v", err)
	}
}

// TestNewClientRejectsTokenOverInsecureRemoteURL evita enviar a credencial para
// HTTP remoto ou para URLs com componentes inesperados.
func TestNewClientRejectsTokenOverInsecureRemoteURL(t *testing.T) {
	t.Parallel()

	for _, baseURL := range []string{
		"http://api.example.com",
		"https://user@example.com",
		"https://api.example.com?redirect=evil",
		"https://api.example.com#fragment",
	} {
		if _, err := NewClient(http.DefaultClient, baseURL, "token"); err == nil {
			t.Fatalf("NewClient() erro = nil para %q", baseURL)
		}
	}
}

func assertQueryValue(t *testing.T, query url.Values, key, expected string) {
	t.Helper()
	if got := query.Get(key); got != expected {
		t.Errorf("query %s = %q, esperado %q", key, got, expected)
	}
}
