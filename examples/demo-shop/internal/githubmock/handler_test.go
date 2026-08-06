package githubmock

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandlerEntregaCommitEDeploymentCoerentes(t *testing.T) {
	t.Parallel()

	const sha = "0123456789abcdef0123456789abcdef01234567"
	handler, err := NewHandler(Config{
		Repository: "acme/checkout", SHA: sha, Environment: "demo",
		Token: "e2e-token", DeploymentAge: time.Minute,
	})
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}

	before := time.Now().UTC()
	deploymentRequest := httptest.NewRequest(http.MethodGet, "/repos/acme/checkout/deployments?environment=demo&per_page=20", nil)
	deploymentRequest.Header.Set("Authorization", "Bearer e2e-token")
	deploymentResponse := httptest.NewRecorder()
	handler.ServeHTTP(deploymentResponse, deploymentRequest)
	if deploymentResponse.Code != http.StatusOK {
		t.Fatalf("deployments status = %d, corpo = %s", deploymentResponse.Code, deploymentResponse.Body.String())
	}
	var deployments []struct {
		ID          int64     `json:"id"`
		SHA         string    `json:"sha"`
		Environment string    `json:"environment"`
		CreatedAt   time.Time `json:"created_at"`
	}
	if err := json.Unmarshal(deploymentResponse.Body.Bytes(), &deployments); err != nil {
		t.Fatalf("decodificar deployments: %v", err)
	}
	if len(deployments) != 1 || deployments[0].ID != 42 || deployments[0].SHA != sha || deployments[0].Environment != "demo" {
		t.Fatalf("deployment inesperado: %#v", deployments)
	}
	expectedDeployment := before.Add(-time.Minute)
	if deployments[0].CreatedAt.Before(expectedDeployment.Add(-time.Second)) || deployments[0].CreatedAt.After(expectedDeployment.Add(time.Second)) {
		t.Fatalf("created_at = %s, esperado próximo de %s", deployments[0].CreatedAt, expectedDeployment)
	}

	commitRequest := httptest.NewRequest(http.MethodGet, "/repos/acme/checkout/commits?per_page=20", nil)
	commitRequest.Header.Set("Authorization", "Bearer e2e-token")
	commitResponse := httptest.NewRecorder()
	handler.ServeHTTP(commitResponse, commitRequest)
	if commitResponse.Code != http.StatusOK {
		t.Fatalf("commits status = %d, corpo = %s", commitResponse.Code, commitResponse.Body.String())
	}
	var commits []struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(commitResponse.Body.Bytes(), &commits); err != nil {
		t.Fatalf("decodificar commits: %v", err)
	}
	if len(commits) != 1 || commits[0].SHA != sha {
		t.Fatalf("commit inesperado: %#v", commits)
	}
}

func TestHandlerProtegeRotasEValidaConfiguracao(t *testing.T) {
	t.Parallel()

	if _, err := NewHandler(Config{}); err == nil {
		t.Fatal("NewHandler() erro = nil para configuração vazia")
	}
	handler, err := NewHandler(Config{
		Repository: "acme/checkout", SHA: "commit-e2e", Environment: "demo",
		Token: "e2e-token", DeploymentAge: time.Minute,
	})
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		token      string
		wantStatus int
	}{
		{name: "health", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK},
		{name: "sem token", method: http.MethodGet, path: "/repos/acme/checkout/deployments", wantStatus: http.StatusUnauthorized},
		{name: "token incorreto", method: http.MethodGet, path: "/repos/acme/checkout/deployments", token: "outro", wantStatus: http.StatusUnauthorized},
		{name: "método", method: http.MethodPost, path: "/repos/acme/checkout/deployments", token: "e2e-token", wantStatus: http.StatusMethodNotAllowed},
		{name: "rota", method: http.MethodGet, path: "/repos/outro/repo/deployments", token: "e2e-token", wantStatus: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(test.method, test.path, nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, esperado %d", response.Code, test.wantStatus)
			}
		})
	}
}
