package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/mcp"
)

// TestLinhaGiganteNaoDerrubaASessao é o caso que mais me preocupa no
// enquadramento por linha: existe um teto de tamanho, e estourá-lo não pode
// significar encerrar a sessão de quem está no meio de uma investigação.
func TestLinhaGiganteNaoDerrubaASessao(t *testing.T) {
	t.Parallel()

	gigante := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_incident","arguments":{"incident_id":"` +
		strings.Repeat("A", 4<<20) + `"}}}`

	respostas := conversar(t, historicoComUmIncidente(),
		gigante,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	)

	if len(respostas) != 2 {
		t.Fatalf("respostas = %d, esperado 2: um erro pela linha gigante e a resposta seguinte", len(respostas))
	}
	erro, existe := respostas[0]["error"].(map[string]any)
	if !existe {
		t.Fatalf("a linha gigante não recebeu erro JSON-RPC: %#v", respostas[0])
	}
	if mensagem, _ := erro["message"].(string); !strings.Contains(mensagem, "limite") {
		t.Fatalf("mensagem = %q, esperado explicar o limite", mensagem)
	}
	// O excedente precisa ter sido descartado até a quebra de linha: se sobrasse
	// no buffer, os pedaços virariam mensagens inválidas e a resposta seguinte
	// não seria a segunda.
	if _, existe := respostas[1]["result"]; !existe {
		t.Fatalf("o servidor não atendeu a requisição seguinte: %#v", respostas[1])
	}
}

// TestErroDoRepositorioViraErroDeToolENaoDerrubaOServidor cobre a falha de
// infraestrutura durante uma chamada: o SQLite pode estar corrompido ou o
// arquivo ter sumido, e a sessão precisa continuar.
func TestErroDoRepositorioViraErroDeToolENaoDerrubaOServidor(t *testing.T) {
	t.Parallel()

	quebrado := &historicoFake{erro: errors.New("banco indisponível")}
	respostas := conversar(t, quebrado,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_incidents","arguments":{"limit":5}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_incident","arguments":{"incident_id":"inc_x"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`,
	)
	if len(respostas) != 3 {
		t.Fatalf("respostas = %d, esperado 3", len(respostas))
	}
	for _, posicao := range []int{0, 1} {
		resultado, ok := respostas[posicao]["result"].(map[string]any)
		if !ok {
			t.Fatalf("resposta %d sem result: %#v", posicao+1, respostas[posicao])
		}
		if erro, _ := resultado["isError"].(bool); !erro {
			t.Fatalf("falha de infraestrutura não marcou isError: %#v", resultado)
		}
	}
	if _, existe := respostas[2]["result"]; !existe {
		t.Fatal("o servidor parou de atender depois de duas falhas do repositório")
	}
}

// TestIncidenteInexistenteExplicaOQueAconteceu garante que o modelo receba uma
// mensagem acionável, e não um erro genérico que o leve a inventar o motivo.
func TestIncidenteInexistenteExplicaOQueAconteceu(t *testing.T) {
	t.Parallel()

	ausente := &historicoFake{erro: application.ErrIncidentNotFound}
	respostas := conversar(t, ausente,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_incident","arguments":{"incident_id":"inc_nao_existe"}}}`,
	)
	resultado := respostas[0]["result"].(map[string]any)
	if erro, _ := resultado["isError"].(bool); !erro {
		t.Fatalf("incidente ausente não marcou isError: %#v", resultado)
	}
	texto := resultado["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(texto, "inc_nao_existe") {
		t.Fatalf("a mensagem não diz qual incidente faltou: %q", texto)
	}
}

// TestSuspeitoInexistenteNaoDerrubaNemMente cobre o erro de uso mais provável:
// o modelo chuta um nome de serviço que não está no ranking.
func TestSuspeitoInexistenteNaoDerrubaNemMente(t *testing.T) {
	t.Parallel()

	respostas := conversar(t, historicoComUmIncidente(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"explain_suspect","arguments":{"incident_id":"inc_abc123","suspect":"servico-que-nao-existe"}}}`,
	)
	resultado := respostas[0]["result"].(map[string]any)
	if erro, _ := resultado["isError"].(bool); !erro {
		t.Fatalf("suspeito inexistente deveria ser erro de tool: %#v", resultado)
	}
}

// TestArgumentosComTipoErradoViramErroDeTool cobre o modelo mandando string
// onde o schema pede inteiro — erro de uso, corrigível por ele mesmo.
func TestArgumentosComTipoErradoViramErroDeTool(t *testing.T) {
	t.Parallel()

	respostas := conversar(t, historicoComUmIncidente(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_incidents","arguments":{"limit":"cinco"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	)
	if len(respostas) != 2 {
		t.Fatalf("respostas = %d, esperado 2", len(respostas))
	}
	resultado := respostas[0]["result"].(map[string]any)
	if erro, _ := resultado["isError"].(bool); !erro {
		t.Fatalf("tipo errado deveria ser erro de tool: %#v", resultado)
	}
	if _, existe := respostas[1]["result"]; !existe {
		t.Fatal("o servidor parou depois de um argumento mal tipado")
	}
}

// TestSaidaQueFalhaEncerraComErroEmVezDeGirarEmSilencio cobre o cliente que
// fecha o pipe: continuar escrevendo em uma saída morta seria um laço mudo.
func TestSaidaQueFalhaEncerraComErroEmVezDeGirarEmSilencio(t *testing.T) {
	t.Parallel()

	entrada := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n",
	)
	err := mcp.Serve(context.Background(), mcp.Options{
		Input: entrada, Output: escritorQuebrado{}, Audit: &bytes.Buffer{},
		History: historicoComUmIncidente(),
	})
	if err == nil {
		t.Fatal("Serve() devolveu nil com a saída quebrada; a falha ficaria invisível")
	}
	if !strings.Contains(err.Error(), "escrever resposta") {
		t.Fatalf("erro = %v, esperado dizer que falhou ao escrever", err)
	}
}

type escritorQuebrado struct{}

func (escritorQuebrado) Write([]byte) (int, error) {
	return 0, errors.New("pipe fechado pelo cliente")
}

// TestContextoCanceladoEncerraASessao cobre o Ctrl+C: o processo precisa parar
// de atender, e não continuar consumindo a entrada.
func TestContextoCanceladoEncerraASessao(t *testing.T) {
	t.Parallel()

	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()

	var saida bytes.Buffer
	entrada := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n",
	)
	if err := mcp.Serve(ctx, mcp.Options{
		Input: entrada, Output: &saida, Audit: &bytes.Buffer{},
		History: historicoComUmIncidente(),
	}); err != nil {
		t.Fatalf("Serve() com contexto cancelado erro = %v", err)
	}
	if strings.TrimSpace(saida.String()) != "" {
		t.Fatalf("o servidor atendeu depois do cancelamento: %q", saida.String())
	}
}

// TestServeExigeSuasDependencias impede um servidor pela metade, que aceitaria
// conexão e falharia só na primeira chamada.
func TestServeExigeSuasDependencias(t *testing.T) {
	t.Parallel()

	for nome, options := range map[string]mcp.Options{
		"sem entrada":   {Output: &bytes.Buffer{}, History: historicoComUmIncidente()},
		"sem saída":     {Input: strings.NewReader(""), History: historicoComUmIncidente()},
		"sem histórico": {Input: strings.NewReader(""), Output: &bytes.Buffer{}},
	} {
		if err := mcp.Serve(context.Background(), options); err == nil {
			t.Fatalf("%s: Serve() aceitou opções incompletas", nome)
		}
	}
}

// TestLoteJSONRPCRecebeRespostaEmVezDeSilencio cobre um cliente que envia lote,
// forma prevista pelo JSON-RPC 2.0 e que este servidor não implementa. Não
// suportar é aceitável; engolir a mensagem e deixar o cliente esperando, não.
func TestLoteJSONRPCRecebeRespostaEmVezDeSilencio(t *testing.T) {
	t.Parallel()

	respostas := conversar(t, historicoComUmIncidente(),
		`[{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}]`,
	)
	if len(respostas) != 1 {
		t.Fatalf("respostas = %d, esperado 1 erro para lote não suportado", len(respostas))
	}
	if _, existe := respostas[0]["error"]; !existe {
		t.Fatalf("lote não recebeu erro JSON-RPC: %#v", respostas[0])
	}
}

// TestRespostaDeErroPreservaOIdDaRequisicao é o que permite ao cliente casar
// resposta com pergunta. Um erro com id trocado trava o cliente esperando.
func TestRespostaDeErroPreservaOIdDaRequisicao(t *testing.T) {
	t.Parallel()

	respostas := conversar(t, historicoComUmIncidente(),
		`{"jsonrpc":"2.0","id":"pedido-textual","method":"metodo/inexistente","params":{}}`,
	)
	if len(respostas) != 1 {
		t.Fatalf("respostas = %d, esperado 1", len(respostas))
	}
	identificador, err := json.Marshal(respostas[0]["id"])
	if err != nil {
		t.Fatalf("id não serializável: %v", err)
	}
	if string(identificador) != `"pedido-textual"` {
		t.Fatalf("id da resposta = %s, esperado \"pedido-textual\"", identificador)
	}
}
