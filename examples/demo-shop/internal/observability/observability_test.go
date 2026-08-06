package observability

import (
	"context"
	"testing"
)

// TestTraceEndpointAcrescentaCaminhoOTLP garante a semântica da variável base
// usada pelo Collector sem depender do comportamento interno do exporter.
func TestTraceEndpointAcrescentaCaminhoOTLP(t *testing.T) {
	tests := map[string]string{
		"http://collector:4318":               "http://collector:4318/v1/traces",
		"http://collector:4318/":              "http://collector:4318/v1/traces",
		"https://collector.example/ingest":    "https://collector.example/ingest/v1/traces",
		"https://collector.example/v1/traces": "https://collector.example/v1/traces",
	}
	for input, expected := range tests {
		actual, err := traceEndpoint(input)
		if err != nil {
			t.Fatalf("traceEndpoint(%q) erro = %v", input, err)
		}
		if actual != expected {
			t.Fatalf("traceEndpoint(%q) = %q; esperado %q", input, actual, expected)
		}
	}
}

// TestConfigValidateExigeIdentidadeCompleta evita traces sem os atributos que
// permitem ao Faultmap correlacionar serviço, versão, ambiente e deployment.
func TestConfigValidateExigeIdentidadeCompleta(t *testing.T) {
	valid := Config{Endpoint: "http://collector:4318", ServiceName: "checkout-service", ServiceVersion: "1.2.3", DeploymentEnvironment: "demo"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() erro = %v", err)
	}
	invalid := []Config{
		{ServiceName: "checkout-service", ServiceVersion: "1", DeploymentEnvironment: "demo"},
		{Endpoint: "http://collector:4318", ServiceVersion: "1", DeploymentEnvironment: "demo"},
		{Endpoint: "http://collector:4318", ServiceName: "checkout-service", DeploymentEnvironment: "demo"},
		{Endpoint: "http://collector:4318", ServiceName: "checkout-service", ServiceVersion: "1"},
		{Endpoint: "://inválido", ServiceName: "checkout-service", ServiceVersion: "1", DeploymentEnvironment: "demo"},
	}
	for index, config := range invalid {
		if err := config.Validate(); err == nil {
			t.Fatalf("configuração inválida %d foi aceita", index)
		}
	}
}

// TestDisabledMantemShutdownSeguro permite testar bootstraps sem abrir uma
// conexão externa e mantém o contrato de encerramento idempotente.
func TestDisabledMantemShutdownSeguro(t *testing.T) {
	shutdown, err := Setup(context.Background(), Config{Disabled: true, ServiceName: "test", ServiceVersion: "test", DeploymentEnvironment: "test"})
	if err != nil {
		t.Fatalf("Setup() erro = %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown erro = %v", err)
	}
}
