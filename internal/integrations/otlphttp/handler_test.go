package otlphttp

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerDescompactaOTLPGzipAntesDaIngestao(t *testing.T) {
	t.Parallel()

	payload := []byte{0x0a, 0x00}
	var compressed bytes.Buffer
	compressor := gzip.NewWriter(&compressed)
	if _, err := compressor.Write(payload); err != nil {
		t.Fatalf("compactar payload do teste: %v", err)
	}
	if err := compressor.Close(); err != nil {
		t.Fatalf("finalizar payload compactado do teste: %v", err)
	}

	var receivedPayload []byte
	handler := newTestHandler(t, IngestFunc(func(_ context.Context, reader io.Reader, encoding Encoding) error {
		if encoding != EncodingProtobuf {
			t.Fatalf("esperava protobuf, recebeu %q", encoding)
		}
		var err error
		receivedPayload, err = io.ReadAll(reader)
		return err
	}), Options{})
	request := httptest.NewRequest(http.MethodPost, TracePath, bytes.NewReader(compressed.Bytes()))
	request.Header.Set("Content-Type", ContentTypeProtobuf)
	request.Header.Set("Content-Encoding", "gzip")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("esperava status 200, recebeu %d: %s", response.Code, response.Body.String())
	}
	if !bytes.Equal(receivedPayload, payload) {
		t.Fatalf("ingester recebeu bytes ainda compactados: recebeu %x, esperado %x", receivedPayload, payload)
	}
}

func TestHandlerValidaContentEncodingETamanhoDescompactado(t *testing.T) {
	t.Parallel()

	compact := func(t *testing.T, payload string) []byte {
		t.Helper()
		var compressed bytes.Buffer
		compressor := gzip.NewWriter(&compressed)
		if _, err := compressor.Write([]byte(payload)); err != nil {
			t.Fatalf("compactar payload do teste: %v", err)
		}
		if err := compressor.Close(); err != nil {
			t.Fatalf("finalizar payload compactado do teste: %v", err)
		}
		return compressed.Bytes()
	}

	tests := []struct {
		name            string
		contentEncoding string
		body            []byte
		maxBodyBytes    int64
		expectedStatus  int
		expectedMessage string
	}{
		{name: "codificação não suportada", contentEncoding: "br", body: []byte("payload"), maxBodyBytes: 64, expectedStatus: http.StatusUnsupportedMediaType, expectedMessage: "Content-Encoding OTLP não suportado"},
		{name: "gzip inválido", contentEncoding: "gzip", body: []byte("inválido"), maxBodyBytes: 64, expectedStatus: http.StatusBadRequest, expectedMessage: "payload OTLP inválido"},
		{name: "corpo descompactado excede limite", contentEncoding: "gzip", body: compact(t, "12345"), maxBodyBytes: 4, expectedStatus: http.StatusRequestEntityTooLarge, expectedMessage: "payload OTLP excede o limite"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			handler := newTestHandler(t, IngestFunc(func(context.Context, io.Reader, Encoding) error {
				calls++
				return nil
			}), Options{MaxRequestBodyBytes: test.maxBodyBytes})
			request := httptest.NewRequest(http.MethodPost, TracePath, bytes.NewReader(test.body))
			request.Header.Set("Content-Type", ContentTypeProtobuf)
			request.Header.Set("Content-Encoding", test.contentEncoding)
			request.ContentLength = -1
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.expectedStatus {
				t.Fatalf("esperava status %d, recebeu %d: %s", test.expectedStatus, response.Code, response.Body.String())
			}
			if calls != 0 {
				t.Fatalf("ingestão foi chamada %d vez(es) para corpo inválido", calls)
			}
			if !strings.Contains(response.Body.String(), test.expectedMessage) {
				t.Fatalf("resposta %q não contém %q", response.Body.String(), test.expectedMessage)
			}
		})
	}
}

func TestHandlerRecebeOTLPJSON(t *testing.T) {
	t.Parallel()

	type contextKey string
	const requestID contextKey = "request-id"
	payload := []byte(`{"resourceSpans":[]}`)
	var receivedEncoding Encoding
	var receivedPayload []byte
	var receivedContextValue any
	handler := newTestHandler(t, IngestFunc(func(ctx context.Context, reader io.Reader, encoding Encoding) error {
		receivedEncoding = encoding
		receivedPayload, _ = io.ReadAll(reader)
		receivedContextValue = ctx.Value(requestID)
		return nil
	}), Options{})

	request := httptest.NewRequest(http.MethodPost, TracePath, bytes.NewReader(payload))
	request.Header.Set("Content-Type", ContentTypeJSON+"; charset=utf-8")
	request = request.WithContext(context.WithValue(request.Context(), requestID, "req-1"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("esperava status 200, recebeu %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != ContentTypeJSON {
		t.Fatalf("esperava Content-Type %q, recebeu %q", ContentTypeJSON, response.Header().Get("Content-Type"))
	}
	if strings.TrimSpace(response.Body.String()) != "{}" {
		t.Fatalf("esperava ExportTraceServiceResponse JSON vazio, recebeu %q", response.Body.String())
	}
	if receivedEncoding != EncodingJSON {
		t.Fatalf("esperava encoding JSON, recebeu %q", receivedEncoding)
	}
	if !bytes.Equal(receivedPayload, payload) {
		t.Fatalf("payload recebido difere: %q", receivedPayload)
	}
	if receivedContextValue != "req-1" {
		t.Fatalf("contexto da requisição não foi propagado: %v", receivedContextValue)
	}
}

func TestHandlerRecebeOTLPProtobuf(t *testing.T) {
	t.Parallel()

	payload := []byte{0x0a, 0x00}
	var receivedEncoding Encoding
	var receivedPayload []byte
	handler := newTestHandler(t, IngestFunc(func(_ context.Context, reader io.Reader, encoding Encoding) error {
		receivedEncoding = encoding
		receivedPayload, _ = io.ReadAll(reader)
		return nil
	}), Options{})

	request := httptest.NewRequest(http.MethodPost, TracePath, bytes.NewReader(payload))
	request.Header.Set("Content-Type", ContentTypeProtobuf)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("esperava status 200, recebeu %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != ContentTypeProtobuf {
		t.Fatalf("esperava Content-Type %q, recebeu %q", ContentTypeProtobuf, response.Header().Get("Content-Type"))
	}
	if response.Body.Len() != 0 {
		t.Fatalf("ExportTraceServiceResponse protobuf vazio não deveria ter bytes: %x", response.Body.Bytes())
	}
	if receivedEncoding != EncodingProtobuf || !bytes.Equal(receivedPayload, payload) {
		t.Fatalf("requisição protobuf não foi encaminhada corretamente: encoding=%q payload=%x", receivedEncoding, receivedPayload)
	}
}

func TestHandlerRejeitaPayloadInvalidoComoGoogleStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		assertBody  func(*testing.T, []byte)
	}{
		{
			name:        "json",
			contentType: ContentTypeJSON,
			assertBody: func(t *testing.T, body []byte) {
				t.Helper()
				var status struct {
					Code    int32  `json:"code"`
					Message string `json:"message"`
				}
				if err := json.Unmarshal(body, &status); err != nil {
					t.Fatalf("resposta não é google.rpc.Status JSON: %v", err)
				}
				if status.Code != 3 || status.Message != "payload OTLP inválido" {
					t.Fatalf("status inesperado: %+v", status)
				}
			},
		},
		{
			name:        "protobuf",
			contentType: ContentTypeProtobuf,
			assertBody: func(t *testing.T, body []byte) {
				t.Helper()
				if !bytes.Equal(body, []byte{0x08, 0x03, 0x12, 0x16, 'p', 'a', 'y', 'l', 'o', 'a', 'd', ' ', 'O', 'T', 'L', 'P', ' ', 'i', 'n', 'v', 0xc3, 0xa1, 'l', 'i', 'd', 'o'}) {
					t.Fatalf("resposta não é google.rpc.Status protobuf esperado: %x", body)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handler := newTestHandler(t, IngestFunc(func(context.Context, io.Reader, Encoding) error {
				return errors.Join(ErrInvalidPayload, errors.New("detalhe interno que não deve vazar"))
			}), Options{})
			request := httptest.NewRequest(http.MethodPost, TracePath, strings.NewReader("inválido"))
			request.Header.Set("Content-Type", test.contentType)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("esperava status 400, recebeu %d", response.Code)
			}
			if response.Header().Get("Content-Type") != test.contentType {
				t.Fatalf("esperava Content-Type %q, recebeu %q", test.contentType, response.Header().Get("Content-Type"))
			}
			if strings.Contains(response.Body.String(), "detalhe interno") {
				t.Fatal("resposta expôs detalhes internos")
			}
			test.assertBody(t, response.Body.Bytes())
		})
	}
}

func TestHandlerValidaMetodoContentTypeETamanhoAntesDaIngestao(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		method          string
		contentType     string
		body            string
		contentLength   int64
		expectedStatus  int
		expectedMessage string
	}{
		{name: "método", method: http.MethodGet, contentType: ContentTypeJSON, expectedStatus: http.StatusMethodNotAllowed, expectedMessage: "método HTTP não permitido"},
		{name: "content type ausente", method: http.MethodPost, expectedStatus: http.StatusUnsupportedMediaType, expectedMessage: "Content-Type OTLP não suportado"},
		{name: "content type não suportado", method: http.MethodPost, contentType: "text/plain", expectedStatus: http.StatusUnsupportedMediaType, expectedMessage: "Content-Type OTLP não suportado"},
		{name: "content length excedido", method: http.MethodPost, contentType: ContentTypeJSON, body: "12345", contentLength: 5, expectedStatus: http.StatusRequestEntityTooLarge, expectedMessage: "payload OTLP excede o limite"},
		{name: "leitura excedida", method: http.MethodPost, contentType: ContentTypeJSON, body: "12345", contentLength: -1, expectedStatus: http.StatusRequestEntityTooLarge, expectedMessage: "payload OTLP excede o limite"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			handler := newTestHandler(t, IngestFunc(func(context.Context, io.Reader, Encoding) error {
				calls++
				return nil
			}), Options{MaxRequestBodyBytes: 4})
			request := httptest.NewRequest(test.method, TracePath, strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			if test.contentLength == -1 {
				request.ContentLength = -1
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.expectedStatus {
				t.Fatalf("esperava status %d, recebeu %d: %s", test.expectedStatus, response.Code, response.Body.String())
			}
			if calls != 0 {
				t.Fatalf("ingestão foi chamada %d vez(es) apesar da requisição inválida", calls)
			}
			if !strings.Contains(response.Body.String(), test.expectedMessage) {
				t.Fatalf("resposta %q não contém %q", response.Body.String(), test.expectedMessage)
			}
			if test.method != http.MethodPost && response.Header().Get("Allow") != http.MethodPost {
				t.Fatalf("esperava Allow POST, recebeu %q", response.Header().Get("Allow"))
			}
		})
	}
}

func TestHandlerAceitaExportTraceServiceRequestVazio(t *testing.T) {
	t.Parallel()

	for _, contentType := range []string{ContentTypeJSON, ContentTypeProtobuf} {
		contentType := contentType
		t.Run(contentType, func(t *testing.T) {
			t.Parallel()
			calls := 0
			handler := newTestHandler(t, IngestFunc(func(context.Context, io.Reader, Encoding) error {
				calls++
				return nil
			}), Options{})
			request := httptest.NewRequest(http.MethodPost, TracePath, nil)
			request.Header.Set("Content-Type", contentType)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("esperava status 200 para request OTLP vazio, recebeu %d: %s", response.Code, response.Body.String())
			}
			if calls != 0 {
				t.Fatalf("request vazio não deveria acionar persistência, recebeu %d chamada(s)", calls)
			}
		})
	}
}

func TestHandlerAceitaCorpoNoLimiteExato(t *testing.T) {
	t.Parallel()

	const payload = "1234"
	calls := 0
	handler := newTestHandler(t, IngestFunc(func(_ context.Context, reader io.Reader, _ Encoding) error {
		calls++
		received, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ler payload encaminhado: %v", err)
		}
		if string(received) != payload {
			t.Fatalf("payload encaminhado difere: %q", received)
		}
		return nil
	}), Options{MaxRequestBodyBytes: int64(len(payload))})
	request := httptest.NewRequest(http.MethodPost, TracePath, strings.NewReader(payload))
	request.Header.Set("Content-Type", ContentTypeProtobuf)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || calls != 1 {
		t.Fatalf("corpo no limite deveria ser aceito: status=%d chamadas=%d", response.Code, calls)
	}
}

func TestHandlerNaoExpoeErroInterno(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, IngestFunc(func(context.Context, io.Reader, Encoding) error {
		return errors.New("senha=segredo")
	}), Options{})
	request := httptest.NewRequest(http.MethodPost, TracePath, strings.NewReader(`{"resourceSpans":[]}`))
	request.Header.Set("Content-Type", ContentTypeJSON)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("esperava status 500, recebeu %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "segredo") || !strings.Contains(response.Body.String(), "falha interna ao ingerir traces") {
		t.Fatalf("resposta interna insegura: %q", response.Body.String())
	}
}

func TestHandlerRespeitaContextoCancelado(t *testing.T) {
	t.Parallel()

	called := false
	handler := newTestHandler(t, IngestFunc(func(context.Context, io.Reader, Encoding) error {
		called = true
		return nil
	}), Options{})
	request := httptest.NewRequest(http.MethodPost, TracePath, strings.NewReader(`{"resourceSpans":[]}`))
	request.Header.Set("Content-Type", ContentTypeJSON)
	ctx, cancel := context.WithCancel(request.Context())
	cancel()
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if called {
		t.Fatal("ingestão não deveria iniciar com contexto já cancelado")
	}
}

func TestHealthHandlerNaoDependeDaIngestao(t *testing.T) {
	t.Parallel()

	handler := NewHealthHandler()
	request := httptest.NewRequest(http.MethodGet, HealthPath, nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != ContentTypeJSON {
		t.Fatalf("health inesperado: status=%d content-type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	if strings.TrimSpace(response.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("health inesperado: %q", response.Body.String())
	}
}

func TestNewHandlerValidaDependenciasEOpcoes(t *testing.T) {
	t.Parallel()

	if _, err := NewHandler(nil, Options{}); err == nil {
		t.Fatal("esperava erro para ingester ausente")
	}
	if _, err := NewHandler(IngestFunc(func(context.Context, io.Reader, Encoding) error { return nil }), Options{MaxRequestBodyBytes: -1}); err == nil {
		t.Fatal("esperava erro para limite negativo")
	}
}

func newTestHandler(t *testing.T, ingester TraceIngester, options Options) http.Handler {
	t.Helper()
	handler, err := NewHandler(ingester, options)
	if err != nil {
		t.Fatalf("criar handler: %v", err)
	}
	return handler
}
