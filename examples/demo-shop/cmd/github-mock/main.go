// O github-mock oferece somente a superfície da API do GitHub necessária para
// comprovar a correlação entre commit, deployment e incidente na demo E2E.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/faultmap/faultmap/examples/demo-shop/internal/demoruntime"
	"github.com/faultmap/faultmap/examples/demo-shop/internal/githubmock"
)

const shutdownTimeout = 5 * time.Second

type config struct {
	port          int
	repository    string
	sha           string
	environment   string
	token         string
	deploymentAge time.Duration
}

// main converte falhas de configuração ou rede em encerramento explícito do
// processo auxiliar, permitindo que o runner E2E detecte a falha.
func main() {
	ctx, cancel := demoruntime.SignalContext(context.Background())
	defer cancel()
	if err := run(ctx, demoruntime.NewEnvironment(os.LookupEnv)); err != nil {
		log.Printf("github-mock encerrado com erro: %v", err)
		os.Exit(1)
	}
}

// loadConfig valida a identidade que deve coincidir com os atributos emitidos
// pelo serviço e limita a idade do deployment à janela do detector.
func loadConfig(environment demoruntime.Environment) (config, error) {
	port, err := environment.Port("PORT", 9090)
	if err != nil {
		return config{}, err
	}
	repository, err := environment.Required("GITHUB_MOCK_REPOSITORY")
	if err != nil {
		return config{}, err
	}
	sha, err := environment.Required("GITHUB_MOCK_SHA")
	if err != nil {
		return config{}, err
	}
	deploymentEnvironment, err := environment.Required("GITHUB_MOCK_ENVIRONMENT")
	if err != nil {
		return config{}, err
	}
	token, err := environment.Required("GITHUB_MOCK_TOKEN")
	if err != nil {
		return config{}, err
	}
	deploymentAge, err := environment.Duration("GITHUB_MOCK_DEPLOYMENT_AGE", time.Minute, false)
	if err != nil {
		return config{}, err
	}
	if deploymentAge > time.Hour {
		return config{}, fmt.Errorf("GITHUB_MOCK_DEPLOYMENT_AGE deve ser no máximo %s", time.Hour)
	}
	return config{
		port: port, repository: repository, sha: sha, environment: deploymentEnvironment,
		token: token, deploymentAge: deploymentAge,
	}, nil
}

// run liga o mock somente ao loopback do container. Essa garantia permite que
// o cliente do Faultmap aceite HTTP local sem enfraquecer a proteção contra
// endpoints GitHub inseguros fora do ambiente E2E.
func run(ctx context.Context, environment demoruntime.Environment) error {
	settings, err := loadConfig(environment)
	if err != nil {
		return fmt.Errorf("carregar configuração: %w", err)
	}
	handler, err := githubmock.NewHandler(githubmock.Config{
		Repository: settings.repository, SHA: settings.sha, Environment: settings.environment,
		Token: settings.token, DeploymentAge: settings.deploymentAge,
	})
	if err != nil {
		return fmt.Errorf("criar handler: %w", err)
	}
	server := &http.Server{
		Addr: "127.0.0.1:" + fmt.Sprint(settings.port), Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	log.Printf("github-mock ouvindo em %s", server.Addr)
	return demoruntime.RunHTTPServer(ctx, server, shutdownTimeout)
}
