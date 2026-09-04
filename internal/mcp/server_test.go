package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/mcp"
	"github.com/faultmap/faultmap/internal/ranking"
)

var incidenteEm = time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)

type historicoFake struct {
	incidentes  []application.IncidentSummary
	diagnostico application.PersistedDiagnosis
	erro        error
}

func (fake *historicoFake) List(_ context.Context, _ int) ([]application.IncidentSummary, error) {
	return fake.incidentes, fake.erro
}

func (fake *historicoFake) Get(_ context.Context, _ string) (application.PersistedDiagnosis, error) {
	if fake.erro != nil {
		return application.PersistedDiagnosis{}, fake.erro
	}
	return fake.diagnostico, nil
}

func historicoComUmIncidente() *historicoFake {
	resumo := application.IncidentSummary{
		ID: "inc_abc123", ServiceName: "payment-service", Environment: "staging",
		Status: "closed", IncidentStart: incidenteEm, IncidentEnd: incidenteEm.Add(time.Minute),
	}
	return &historicoFake{
		incidentes: []application.IncidentSummary{resumo},
		diagnostico: application.PersistedDiagnosis{
			Incident: resumo,
			Findings: []detection.Finding{{
				Rule: detection.RuleSchemaChangeProximity, ServiceName: "payment-service",
				Score: 0.75, Confidence: detection.ConfidenceHigh,
				Evidence:    []detection.Evidence{{Summary: "O objeto índice idx_payments_created_at da base payments foi removido."}},
				Limitations: []string{"Proximidade temporal não prova causalidade."},
			}},
			Suspects: []ranking.Suspect{{
				Kind: detection.SubjectService, ID: "payment-service", Label: "payment-service",
				Score: 0.22, Confidence: detection.ConfidenceHigh,
				Contributions: []ranking.ScoreContribution{{
					RuleID: detection.RuleSchemaChangeProximity, Value: 0.22, Reason: "score 0.75 × peso 0.30 = 0.22",
				}},
			}},
		},
	}
}

// conversar roda uma sessão completa contra o servidor e devolve uma resposta
// por requisição enviada, ignorando notificações, que não têm resposta.
func conversar(t *testing.T, historico application.IncidentHistoryReader, requisicoes ...string) []map[string]any {
	t.Helper()

	entrada := strings.NewReader(strings.Join(requisicoes, "\n") + "\n")
	var saida, auditoria bytes.Buffer
	if err := mcp.Serve(context.Background(), mcp.Options{
		Input: entrada, Output: &saida, Audit: &auditoria, History: historico,
	}); err != nil {
		t.Fatalf("Serve() erro = %v", err)
	}

	respostas := make([]map[string]any, 0, len(requisicoes))
	for _, linha := range strings.Split(strings.TrimSpace(saida.String()), "\n") {
		if strings.TrimSpace(linha) == "" {
			continue
		}
		var resposta map[string]any
		if err := json.Unmarshal([]byte(linha), &resposta); err != nil {
			t.Fatalf("resposta não é JSON válido: %q: %v", linha, err)
		}
		if resposta["jsonrpc"] != "2.0" {
			t.Fatalf("resposta sem jsonrpc 2.0: %q", linha)
		}
		respostas = append(respostas, resposta)
	}
	return respostas
}

func conteudoDaTool(t *testing.T, resposta map[string]any) string {
	t.Helper()

	resultado, ok := resposta["result"].(map[string]any)
	if !ok {
		t.Fatalf("resposta sem result: %#v", resposta)
	}
	conteudo, ok := resultado["content"].([]any)
	if !ok || len(conteudo) == 0 {
		t.Fatalf("result sem content: %#v", resultado)
	}
	bloco, ok := conteudo[0].(map[string]any)
	if !ok {
		t.Fatalf("bloco de conteúdo inesperado: %#v", conteudo[0])
	}
	texto, ok := bloco["text"].(string)
	if !ok {
		t.Fatalf("bloco sem texto: %#v", bloco)
	}
	return texto
}

// TestInitializeDeclaraProtocoloECapacidades cobre o aperto de mão sem o qual
// nenhum cliente MCP prossegue.
func TestInitializeDeclaraProtocoloECapacidades(t *testing.T) {
	t.Parallel()

	respostas := conversar(t, historicoComUmIncidente(),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
	)
	if len(respostas) != 1 {
		t.Fatalf("respostas = %d, esperado 1", len(respostas))
	}
	resultado, ok := respostas[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("initialize sem result: %#v", respostas[0])
	}
	if _, existe := resultado["protocolVersion"]; !existe {
		t.Fatalf("initialize não declarou protocolVersion: %#v", resultado)
	}
	capacidades, ok := resultado["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("initialize sem capabilities: %#v", resultado)
	}
	if _, existe := capacidades["tools"]; !existe {
		t.Fatalf("o servidor não declarou a capacidade de tools: %#v", capacidades)
	}
}

// TestNotificacaoNaoRecebeResposta protege o protocolo: responder a uma
// notificação quebra clientes que não esperam nada de volta.
func TestNotificacaoNaoRecebeResposta(t *testing.T) {
	t.Parallel()

	respostas := conversar(t, historicoComUmIncidente(),
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	)
	if len(respostas) != 0 {
		t.Fatalf("respostas = %#v, esperado nenhuma para notificação", respostas)
	}
}

// TestToolsListExpoeAsTresFerramentasDeLeitura confirma o contrato que o
// cliente usa para descobrir o que o Faultmap oferece.
func TestToolsListExpoeAsTresFerramentasDeLeitura(t *testing.T) {
	t.Parallel()

	respostas := conversar(t, historicoComUmIncidente(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
	)
	resultado := respostas[0]["result"].(map[string]any)
	tools, ok := resultado["tools"].([]any)
	if !ok {
		t.Fatalf("tools/list sem lista: %#v", resultado)
	}

	nomes := make(map[string]map[string]any, len(tools))
	for _, item := range tools {
		tool := item.(map[string]any)
		nomes[tool["name"].(string)] = tool
	}
	for _, esperada := range []string{"list_incidents", "get_incident", "explain_suspect"} {
		tool, existe := nomes[esperada]
		if !existe {
			t.Fatalf("tool %q ausente: %#v", esperada, nomes)
		}
		if descricao, _ := tool["description"].(string); strings.TrimSpace(descricao) == "" {
			t.Fatalf("tool %q sem descrição: um cliente não saberia quando usá-la", esperada)
		}
		if _, existe := tool["inputSchema"]; !existe {
			t.Fatalf("tool %q sem inputSchema", esperada)
		}
	}
	if len(nomes) != 3 {
		t.Fatalf("tools = %d, esperado exatamente as três de leitura: %#v", len(nomes), nomes)
	}
}

// TestNaoExisteToolQueDispareInvestigacao é a posição do produto virada teste: o
// LLM consome e explica resultados estruturados, não participa do motor. Uma
// tool que rodasse `diagnose` colocaria o modelo dentro da análise determinística.
func TestNaoExisteToolQueDispareInvestigacao(t *testing.T) {
	t.Parallel()

	respostas := conversar(t, historicoComUmIncidente(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
	)
	resultado := respostas[0]["result"].(map[string]any)
	for _, item := range resultado["tools"].([]any) {
		nome := item.(map[string]any)["name"].(string)
		for _, proibido := range []string{"diagnose", "ingest", "serve", "retention", "delete"} {
			if strings.Contains(nome, proibido) {
				t.Fatalf("tool %q escreve ou dispara investigação; o servidor é somente leitura", nome)
			}
		}
	}
}

// TestToolsCallDevolveOsDadosDoSnapshot exercita as três ferramentas contra um
// diagnóstico persistido.
func TestToolsCallDevolveOsDadosDoSnapshot(t *testing.T) {
	t.Parallel()

	respostas := conversar(t, historicoComUmIncidente(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_incidents","arguments":{"limit":5}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_incident","arguments":{"incident_id":"inc_abc123"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"explain_suspect","arguments":{"incident_id":"inc_abc123","suspect":"payment-service"}}}`,
	)
	if len(respostas) != 3 {
		t.Fatalf("respostas = %d, esperado 3", len(respostas))
	}
	for posicao, esperado := range []string{"inc_abc123", "payment-service", "schema_change_proximity"} {
		texto := conteudoDaTool(t, respostas[posicao])
		if !strings.Contains(texto, esperado) {
			t.Fatalf("resposta %d não contém %q: %s", posicao+1, esperado, texto)
		}
		if !json.Valid([]byte(texto)) {
			t.Fatalf("resposta %d não é JSON válido: %s", posicao+1, texto)
		}
	}
}

// TestToolsCallSemArgumentoObrigatorioDevolveErroDeTool distingue erro de uso —
// que o modelo pode corrigir sozinho — de falha de protocolo.
func TestToolsCallSemArgumentoObrigatorioDevolveErroDeTool(t *testing.T) {
	t.Parallel()

	respostas := conversar(t, historicoComUmIncidente(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_incident","arguments":{}}}`,
	)
	resultado, ok := respostas[0]["result"].(map[string]any)
	if !ok {
		t.Fatalf("esperado result com isError, veio %#v", respostas[0])
	}
	if erro, _ := resultado["isError"].(bool); !erro {
		t.Fatalf("resultado não marcou isError: %#v", resultado)
	}
}

// TestClienteMalComportadoNaoDerrubaOServidor é o teste que mais protege uma
// sessão real: um cliente que envia lixo precisa receber erro e continuar sendo
// atendido, não encerrar o servidor.
func TestClienteMalComportadoNaoDerrubaOServidor(t *testing.T) {
	t.Parallel()

	respostas := conversar(t, historicoComUmIncidente(),
		`{isso não é json`,
		`{"jsonrpc":"2.0","id":2,"method":"metodo/inexistente","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"tool_que_nao_existe","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/list","params":{}}`,
	)
	if len(respostas) != 4 {
		t.Fatalf("respostas = %d, esperado 4: o servidor parou antes do fim", len(respostas))
	}
	for _, posicao := range []int{0, 1} {
		if _, existe := respostas[posicao]["error"]; !existe {
			t.Fatalf("resposta %d deveria ser erro JSON-RPC: %#v", posicao+1, respostas[posicao])
		}
	}
	// A última precisa ter sido atendida normalmente, provando que o servidor
	// seguiu de pé depois de três entradas ruins seguidas.
	if _, existe := respostas[3]["result"]; !existe {
		t.Fatalf("o servidor não voltou a atender depois dos erros: %#v", respostas[3])
	}
}

// TestRespostasSaoDeterministicas repete a promessa do resto do produto: mesma
// entrada, mesma saída, byte a byte.
func TestRespostasSaoDeterministicas(t *testing.T) {
	t.Parallel()

	requisicao := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_incident","arguments":{"incident_id":"inc_abc123"}}}`
	primeira := conteudoDaTool(t, conversar(t, historicoComUmIncidente(), requisicao)[0])
	segunda := conteudoDaTool(t, conversar(t, historicoComUmIncidente(), requisicao)[0])

	if primeira != segunda {
		t.Fatalf("duas execuções divergiram:\n%s\n%s", primeira, segunda)
	}
}
