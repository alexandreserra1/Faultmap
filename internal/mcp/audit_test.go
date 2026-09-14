package mcp_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/mcp"
)

// TestAuditoriaNaoEscreveNaSaidaDoProtocolo é a regra que, quebrada, corrompe
// toda sessão: stdout é o transporte JSON-RPC. Uma linha de log ali vira uma
// mensagem malformada para o cliente.
func TestAuditoriaNaoEscreveNaSaidaDoProtocolo(t *testing.T) {
	t.Parallel()

	var saida, auditoria bytes.Buffer
	entrada := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_incidents","arguments":{"limit":5}}}` + "\n",
	)
	if err := mcp.Serve(context.Background(), mcp.Options{
		Input: entrada, Output: &saida, Audit: &auditoria, History: historicoComUmIncidente(),
	}); err != nil {
		t.Fatalf("Serve() erro = %v", err)
	}

	for _, linha := range strings.Split(strings.TrimSpace(saida.String()), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(linha), "{") {
			t.Fatalf("saída do protocolo contém linha que não é JSON-RPC: %q", linha)
		}
	}
	if !strings.Contains(auditoria.String(), "list_incidents") {
		t.Fatalf("a auditoria não registrou a chamada: %q", auditoria.String())
	}
}

// TestAuditoriaNaoRegistraOsArgumentos evita que a trilha vire um segundo
// repositório de identificadores de incidente.
func TestAuditoriaNaoRegistraOsArgumentos(t *testing.T) {
	t.Parallel()

	var saida, auditoria bytes.Buffer
	entrada := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_incident","arguments":{"incident_id":"inc_abc123"}}}` + "\n",
	)
	if err := mcp.Serve(context.Background(), mcp.Options{
		Input: entrada, Output: &saida, Audit: &auditoria, History: historicoComUmIncidente(),
	}); err != nil {
		t.Fatalf("Serve() erro = %v", err)
	}

	if strings.Contains(auditoria.String(), "inc_abc123") {
		t.Fatalf("a auditoria registrou o argumento da chamada: %q", auditoria.String())
	}
	if !strings.Contains(auditoria.String(), "get_incident") {
		t.Fatalf("a auditoria não registrou o nome da tool: %q", auditoria.String())
	}
}
