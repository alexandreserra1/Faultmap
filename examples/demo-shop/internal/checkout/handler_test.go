package checkout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHandlerEncaminhaPagamentoComSucesso(t *testing.T) {
	var chamadas atomic.Int32
	payment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamadas.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/payment" {
			t.Fatalf("requisição inesperada: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer payment.Close()

	handler, err := NewHandler(Config{
		PaymentURL:      payment.URL + "/payment",
		PaymentTimeout:  time.Second,
		PaymentAttempts: 2,
	}, payment.Client())
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/checkout", strings.NewReader(`{"order_id":"order-1","amount_cents":1990}`))
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, esperado %d; corpo=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	if chamadas.Load() != 1 {
		t.Fatalf("chamadas payment = %d, esperado 1", chamadas.Load())
	}
}

func TestHandlerRepetePagamentoAteLimite(t *testing.T) {
	var chamadas atomic.Int32
	payment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if chamadas.Add(1) < 3 {
			http.Error(w, "temporário", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer payment.Close()

	handler, err := NewHandler(Config{PaymentURL: payment.URL, PaymentTimeout: time.Second, PaymentAttempts: 3}, payment.Client())
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/checkout", strings.NewReader(`{"order_id":"order-2","amount_cents":2500}`)))

	if recorder.Code != http.StatusCreated || chamadas.Load() != 3 {
		t.Fatalf("status=%d chamadas=%d", recorder.Code, chamadas.Load())
	}
}

func TestHandlerRespeitaCancelamentoDoContexto(t *testing.T) {
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	handler, err := NewHandler(Config{PaymentURL: "http://payment/payment", PaymentTimeout: time.Minute, PaymentAttempts: 3}, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/checkout", strings.NewReader(`{"order_id":"order-3","amount_cents":3000}`)).WithContext(ctx)
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, esperado %d", recorder.Code, http.StatusGatewayTimeout)
	}
}

func TestHandlerValidaEntradaEMetodo(t *testing.T) {
	handler, err := NewHandler(Config{PaymentURL: "http://payment/payment", PaymentTimeout: time.Second, PaymentAttempts: 1}, &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("não deveria chamar")
	})})
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}

	for _, test := range []struct {
		name   string
		method string
		body   string
		status int
	}{
		{name: "método", method: http.MethodGet, body: "", status: http.StatusMethodNotAllowed},
		{name: "json inválido", method: http.MethodPost, body: "{", status: http.StatusBadRequest},
		{name: "campos inválidos", method: http.MethodPost, body: `{}`, status: http.StatusBadRequest},
		{name: "dados adicionais", method: http.MethodPost, body: `{"order_id":"order-4","amount_cents":3000} {}`, status: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(test.method, "/checkout", strings.NewReader(test.body)))
			if recorder.Code != test.status {
				t.Fatalf("status = %d, esperado %d", recorder.Code, test.status)
			}
		})
	}
}

func TestNewHandlerRejeitaURLRelativa(t *testing.T) {
	t.Parallel()

	_, err := NewHandler(Config{PaymentURL: "/payment", PaymentTimeout: time.Second, PaymentAttempts: 1}, http.DefaultClient)
	if err == nil {
		t.Fatal("NewHandler() deveria exigir URL HTTP absoluta")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

// TestHandlerFanOutFazChamadasParalelasBemSucedidas cobre o fan-out legítimo:
// várias chamadas ao pagamento no mesmo trace sem que nenhuma seja um retry.
// É o tráfego que um detector de retry storm pode confundir com repetição
// anormal, e por isso precisa existir na demo.
func TestHandlerFanOutFazChamadasParalelasBemSucedidas(t *testing.T) {
	t.Parallel()

	var chamadas atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		chamadas.Add(1)
		writer.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	handler, err := NewHandler(Config{
		PaymentURL:      server.URL,
		PaymentTimeout:  2 * time.Second,
		PaymentAttempts: 1,
		PaymentFanOut:   4,
	}, server.Client())
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost, "/checkout", strings.NewReader(`{"order_id":"fan-1","amount_cents":1990}`),
	))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, esperado 201; corpo = %s", recorder.Code, recorder.Body.String())
	}
	if total := chamadas.Load(); total != 4 {
		t.Fatalf("chamadas ao pagamento = %d, esperado 4", total)
	}
}

// TestHandlerFanOutFalhaQuandoUmaChamadaFalha mantém o checkout honesto: se um
// ramo do fan-out não conclui, a compra não pode ser reportada como criada.
func TestHandlerFanOutFalhaQuandoUmaChamadaFalha(t *testing.T) {
	t.Parallel()

	var chamadas atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if chamadas.Add(1) == 2 {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		writer.WriteHeader(http.StatusCreated)
	}))
	t.Cleanup(server.Close)

	handler, err := NewHandler(Config{
		PaymentURL:      server.URL,
		PaymentTimeout:  2 * time.Second,
		PaymentAttempts: 1,
		PaymentFanOut:   3,
	}, server.Client())
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost, "/checkout", strings.NewReader(`{"order_id":"fan-2","amount_cents":1990}`),
	))
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, esperado 502", recorder.Code)
	}
}

// TestNewHandlerRejeitaFanOutForaDoIntervalo evita fan-out ilimitado.
func TestNewHandlerRejeitaFanOutForaDoIntervalo(t *testing.T) {
	t.Parallel()

	for _, fanOut := range []int{-1, 11} {
		_, err := NewHandler(Config{
			PaymentURL:      "http://payment.local/payment",
			PaymentTimeout:  time.Second,
			PaymentAttempts: 1,
			PaymentFanOut:   fanOut,
		}, http.DefaultClient)
		if err == nil {
			t.Fatalf("NewHandler() erro = nil para PaymentFanOut = %d", fanOut)
		}
	}
}
