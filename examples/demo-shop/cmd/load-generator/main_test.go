package main

import (
	"testing"
	"time"

	"github.com/faultmap/faultmap/examples/demo-shop/internal/demoruntime"
)

// TestLoadConfigConverteCargaLimitada cobre quantidade, concorrência, timeout
// e intervalo sem disparar tráfego durante o teste.
func TestLoadConfigConverteCargaLimitada(t *testing.T) {
	environment := demoruntime.NewEnvironment(mapLookup(map[string]string{
		"SERVICE_VERSION": "1.0.0", "DEPLOYMENT_ENVIRONMENT": "test", "OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector:4318",
		"TARGET_URL": "http://checkout/checkout", "REQUESTS": "25", "CONCURRENCY": "4",
		"REQUEST_TIMEOUT": "3s", "REQUEST_INTERVAL": "10ms",
	}))
	config, err := loadConfig(environment)
	if err != nil {
		t.Fatalf("loadConfig() erro = %v", err)
	}
	if config.requests != 25 || config.concurrency != 4 || config.requestTimeout != 3*time.Second || config.requestInterval != 10*time.Millisecond {
		t.Fatalf("configuração inesperada: %#v", config)
	}
}

// TestLoadConfigRejeitaConcorrenciaExcessiva impede carga acidentalmente ilimitada.
func TestLoadConfigRejeitaConcorrenciaExcessiva(t *testing.T) {
	environment := demoruntime.NewEnvironment(mapLookup(map[string]string{
		"SERVICE_VERSION": "1", "DEPLOYMENT_ENVIRONMENT": "test", "OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector:4318",
		"TARGET_URL": "http://checkout/checkout", "CONCURRENCY": "101",
	}))
	if _, err := loadConfig(environment); err == nil {
		t.Fatal("esperava erro para concorrência excessiva")
	}
}

// mapLookup mantém cada teste independente do ambiente global.
func mapLookup(values map[string]string) demoruntime.Lookup {
	return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
}
