// Package payment implementa a API stateless de pagamentos da demonstração.
package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const maximumRequestBody = 1 << 20

// Payment representa o dado persistido pelo serviço de pagamento.
type Payment struct {
	OrderID     string `json:"order_id"`
	AmountCents int64  `json:"amount_cents"`
}

// Repository delimita a persistência para manter SQL fora do handler.
type Repository interface {
	Create(context.Context, Payment) error
}

// Scenario controla falhas HTTP reproduzíveis sem armazenar estado no processo.
type Scenario struct {
	ForceHTTPStatus int
}

// Handler valida pagamentos e delega sua persistência ao repositório.
type Handler struct {
	repository Repository
	scenario   Scenario
}

// NewHandler cria um handler com dependências compartilhadas pelo bootstrap.
func NewHandler(repository Repository, scenario Scenario) (*Handler, error) {
	if repository == nil {
		return nil, fmt.Errorf("criar handler de pagamento: repositório é obrigatório")
	}
	if scenario.ForceHTTPStatus != 0 && (scenario.ForceHTTPStatus < 400 || scenario.ForceHTTPStatus > 599) {
		return nil, fmt.Errorf("criar handler de pagamento: status forçado deve estar entre 400 e 599")
	}
	return &Handler{repository: repository, scenario: scenario}, nil
}

// ServeHTTP processa somente POST /payment e não expõe detalhes do banco ao cliente.
func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "método não permitido", http.StatusMethodNotAllowed)
		return
	}
	var payment Payment
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, maximumRequestBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payment); err != nil || decoder.Decode(&struct{}{}) != io.EOF || payment.OrderID == "" || payment.AmountCents <= 0 {
		http.Error(writer, "pagamento inválido", http.StatusBadRequest)
		return
	}
	if handler.scenario.ForceHTTPStatus != 0 {
		http.Error(writer, "falha simulada", handler.scenario.ForceHTTPStatus)
		return
	}
	if err := handler.repository.Create(request.Context(), payment); err != nil {
		if request.Context().Err() != nil {
			http.Error(writer, "requisição cancelada", http.StatusRequestTimeout)
			return
		}
		http.Error(writer, "não foi possível registrar o pagamento", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusCreated)
	if _, err := writer.Write([]byte(`{"status":"created"}`)); err != nil {
		return
	}
}
