package main

import (
	"testing"
	"time"

	"github.com/faultmap/faultmap/examples/demo-shop/internal/demoruntime"
)

// TestLoadConfigConverteGitHubMock garante que a identidade do deployment
// usada pelo mock seja recebida explicitamente do cenário E2E.
func TestLoadConfigConverteGitHubMock(t *testing.T) {
	environment := demoruntime.NewEnvironment(mapLookup(map[string]string{
		"PORT": "9090", "GITHUB_MOCK_REPOSITORY": "acme/checkout",
		"GITHUB_MOCK_SHA": "abc123", "GITHUB_MOCK_ENVIRONMENT": "demo",
		"GITHUB_MOCK_TOKEN": "e2e-token", "GITHUB_MOCK_DEPLOYMENT_AGE": "90s",
	}))

	settings, err := loadConfig(environment)
	if err != nil {
		t.Fatalf("loadConfig() erro = %v", err)
	}
	if settings.port != 9090 || settings.repository != "acme/checkout" || settings.sha != "abc123" || settings.deploymentAge != 90*time.Second {
		t.Fatalf("configuração inesperada: %#v", settings)
	}
}

// TestLoadConfigExigeSHA evita um teste falso-positivo sem vínculo entre a
// versão observada na telemetria e o commit importado.
func TestLoadConfigExigeSHA(t *testing.T) {
	environment := demoruntime.NewEnvironment(mapLookup(map[string]string{
		"GITHUB_MOCK_REPOSITORY": "acme/checkout", "GITHUB_MOCK_ENVIRONMENT": "demo",
		"GITHUB_MOCK_TOKEN": "e2e-token",
	}))
	if _, err := loadConfig(environment); err == nil {
		t.Fatal("esperava erro quando GITHUB_MOCK_SHA está ausente")
	}
}

// mapLookup mantém o teste independente das variáveis do processo do usuário.
func mapLookup(values map[string]string) demoruntime.Lookup {
	return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
}
