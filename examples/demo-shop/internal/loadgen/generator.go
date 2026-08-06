// Package loadgen produz carga limitada e cancelável contra o checkout.
package loadgen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	maximumRequests    = 100_000
	maximumConcurrency = 100
)

// Config limita a quantidade, concorrência e duração das requisições geradas.
type Config struct {
	CheckoutURL    string
	Requests       int
	Concurrency    int
	Interval       time.Duration
	RequestTimeout time.Duration
}

// Result resume os resultados sem reter respostas ou estado de sessão.
type Result struct {
	Success int64
	Failure int64
}

// Generator reutiliza um único cliente HTTP e executa uma fila limitada de trabalho.
type Generator struct {
	config Config
	client *http.Client
}

// New valida limites operacionais antes que qualquer carga seja enviada.
func New(config Config, client *http.Client) (*Generator, error) {
	checkoutURL, err := url.Parse(config.CheckoutURL)
	if err != nil {
		return nil, fmt.Errorf("validar TARGET_URL: %w", err)
	}
	if (checkoutURL.Scheme != "http" && checkoutURL.Scheme != "https") || checkoutURL.Host == "" {
		return nil, errors.New("TARGET_URL deve ser uma URL HTTP(S) absoluta")
	}
	if config.Requests < 1 || config.Requests > maximumRequests {
		return nil, fmt.Errorf("REQUESTS deve estar entre 1 e %d", maximumRequests)
	}
	if config.Concurrency == 0 {
		config.Concurrency = 1
	}
	if config.Concurrency < 1 || config.Concurrency > maximumConcurrency {
		return nil, fmt.Errorf("CONCURRENCY deve estar entre 1 e %d", maximumConcurrency)
	}
	if config.Interval < 0 {
		return nil, errors.New("INTERVAL não pode ser negativo")
	}
	if config.RequestTimeout <= 0 {
		return nil, errors.New("REQUEST_TIMEOUT deve ser positivo")
	}
	if client == nil {
		return nil, errors.New("cliente HTTP é obrigatório")
	}
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clientCopy := *client
	clientCopy.Transport = otelhttp.NewTransport(transport)
	return &Generator{config: config, client: &clientCopy}, nil
}

// Run distribui a quantidade total entre workers e encerra ao receber cancelamento.
func (generator *Generator) Run(ctx context.Context) (Result, error) {
	jobs := make(chan int)
	var success atomic.Int64
	var failure atomic.Int64
	var workers sync.WaitGroup
	for worker := 0; worker < generator.config.Concurrency; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				if err := generator.send(ctx, index); err != nil {
					failure.Add(1)
					continue
				}
				success.Add(1)
			}
		}()
	}

	var runErr error
	for index := 0; index < generator.config.Requests; index++ {
		select {
		case <-ctx.Done():
			runErr = ctx.Err()
			index = generator.config.Requests
		case jobs <- index:
		}
		if generator.config.Interval > 0 && runErr == nil {
			timer := time.NewTimer(generator.config.Interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				runErr = ctx.Err()
				index = generator.config.Requests
			case <-timer.C:
			}
		}
	}
	close(jobs)
	workers.Wait()
	return Result{Success: success.Load(), Failure: failure.Load()}, runErr
}

// send produz identificadores únicos para preservar a idempotência durante retries downstream.
func (generator *Generator) send(ctx context.Context, index int) error {
	payload, err := json.Marshal(map[string]any{
		"order_id":     fmt.Sprintf("load-%d-%d", time.Now().UnixNano(), index),
		"amount_cents": int64(1990),
	})
	if err != nil {
		return fmt.Errorf("serializar checkout: %w", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, generator.config.RequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, generator.config.CheckoutURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("criar checkout: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := generator.client.Do(request)
	if err != nil {
		return fmt.Errorf("executar checkout: %w", err)
	}
	_, copyErr := io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	closeErr := response.Body.Close()
	if copyErr != nil || closeErr != nil {
		return fmt.Errorf("consumir resposta do checkout: %w", errors.Join(copyErr, closeErr))
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("checkout retornou HTTP %d", response.StatusCode)
	}
	return nil
}
