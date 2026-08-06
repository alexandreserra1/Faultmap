// Package checkout implementa a API stateless que encaminha compras ao serviço de pagamentos.
package checkout

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const maximumRequestBody = 1 << 20

// Config concentra os limites do acesso ao serviço de pagamentos.
type Config struct {
	PaymentURL      string
	PaymentTimeout  time.Duration
	PaymentAttempts int
}

// Request representa os dados mínimos de uma compra usados pela demonstração.
type Request struct {
	OrderID     string `json:"order_id"`
	AmountCents int64  `json:"amount_cents"`
}

// Handler valida o checkout e encaminha o pagamento sem guardar sessão local.
type Handler struct {
	config Config
	client *http.Client
}

// NewHandler cria o handler e reutiliza o transporte do cliente informado com instrumentação OTEL.
func NewHandler(config Config, client *http.Client) (*Handler, error) {
	parsedURL, err := url.Parse(config.PaymentURL)
	if err != nil {
		return nil, fmt.Errorf("validar PAYMENT_URL: %w", err)
	}
	if (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return nil, errors.New("PAYMENT_URL deve ser uma URL HTTP(S) absoluta")
	}
	if config.PaymentTimeout <= 0 {
		return nil, errors.New("PAYMENT_TIMEOUT deve ser positivo")
	}
	if config.PaymentAttempts < 1 || config.PaymentAttempts > 10 {
		return nil, errors.New("PAYMENT_MAX_ATTEMPTS deve estar entre 1 e 10")
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
	return &Handler{config: config, client: &clientCopy}, nil
}

// ServeHTTP processa somente POST /checkout e traduz falhas downstream em respostas seguras.
func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "método não permitido", http.StatusMethodNotAllowed)
		return
	}
	var checkout Request
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, maximumRequestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&checkout); err != nil || decoder.Decode(&struct{}{}) != io.EOF || checkout.OrderID == "" || checkout.AmountCents <= 0 {
		http.Error(writer, "checkout inválido", http.StatusBadRequest)
		return
	}
	payload, err := json.Marshal(checkout)
	if err != nil {
		http.Error(writer, "não foi possível preparar o pagamento", http.StatusInternalServerError)
		return
	}

	status, err := handler.sendPayment(request.Context(), payload)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			http.Error(writer, "tempo do pagamento excedido", http.StatusGatewayTimeout)
			return
		}
		http.Error(writer, "pagamento indisponível", http.StatusBadGateway)
		return
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		http.Error(writer, "pagamento recusado", http.StatusBadGateway)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	if _, err := writer.Write([]byte(`{"status":"created"}`)); err != nil {
		return
	}
}

// sendPayment limita cada tentativa separadamente e mantém todas no contexto do trace recebido.
func (handler *Handler) sendPayment(ctx context.Context, payload []byte) (int, error) {
	var lastStatus int
	var lastErr error
	for attempt := 0; attempt < handler.config.PaymentAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		attemptCtx, cancel := context.WithTimeout(ctx, handler.config.PaymentTimeout)
		request, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, handler.config.PaymentURL, bytes.NewReader(payload))
		if err != nil {
			cancel()
			return 0, fmt.Errorf("criar requisição de pagamento: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := handler.client.Do(request)
		if err != nil {
			cancel()
			lastErr = err
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if ctx.Err() != nil {
					return 0, ctx.Err()
				}
			}
			continue
		}
		_, copyErr := io.Copy(io.Discard, io.LimitReader(response.Body, maximumRequestBody))
		closeErr := response.Body.Close()
		cancel()
		if copyErr != nil || closeErr != nil {
			lastErr = errors.Join(copyErr, closeErr)
			continue
		}
		lastStatus = response.StatusCode
		if response.StatusCode < http.StatusInternalServerError {
			return response.StatusCode, nil
		}
	}
	if lastErr != nil {
		return 0, fmt.Errorf("executar pagamento: %w", lastErr)
	}
	return lastStatus, nil
}
