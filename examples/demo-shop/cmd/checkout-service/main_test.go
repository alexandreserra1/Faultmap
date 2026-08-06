package main

import (
	"testing"
	"time"

	"github.com/faultmap/faultmap/examples/demo-shop/internal/demoruntime"
)

// TestLoadConfigConverteCheckout cobre todos os controles que alteram rede,
// retries e identidade observável do serviço.
func TestLoadConfigConverteCheckout(t *testing.T) {
	environment := demoruntime.NewEnvironment(mapLookup(map[string]string{
		"PORT": "9090", "SERVICE_VERSION": "2.0.0", "DEPLOYMENT_ENVIRONMENT": "test",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector:4318", "PAYMENT_URL": "http://payment/payment",
		"PAYMENT_TIMEOUT": "750ms", "PAYMENT_MAX_ATTEMPTS": "3",
	}))
	config, err := loadConfig(environment)
	if err != nil {
		t.Fatalf("loadConfig() erro = %v", err)
	}
	if config.port != 9090 || config.paymentTimeout != 750*time.Millisecond || config.paymentAttempts != 3 || config.serviceVersion != "2.0.0" {
		t.Fatalf("configuração inesperada: %#v", config)
	}
}

// TestLoadConfigRejeitaRetryIlimitado evita tempestades causadas por erro de configuração.
func TestLoadConfigRejeitaRetryIlimitado(t *testing.T) {
	environment := demoruntime.NewEnvironment(mapLookup(map[string]string{
		"SERVICE_VERSION": "1", "DEPLOYMENT_ENVIRONMENT": "test", "OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector:4318",
		"PAYMENT_URL": "http://payment/payment", "PAYMENT_MAX_ATTEMPTS": "999",
	}))
	if _, err := loadConfig(environment); err == nil {
		t.Fatal("esperava erro para tentativas acima do limite")
	}
}

// mapLookup mantém cada teste independente do ambiente global.
func mapLookup(values map[string]string) demoruntime.Lookup {
	return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
}
