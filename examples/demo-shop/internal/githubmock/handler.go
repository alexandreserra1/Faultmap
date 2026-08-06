// Package githubmock fornece uma implementação mínima e determinística da API
// do GitHub usada exclusivamente pelos testes E2E da demonstração.
package githubmock

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var repositoryPartPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// Config reúne os dados que precisam permanecer coerentes entre o commit, o
// deployment e a versão de serviço emitida pela telemetria da demo.
type Config struct {
	Repository    string
	SHA           string
	Environment   string
	Token         string
	DeploymentAge time.Duration
}

// NewHandler cria um servidor HTTP local compatível apenas com as operações
// consumidas pelo importador GitHub. Os instantes são calculados em cada
// requisição para manter o deployment próximo do incidente no teste E2E.
func NewHandler(config Config) (http.Handler, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
	})

	commitsPath := "/repos/" + config.Repository + "/commits"
	deploymentsPath := "/repos/" + config.Repository + "/deployments"
	mux.Handle("GET "+commitsPath, authenticate(config.Token, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		deploymentAt := time.Now().UTC().Add(-config.DeploymentAge).Truncate(time.Second)
		writeJSON(response, []any{
			map[string]any{
				"sha": config.SHA,
				"commit": map[string]any{
					"message":   "Versão E2E com regressão de timeout",
					"committer": map[string]any{"date": deploymentAt.Add(-time.Minute)},
				},
				"author": map[string]any{"login": "faultmap-e2e"},
			},
		})
	})))
	mux.Handle("GET "+deploymentsPath, authenticate(config.Token, http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		deploymentAt := time.Now().UTC().Add(-config.DeploymentAge).Truncate(time.Second)
		writeJSON(response, []any{
			map[string]any{
				"id": 42, "sha": config.SHA, "ref": "main", "task": "deploy",
				"environment": config.Environment, "created_at": deploymentAt,
			},
		})
	})))

	return mux, nil
}

// validateConfig impede que valores arbitrários alterem as rotas registradas
// no ServeMux ou produzam um deployment fora da janela aceita pelo detector.
func validateConfig(config Config) error {
	parts := strings.Split(config.Repository, "/")
	if len(parts) != 2 || !repositoryPartPattern.MatchString(parts[0]) || !repositoryPartPattern.MatchString(parts[1]) {
		return errors.New("repositório deve estar no formato owner/repo")
	}
	if strings.TrimSpace(config.SHA) == "" {
		return errors.New("SHA é obrigatório")
	}
	if strings.TrimSpace(config.Environment) == "" {
		return errors.New("ambiente é obrigatório")
	}
	if config.Token == "" {
		return errors.New("token é obrigatório")
	}
	if config.DeploymentAge <= 0 || config.DeploymentAge > time.Hour {
		return fmt.Errorf("idade do deployment deve estar entre zero e uma hora: %s", config.DeploymentAge)
	}
	return nil
}

// authenticate compara o cabeçalho completo em tempo constante para não
// introduzir um exemplo de autenticação vulnerável, mesmo sendo um mock local.
func authenticate(token string, next http.Handler) http.Handler {
	expected := []byte("Bearer " + token)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		provided := []byte(request.Header.Get("Authorization"))
		if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(response, request)
	})
}

// writeJSON mantém o contrato das respostas em um único lugar; como o corpo é
// formado apenas por valores internos, uma falha de codificação encerra a
// resposta sem registrar token ou outros dados potencialmente sensíveis.
func writeJSON(response http.ResponseWriter, value any) {
	response.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(response).Encode(value); err != nil {
		return
	}
}
