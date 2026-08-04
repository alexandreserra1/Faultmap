package mermaid

import (
	"bytes"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/evidencegraph"
)

// TestRenderTraceGraphProduzMermaidDeterministico garante que o artefato não
// dependa da ordem recebida e preserve somente rótulos escapados.
func TestRenderTraceGraphProduzMermaidDeterministico(t *testing.T) {
	t.Parallel()

	graph := evidencegraph.Graph{
		TraceID: "trace-1",
		Nodes: []evidencegraph.Node{
			{ID: "span:http", Kind: evidencegraph.NodeKindSpan, Label: `POST /checkout "especial"`},
			{ID: "trace:trace-1", Kind: evidencegraph.NodeKindTrace, Label: "trace-1"},
			{ID: "service:checkout", Kind: evidencegraph.NodeKindService, Label: "checkout-service"},
			{ID: "span:db", Kind: evidencegraph.NodeKindSpan, Label: "INSERT orders"},
		},
		Edges: []evidencegraph.Edge{
			{From: "span:http", To: "span:db", Relation: evidencegraph.RelationQueries},
			{From: "trace:trace-1", To: "span:http", Relation: evidencegraph.RelationContains},
			{From: "service:checkout", To: "trace:trace-1", Relation: evidencegraph.RelationContains},
			{From: "trace:trace-1", To: "span:db", Relation: evidencegraph.RelationContains},
		},
	}

	var first bytes.Buffer
	if err := RenderTraceGraph(&first, graph); err != nil {
		t.Fatalf("RenderTraceGraph() erro = %v", err)
	}
	reversed := graph
	reversed.Nodes = reverseNodes(graph.Nodes)
	reversed.Edges = reverseEdges(graph.Edges)
	var second bytes.Buffer
	if err := RenderTraceGraph(&second, reversed); err != nil {
		t.Fatalf("RenderTraceGraph() invertido erro = %v", err)
	}
	if first.String() != second.String() {
		t.Fatalf("Mermaid depende da ordem:\n%s\n---\n%s", first.String(), second.String())
	}

	result := first.String()
	for _, expected := range []string{
		"flowchart TD",
		`["checkout-service"]`,
		`["INSERT orders"]`,
		`POST /checkout &quot;especial&quot;`,
		`|contém|`,
		`|consulta|`,
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("Mermaid não contém %q:\n%s", expected, result)
		}
	}
	if strings.Contains(result, `POST /checkout "especial"`) {
		t.Fatalf("rótulo não foi escapado:\n%s", result)
	}
}

// TestRenderTraceGraphRejeitaArestaSemNo evita gerar um diagrama silenciosamente corrompido.
func TestRenderTraceGraphRejeitaArestaSemNo(t *testing.T) {
	t.Parallel()

	graph := evidencegraph.Graph{
		Nodes: []evidencegraph.Node{{ID: "trace:trace-1", Label: "trace-1"}},
		Edges: []evidencegraph.Edge{{From: "trace:trace-1", To: "span:ausente", Relation: evidencegraph.RelationContains}},
	}
	if err := RenderTraceGraph(&bytes.Buffer{}, graph); err == nil {
		t.Fatal("RenderTraceGraph() erro = nil para aresta sem nó")
	}
}

func reverseNodes(values []evidencegraph.Node) []evidencegraph.Node {
	reversed := append([]evidencegraph.Node(nil), values...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed
}

func reverseEdges(values []evidencegraph.Edge) []evidencegraph.Edge {
	reversed := append([]evidencegraph.Edge(nil), values...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed
}
