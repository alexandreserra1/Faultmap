package payment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHandlerPersistePagamento(t *testing.T) {
	repository := &repositoryStub{}
	handler := mustHandler(t, repository, Scenario{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/payment", strings.NewReader(`{"order_id":"order-1","amount_cents":1990}`)))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, esperado %d; corpo=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if repository.calls.Load() != 1 || repository.last.OrderID != "order-1" {
		t.Fatalf("persistência inesperada: calls=%d payment=%+v", repository.calls.Load(), repository.last)
	}
}

func TestHandlerForcaStatusSemPersistir(t *testing.T) {
	repository := &repositoryStub{}
	handler := mustHandler(t, repository, Scenario{ForceHTTPStatus: http.StatusServiceUnavailable})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/payment", strings.NewReader(`{"order_id":"order-3","amount_cents":3000}`)))

	if recorder.Code != http.StatusServiceUnavailable || repository.calls.Load() != 0 {
		t.Fatalf("status=%d chamadas=%d", recorder.Code, repository.calls.Load())
	}
}

func TestHandlerConverteFalhaDoBancoSemVazarDetalhes(t *testing.T) {
	repository := &repositoryStub{err: errors.New("senha secreta do banco")}
	handler := mustHandler(t, repository, Scenario{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/payment", strings.NewReader(`{"order_id":"order-4","amount_cents":4000}`)))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "senha secreta") {
		t.Fatalf("resposta vazou detalhe de infraestrutura: %s", recorder.Body.String())
	}
}

func TestHandlerRejeitaJSONComDadosAdicionais(t *testing.T) {
	t.Parallel()

	handler := mustHandler(t, &repositoryStub{}, Scenario{})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/payment", strings.NewReader(`{"order_id":"order-5","amount_cents":5000} {}`)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, esperado 400", recorder.Code)
	}
}

func TestNewHandlerValidaDependenciaECenario(t *testing.T) {
	t.Parallel()

	if _, err := NewHandler(nil, Scenario{}); err == nil {
		t.Fatal("NewHandler() deveria rejeitar repositório ausente")
	}
	if _, err := NewHandler(&repositoryStub{}, Scenario{ForceHTTPStatus: 200}); err == nil {
		t.Fatal("NewHandler() deveria rejeitar status forçado fora da faixa de erro")
	}
}

func mustHandler(t *testing.T, repository Repository, scenario Scenario) *Handler {
	t.Helper()
	handler, err := NewHandler(repository, scenario)
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}
	return handler
}

type repositoryStub struct {
	calls atomic.Int32
	last  Payment
	err   error
}

func (stub *repositoryStub) Create(_ context.Context, payment Payment) error {
	stub.calls.Add(1)
	stub.last = payment
	return stub.err
}

// TestHandlerErroCrônicoÉDeterminísticoPorPedido garante que a taxa de erro de
// fundo depende apenas do order_id. Sem contador nem sorteio, o handler
// permanece stateless e o mesmo pedido falha igual em qualquer execução.
func TestHandlerErroCrônicoÉDeterminísticoPorPedido(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler(&repositoryStub{}, Scenario{ChronicErrorPercent: 25})
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}

	primeiraPassada := make(map[string]int, 40)
	falhas := 0
	for index := 0; index < 40; index++ {
		orderID := fmt.Sprintf("pedido-%d", index)
		status := executePayment(t, handler, orderID)
		primeiraPassada[orderID] = status
		if status == http.StatusInternalServerError {
			falhas++
		}
	}
	// Com 25% configurados, a amostra precisa falhar de forma perceptível mas
	// jamais integralmente: o objetivo é ruído de fundo, não indisponibilidade.
	if falhas == 0 || falhas >= 40 {
		t.Fatalf("falhas = %d de 40, esperado ruído parcial", falhas)
	}

	for orderID, expected := range primeiraPassada {
		if repetido := executePayment(t, handler, orderID); repetido != expected {
			t.Fatalf("pedido %q devolveu %d na primeira passada e %d na segunda", orderID, expected, repetido)
		}
	}
}

// TestHandlerSemErroCrônicoNãoFalha confirma que o padrão continua saudável.
func TestHandlerSemErroCrônicoNãoFalha(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler(&repositoryStub{}, Scenario{})
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}
	for index := 0; index < 20; index++ {
		if status := executePayment(t, handler, fmt.Sprintf("pedido-%d", index)); status != http.StatusCreated {
			t.Fatalf("pedido %d devolveu %d, esperado 201", index, status)
		}
	}
}

// TestNewHandlerRejeitaErroCrônicoForaDoIntervalo mantém o cenário plausível.
func TestNewHandlerRejeitaErroCrônicoForaDoIntervalo(t *testing.T) {
	t.Parallel()

	for _, percent := range []int{-1, 101} {
		if _, err := NewHandler(&repositoryStub{}, Scenario{ChronicErrorPercent: percent}); err == nil {
			t.Fatalf("NewHandler() erro = nil para ChronicErrorPercent = %d", percent)
		}
	}
}

func executePayment(t *testing.T, handler *Handler, orderID string) int {
	t.Helper()

	body := fmt.Sprintf(`{"order_id":%q,"amount_cents":1990}`, orderID)
	request := httptest.NewRequest(http.MethodPost, "/payment", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Code
}
