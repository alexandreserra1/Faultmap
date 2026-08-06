package demoruntime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestNewHTTPServerConfiguraLimitesEHealth verifica as proteções que devem ser
// iguais em todos os serviços HTTP da demonstração.
func TestNewHTTPServerConfiguraLimitesEHealth(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) })
	server, err := NewHTTPServer(8080, "/work", handler)
	if err != nil {
		t.Fatalf("NewHTTPServer() erro = %v", err)
	}
	if server.ReadHeaderTimeout <= 0 || server.ReadTimeout <= 0 || server.WriteTimeout <= 0 || server.IdleTimeout <= 0 {
		t.Fatalf("timeouts não configurados: %#v", server)
	}

	health := httptest.NewRecorder()
	server.Handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusOK || health.Body.String() != `{"status":"ok"}` {
		t.Fatalf("health = %d %q", health.Code, health.Body.String())
	}
	work := httptest.NewRecorder()
	server.Handler.ServeHTTP(work, httptest.NewRequest(http.MethodPost, "/work", nil))
	if work.Code != http.StatusNoContent {
		t.Fatalf("work status = %d", work.Code)
	}
}

// TestRunHTTPServerEncerraComContexto garante shutdown limitado sem depender
// de sinais reais do sistema operacional no teste.
func TestRunHTTPServerEncerraComContexto(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := &http.Server{Addr: "127.0.0.1:0", Handler: http.NewServeMux(), ReadHeaderTimeout: time.Second}
	if err := RunHTTPServer(ctx, server, time.Second); err != nil {
		t.Fatalf("RunHTTPServer() erro = %v", err)
	}
}
