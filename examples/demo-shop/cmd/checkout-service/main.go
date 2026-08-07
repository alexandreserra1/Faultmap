// O checkout-service expõe uma API stateless e encaminha pagamentos com traces distribuídos.
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

	"github.com/faultmap/faultmap/examples/demo-shop/internal/checkout"
	"github.com/faultmap/faultmap/examples/demo-shop/internal/demoruntime"
	"github.com/faultmap/faultmap/examples/demo-shop/internal/observability"
)

const shutdownTimeout = 5 * time.Second

type config struct {
	port                  int
	serviceVersion        string
	deploymentEnvironment string
	otelEndpoint          string
	paymentURL            string
	paymentTimeout        time.Duration
	paymentAttempts       int
	paymentFanOut         int
}

// main transforma falhas de bootstrap em encerramento explícito do processo.
func main() {
	ctx, cancel := demoruntime.SignalContext(context.Background())
	defer cancel()
	if err := run(ctx, demoruntime.NewEnvironment(os.LookupEnv)); err != nil {
		log.Printf("checkout-service encerrado com erro: %v", err)
		os.Exit(1)
	}
}

// loadConfig valida toda a configuração antes de abrir rede ou exportadores.
func loadConfig(environment demoruntime.Environment) (config, error) {
	port, err := environment.Port("PORT", 8080)
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
	paymentURL, err := environment.Required("PAYMENT_URL")
	if err != nil {
		return config{}, err
	}
	parsedPaymentURL, err := url.ParseRequestURI(paymentURL)
	if err != nil || parsedPaymentURL.Host == "" || (parsedPaymentURL.Scheme != "http" && parsedPaymentURL.Scheme != "https") {
		return config{}, errors.New("PAYMENT_URL deve ser uma URL HTTP ou HTTPS válida")
	}
	paymentTimeout, err := environment.Duration("PAYMENT_TIMEOUT", 2*time.Second, false)
	if err != nil {
		return config{}, err
	}
	paymentAttempts, err := environment.Int("PAYMENT_MAX_ATTEMPTS", 1, 1, 10)
	if err != nil {
		return config{}, err
	}
	paymentFanOut, err := environment.Int("PAYMENT_FANOUT", 0, 0, 10)
	if err != nil {
		return config{}, err
	}
	return config{
		port: port, serviceVersion: serviceVersion, deploymentEnvironment: deploymentEnvironment,
		otelEndpoint: otelEndpoint, paymentURL: paymentURL, paymentTimeout: paymentTimeout, paymentAttempts: paymentAttempts,
		paymentFanOut: paymentFanOut,
	}, nil
}

// run compõe dependências compartilhadas e garante flush de telemetria ao encerrar.
func run(ctx context.Context, environment demoruntime.Environment) (runErr error) {
	settings, err := loadConfig(environment)
	if err != nil {
		return fmt.Errorf("carregar configuração: %w", err)
	}
	shutdown, err := observability.Setup(ctx, observability.Config{
		Endpoint: settings.otelEndpoint, ServiceName: "checkout-service", ServiceVersion: settings.serviceVersion,
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
	handler, err := checkout.NewHandler(checkout.Config{
		PaymentURL: settings.paymentURL, PaymentTimeout: settings.paymentTimeout, PaymentAttempts: settings.paymentAttempts,
		PaymentFanOut: settings.paymentFanOut,
	}, client)
	if err != nil {
		return fmt.Errorf("criar checkout: %w", err)
	}
	server, err := demoruntime.NewHTTPServer(settings.port, "POST /checkout", handler)
	if err != nil {
		return fmt.Errorf("criar servidor HTTP: %w", err)
	}
	log.Printf("checkout-service ouvindo em %s", server.Addr)
	return demoruntime.RunHTTPServer(ctx, server, shutdownTimeout)
}
