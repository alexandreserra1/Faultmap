package main

import (
	"testing"
	"time"

	"github.com/faultmap/faultmap/examples/demo-shop/internal/demoruntime"
)

// TestLoadConfigConvertePoolPostgres verifica que todos os limites do pool são
// explícitos antes que uma conexão seja aberta.
func TestLoadConfigConvertePoolPostgres(t *testing.T) {
	environment := demoruntime.NewEnvironment(mapLookup(map[string]string{
		"SERVICE_VERSION": "2.0.0", "DEPLOYMENT_ENVIRONMENT": "test", "OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector:4318",
		"DATABASE_URL": "postgres://demo:demo@postgres/demo", "DB_MAX_OPEN_CONNS": "8", "DB_MAX_IDLE_CONNS": "4",
		"DB_CONN_MAX_LIFETIME": "20m", "DB_CONN_MAX_IDLE_TIME": "2m", "DB_DELAY": "300ms", "FORCE_HTTP_STATUS": "500",
	}))
	config, err := loadConfig(environment)
	if err != nil {
		t.Fatalf("loadConfig() erro = %v", err)
	}
	if config.maxOpenConns != 8 || config.maxIdleConns != 4 || config.connMaxLifetime != 20*time.Minute || config.dbDelay != 300*time.Millisecond || config.forceHTTPStatus != 500 {
		t.Fatalf("configuração inesperada: %#v", config)
	}
}

// TestLoadConfigRejeitaIdleMaiorQueOpen preserva um pool coerente e previsível.
func TestLoadConfigRejeitaIdleMaiorQueOpen(t *testing.T) {
	environment := demoruntime.NewEnvironment(mapLookup(map[string]string{
		"SERVICE_VERSION": "1", "DEPLOYMENT_ENVIRONMENT": "test", "OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector:4318",
		"DATABASE_URL": "postgres://demo:demo@postgres/demo", "DB_MAX_OPEN_CONNS": "2", "DB_MAX_IDLE_CONNS": "3",
	}))
	if _, err := loadConfig(environment); err == nil {
		t.Fatal("esperava erro para idle maior que open")
	}
}

// mapLookup mantém cada teste independente do ambiente global.
func mapLookup(values map[string]string) demoruntime.Lookup {
	return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
}
