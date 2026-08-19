// O pilot-gateway põe um segundo serviço na frente da aplicação sob piloto,
// para que o ranking do Faultmap tenha entre quem escolher.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/faultmap/faultmap/examples/internal/demoruntime"
	"github.com/faultmap/faultmap/examples/internal/observability"
	"github.com/faultmap/faultmap/examples/pilot/gateway"
)

const shutdownTimeout = 5 * time.Second

type config struct {
	port                  int
	serviceName           string
	serviceVersion        string
	deploymentEnvironment string
	otelEndpoint          string
	upstreamURL           string
	delayFile             string
}

func main() {
	ctx, cancel := demoruntime.SignalContext(context.Background())
	defer cancel()
	if err := run(ctx, demoruntime.NewEnvironment(os.LookupEnv)); err != nil {
		log.Printf("pilot-gateway encerrado com erro: %v", err)
		os.Exit(1)
	}
}

// loadConfig valida tudo antes de abrir rede ou exportador, para que um erro de
// configuração apareça na largada e não no meio de um incidente.
func loadConfig(environment demoruntime.Environment) (config, error) {
	port, err := environment.Port("PORT", 8100)
	if err != nil {
		return config{}, err
	}
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
	upstreamURL, err := environment.Required("UPSTREAM_URL")
	if err != nil {
		return config{}, err
	}
	return config{
		port:                  port,
		serviceName:           environment.Value("SERVICE_NAME", "strideredge-gateway"),
		serviceVersion:        serviceVersion,
		deploymentEnvironment: deploymentEnvironment,
		otelEndpoint:          otelEndpoint,
		upstreamURL:           upstreamURL,
		delayFile:             environment.Value("GATEWAY_DELAY_FILE", ""),
	}, nil
}

func run(ctx context.Context, environment demoruntime.Environment) (runErr error) {
	settings, err := loadConfig(environment)
	if err != nil {
		return fmt.Errorf("carregar configuração: %w", err)
	}
	shutdown, err := observability.Setup(ctx, observability.Config{
		Endpoint:              settings.otelEndpoint,
		ServiceName:           settings.serviceName,
		ServiceVersion:        settings.serviceVersion,
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

	handler, err := gateway.NewHandler(settings.upstreamURL)
	if err != nil {
		return fmt.Errorf("criar proxy: %w", err)
	}
	if settings.delayFile != "" {
		handler = gateway.WithDelayFile(handler, settings.delayFile)
	}

	// A rota é "/" porque o proxy repassa qualquer caminho: decidir o que passa
	// e o que não passa é trabalho de gateway, e um gateway traria suspeitos
	// falsos para o diagnóstico.
	server, err := demoruntime.NewHTTPServer(settings.port, "/", handler)
	if err != nil {
		return fmt.Errorf("criar servidor HTTP: %w", err)
	}
	log.Printf("pilot-gateway %s na porta %d encaminhando para %s", settings.serviceName, settings.port, settings.upstreamURL)
	return demoruntime.RunHTTPServer(ctx, server, shutdownTimeout)
}
