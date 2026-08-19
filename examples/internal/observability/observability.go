// Package observability configura tracing OTLP compartilhado pelos processos
// da demonstração e mantém detalhes do SDK fora dos handlers.
package observability

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

// Config identifica a origem dos traces e o receiver que os receberá.
type Config struct {
	Endpoint              string
	ServiceName           string
	ServiceVersion        string
	DeploymentEnvironment string
	Disabled              bool
}

// Validate impede traces sem identidade e endpoints ambíguos.
func (config Config) Validate() error {
	if config.Disabled {
		return nil
	}
	if strings.TrimSpace(config.Endpoint) == "" || strings.TrimSpace(config.ServiceName) == "" || strings.TrimSpace(config.ServiceVersion) == "" || strings.TrimSpace(config.DeploymentEnvironment) == "" {
		return errors.New("endpoint, service.name, service.version e deployment.environment.name são obrigatórios")
	}
	if _, err := traceEndpoint(config.Endpoint); err != nil {
		return errors.New("OTEL_EXPORTER_OTLP_ENDPOINT deve ser uma URL HTTP ou HTTPS válida")
	}
	return nil
}

// Setup instala provider e propagadores globais; o shutdown devolvido deve ser
// chamado no encerramento para exportar os spans ainda em memória.
func Setup(ctx context.Context, config Config) (func(context.Context) error, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if config.Disabled {
		return func(context.Context) error { return nil }, nil
	}
	endpoint, err := traceEndpoint(config.Endpoint)
	if err != nil {
		return nil, err
	}
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
	if err != nil {
		return nil, fmt.Errorf("criar exportador OTLP HTTP: %w", err)
	}
	resources, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			attribute.String("service.name", config.ServiceName),
			attribute.String("service.version", config.ServiceVersion),
			attribute.String("deployment.environment.name", config.DeploymentEnvironment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("criar resource OTEL: %w", err)
	}
	provider := tracesdk.NewTracerProvider(
		tracesdk.WithResource(resources),
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		tracesdk.WithBatcher(exporter),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}

// traceEndpoint converte o endpoint OTLP base no caminho exato de traces
// esperado por WithEndpointURL, preservando prefixos de proxy configurados.
func traceEndpoint(raw string) (string, error) {
	endpoint, err := url.ParseRequestURI(raw)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return "", errors.New("endpoint OTLP deve ser uma URL HTTP ou HTTPS válida")
	}
	path := strings.TrimRight(endpoint.Path, "/")
	if !strings.HasSuffix(path, "/v1/traces") {
		path += "/v1/traces"
	}
	endpoint.Path = path
	return endpoint.String(), nil
}
