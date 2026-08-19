package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGatewayPropagaContextoDeTrace cobre a única razão de este proxy existir:
// sem o cabeçalho de contexto chegando ao serviço de trás, os dois lados
// produzem traces separados e não há grafo para o ranking comparar.
func TestGatewayPropagaContextoDeTrace(t *testing.T) {
	t.Parallel()

	recebido := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		recebido <- request.Header.Get("traceparent")
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)

	proxy := novoProxy(t, backend.URL)
	requisição := httptest.NewRequest(http.MethodGet, "/api/v1/injuries", nil)
	// Um traceparent válido chegando de fora simula o que o cliente ou um
	// serviço anterior teria enviado.
	requisição.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	gravador := httptest.NewRecorder()
	proxy.ServeHTTP(gravador, requisição)

	select {
	case cabeçalho := <-recebido:
		if cabeçalho == "" {
			t.Fatal("o serviço de trás não recebeu traceparent; os traces ficariam desconectados")
		}
		if !strings.Contains(cabeçalho, "4bf92f3577b34da6a3ce929d0e0e4736") {
			t.Fatalf("o trace de origem não foi preservado: %s", cabeçalho)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("o serviço de trás não foi chamado")
	}
}

// TestGatewayRepassaCorpoEStatus garante que o proxy não altera a resposta: ele
// existe para haver um segundo serviço no trace, não para transformar nada.
func TestGatewayRepassaCorpoEStatus(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusTeapot)
		if _, err := writer.Write([]byte(`{"lesoes":[]}`)); err != nil {
			t.Errorf("escrever corpo: %v", err)
		}
	}))
	t.Cleanup(backend.Close)

	gravador := httptest.NewRecorder()
	novoProxy(t, backend.URL).ServeHTTP(gravador, httptest.NewRequest(http.MethodGet, "/api/v1/injuries", nil))

	if gravador.Code != http.StatusTeapot {
		t.Fatalf("status = %d, esperado 418 repassado do serviço de trás", gravador.Code)
	}
	corpo, err := io.ReadAll(gravador.Body)
	if err != nil {
		t.Fatalf("ler corpo: %v", err)
	}
	if string(corpo) != `{"lesoes":[]}` {
		t.Fatalf("corpo alterado pelo proxy: %s", corpo)
	}
}

// TestGatewayRepassaAutorização confirma que o cabeçalho de credencial chega ao
// serviço de trás. Sem ele a rota do piloto responde 401 e o incidente mediria
// autenticação, não o que foi injetado.
func TestGatewayRepassaAutorização(t *testing.T) {
	t.Parallel()

	recebido := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		recebido <- request.Header.Get("Authorization")
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)

	requisição := httptest.NewRequest(http.MethodGet, "/api/v1/injuries", nil)
	requisição.Header.Set("Authorization", "Bearer token-do-piloto")
	novoProxy(t, backend.URL).ServeHTTP(httptest.NewRecorder(), requisição)

	select {
	case cabeçalho := <-recebido:
		if cabeçalho != "Bearer token-do-piloto" {
			t.Fatalf("Authorization = %q", cabeçalho)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("o serviço de trás não foi chamado")
	}
}

// TestNewHandlerRejeitaDestinoInválido evita que o processo suba apontando para
// lugar nenhum e só descubra durante o incidente.
func TestNewHandlerRejeitaDestinoInválido(t *testing.T) {
	t.Parallel()

	for _, destino := range []string{"", "   ", "://sem-esquema"} {
		if _, err := NewHandler(destino); err == nil {
			t.Fatalf("destino %q foi aceito", destino)
		}
	}
}

func novoProxy(t *testing.T, destino string) http.Handler {
	t.Helper()
	if _, err := url.Parse(destino); err != nil {
		t.Fatalf("destino inválido no teste: %v", err)
	}
	handler, err := NewHandler(destino)
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}
	return handler
}

// TestAtrasoDeArquivoLigaEDesligaSemReiniciar cobre o cenário em que o culpado
// é o próprio proxy. Ler a falha de um arquivo a cada requisição evita
// reiniciar o processo entre a baseline e o incidente, o que abriria um
// intervalo morto entre as duas janelas comparadas.
func TestAtrasoDeArquivoLigaEDesligaSemReiniciar(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)

	arquivo := filepath.Join(t.TempDir(), "atraso.txt")
	proxy, err := NewHandler(backend.URL)
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}
	comAtraso := WithDelayFile(proxy, arquivo)

	medir := func() time.Duration {
		início := time.Now()
		comAtraso.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/injuries", nil))
		return time.Since(início)
	}

	if limpo := medir(); limpo > 100*time.Millisecond {
		t.Fatalf("sem arquivo houve atraso de %v", limpo)
	}
	if err := os.WriteFile(arquivo, []byte("250"), 0o600); err != nil {
		t.Fatalf("escrever arquivo de atraso: %v", err)
	}
	if atrasado := medir(); atrasado < 200*time.Millisecond {
		t.Fatalf("com arquivo o atraso foi de apenas %v", atrasado)
	}
	if err := os.Remove(arquivo); err != nil {
		t.Fatalf("remover arquivo de atraso: %v", err)
	}
	if limpo := medir(); limpo > 100*time.Millisecond {
		t.Fatalf("após remover o arquivo o atraso persistiu: %v", limpo)
	}
}

// TestAtrasoIgnoraConteúdoInválido impede que um erro de escrita no arquivo
// derrube o proxy no meio de um incidente.
func TestAtrasoIgnoraConteúdoInválido(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)

	arquivo := filepath.Join(t.TempDir(), "atraso.txt")
	if err := os.WriteFile(arquivo, []byte("bobagem"), 0o600); err != nil {
		t.Fatalf("escrever arquivo: %v", err)
	}
	proxy, err := NewHandler(backend.URL)
	if err != nil {
		t.Fatalf("NewHandler() erro = %v", err)
	}
	gravador := httptest.NewRecorder()
	WithDelayFile(proxy, arquivo).ServeHTTP(gravador, httptest.NewRequest(http.MethodGet, "/api/v1/injuries", nil))
	if gravador.Code != http.StatusOK {
		t.Fatalf("status = %d; conteúdo inválido não pode derrubar o proxy", gravador.Code)
	}
}
