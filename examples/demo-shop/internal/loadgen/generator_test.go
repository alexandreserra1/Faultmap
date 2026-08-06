package loadgen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestGeneratorEnviaQuantidadeLimitada(t *testing.T) {
	var chamadas atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamadas.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/checkout" {
			t.Fatalf("requisição inesperada: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	generator, err := New(Config{CheckoutURL: server.URL + "/checkout", Requests: 3, Interval: time.Millisecond, RequestTimeout: time.Second}, server.Client())
	if err != nil {
		t.Fatalf("New() erro = %v", err)
	}
	result, err := generator.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() erro = %v", err)
	}
	if result.Success != 3 || chamadas.Load() != 3 {
		t.Fatalf("resultado=%+v chamadas=%d", result, chamadas.Load())
	}
}

func TestGeneratorRespeitaCancelamento(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	generator, err := New(Config{CheckoutURL: server.URL, Requests: 10, Interval: time.Millisecond, RequestTimeout: time.Minute}, server.Client())
	if err != nil {
		t.Fatalf("New() erro = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := generator.Run(ctx); err == nil {
		t.Fatal("Run() deveria respeitar cancelamento")
	}
}

func TestGeneratorLimitaConcorrencia(t *testing.T) {
	var atuais atomic.Int32
	var maximo atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		current := atuais.Add(1)
		defer atuais.Add(-1)
		for {
			seen := maximo.Load()
			if current <= seen || maximo.CompareAndSwap(seen, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	generator, err := New(Config{CheckoutURL: server.URL, Requests: 12, Concurrency: 3, RequestTimeout: time.Second}, server.Client())
	if err != nil {
		t.Fatalf("New() erro = %v", err)
	}
	if _, err := generator.Run(context.Background()); err != nil {
		t.Fatalf("Run() erro = %v", err)
	}
	if got := maximo.Load(); got != 3 {
		t.Fatalf("concorrência máxima = %d, esperado 3", got)
	}
}

func TestGeneratorRejeitaLimitesPerigosos(t *testing.T) {
	for _, config := range []Config{
		{CheckoutURL: "/checkout", Requests: 1, Concurrency: 1, RequestTimeout: time.Second},
		{CheckoutURL: "http://checkout", Requests: 100_001, Concurrency: 1, RequestTimeout: time.Second},
		{CheckoutURL: "http://checkout", Requests: 1, Concurrency: 101, RequestTimeout: time.Second},
	} {
		if _, err := New(config, http.DefaultClient); err == nil {
			t.Fatalf("New(%+v) deveria rejeitar configuração", config)
		}
	}
}
