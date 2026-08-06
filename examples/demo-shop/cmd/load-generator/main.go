// O load-generator envia uma quantidade finita de checkouts com concorrência limitada.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/faultmap/faultmap/examples/demo-shop/internal/demoruntime"
	"github.com/faultmap/faultmap/examples/demo-shop/internal/loadgen"
	"github.com/faultmap/faultmap/examples/demo-shop/internal/observability"
)

const shutdownTimeout = 5 * time.Second

type config struct {
	serviceVersion        string
	deploymentEnvironment string
	otelEndpoint          string
	targetURL             string
	requests              int
	concurrency           int
	requestTimeout        time.Duration
	requestInterval       time.Duration
}

// main transforma falhas de execução em código de saída observável pelo orquestrador.
func main() {
	ctx, cancel := demoruntime.SignalContext(context.Background())
	defer cancel()
	if err := run(ctx, demoruntime.NewEnvironment(os.LookupEnv)); err != nil {
		log.Printf("load-generator encerrado com erro: %v", err)
		os.Exit(1)
	}
}

// loadConfig aplica limites rígidos para evitar geração de carga acidentalmente ilimitada.
func loadConfig(environment demoruntime.Environment) (config, error) {
	serviceVersion, err := environment.Required("SERVICE_VERSION")
	if err != nil {
		return config{}, err
	}
	deploymentEnvironment, err := environment.Required("DEPLOYMENT_ENVIRONMENT")
	if err != nil {
		return config{}, err
	}
	otelEndpoint, err := environment.Required("OTEL_EXPORTER_OTLP_ENDPOINT")
	if err != nil {
		return config{}, err
	}
	targetURL, err := environment.Required("TARGET_URL")
	if err != nil {
		return config{}, err
	}
	parsedTargetURL, err := url.ParseRequestURI(targetURL)
	if err != nil || parsedTargetURL.Host == "" || (parsedTargetURL.Scheme != "http" && parsedTargetURL.Scheme != "https") {
		return config{}, errors.New("TARGET_URL deve ser uma URL HTTP ou HTTPS válida")
	}
	requests, err := environment.Int("REQUESTS", 100, 1, 100_000)
	if err != nil {
		return config{}, err
	}
	concurrency, err := environment.Int("CONCURRENCY", 5, 1, 100)
	if err != nil {
		return config{}, err
	}
	requestTimeout, err := environment.Duration("REQUEST_TIMEOUT", 5*time.Second, false)
	if err != nil {
		return config{}, err
	}
	requestInterval, err := environment.Duration("REQUEST_INTERVAL", 0, true)
	if err != nil {
		return config{}, err
	}
	return config{
		serviceVersion: serviceVersion, deploymentEnvironment: deploymentEnvironment, otelEndpoint: otelEndpoint,
		targetURL: targetURL, requests: requests, concurrency: concurrency, requestTimeout: requestTimeout, requestInterval: requestInterval,
	}, nil
}

// run configura tracing e aguarda todos os workers antes do flush final.
func run(ctx context.Context, environment demoruntime.Environment) (runErr error) {
	settings, err := loadConfig(environment)
	if err != nil {
		return fmt.Errorf("carregar configuração: %w", err)
	}
	shutdown, err := observability.Setup(ctx, observability.Config{
		Endpoint: settings.otelEndpoint, ServiceName: "load-generator", ServiceVersion: settings.serviceVersion,
		DeploymentEnvironment: settings.deploymentEnvironment,
	})
	if err != nil {
		return fmt.Errorf("configurar observabilidade: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		runErr = errors.Join(runErr, shutdown(shutdownCtx))
	}()

	client := &http.Client{}
	defer client.CloseIdleConnections()
	generator, err := loadgen.New(loadgen.Config{
		CheckoutURL: settings.targetURL, Requests: settings.requests, Concurrency: settings.concurrency,
		Interval: settings.requestInterval, RequestTimeout: settings.requestTimeout,
	}, client)
	if err != nil {
		return fmt.Errorf("criar gerador de carga: %w", err)
	}
	result, err := generator.Run(ctx)
	log.Printf("carga concluída: %d sucesso(s), %d falha(s)", result.Success, result.Failure)
	if err != nil {
		return fmt.Errorf("executar carga: %w", err)
	}
	return nil
}
