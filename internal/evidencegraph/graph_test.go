package evidencegraph

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestBuildCriaEstruturaDoTraceComEvidencias protege o contrato mínimo consumido pelos relatórios.
func TestBuildCriaEstruturaDoTraceComEvidencias(t *testing.T) {
	t.Parallel()

	traceID := "trace-1"
	http := signal("evidence-http", traceID, "http-span", "checkout-service", time.Unix(10, 0), map[string]string{
		"span.name":                 "POST /checkout",
		"http.request.method":       "POST",
		"http.response.status_code": "500",
	})
	database := signal("evidence-database", traceID, "database-span", "checkout-service", time.Unix(11, 0), map[string]string{
		"span.name":      "INSERT orders",
		"span.parent_id": "http-span",
		"db.system.name": "postgresql",
	})

	graph, err := Build(traceID, []domain.Signal{database, http})
	if err != nil {
		t.Fatalf("Build() retornou erro: %v", err)
	}
	if graph.TraceID != traceID {
		t.Fatalf("TraceID = %q, esperava %q", graph.TraceID, traceID)
	}

	assertNode(t, graph, "service:checkout-service", NodeKindService, "checkout-service", []string{"evidence-database", "evidence-http"})
	assertNode(t, graph, "trace:trace-1", NodeKindTrace, "trace-1", []string{"evidence-database", "evidence-http"})
	assertNode(t, graph, "span:http-span", NodeKindSpan, "POST /checkout", []string{"evidence-http"})
	assertNode(t, graph, "span:database-span", NodeKindSpan, "INSERT orders", []string{"evidence-database"})

	assertEdge(t, graph, "service:checkout-service", "trace:trace-1", RelationContains, []string{"evidence-database", "evidence-http"})
	assertEdge(t, graph, "trace:trace-1", "span:http-span", RelationContains, []string{"evidence-http"})
	assertEdge(t, graph, "trace:trace-1", "span:database-span", RelationContains, []string{"evidence-database"})
	assertEdge(t, graph, "span:http-span", "span:database-span", RelationQueries, []string{"evidence-database", "evidence-http"})
}

// TestBuildUsaParentSpanIDParaNaoAssociarBancoAoHTTPIncorreto evita relações causais ambíguas.
func TestBuildUsaParentSpanIDParaNaoAssociarBancoAoHTTPIncorreto(t *testing.T) {
	t.Parallel()

	traceID := "trace-parent"
	signals := []domain.Signal{
		signal("http-a-evidence", traceID, "http-a", "checkout-service", time.Unix(10, 0), map[string]string{"http.request.method": "POST"}),
		signal("http-b-evidence", traceID, "http-b", "checkout-service", time.Unix(11, 0), map[string]string{"http.request.method": "POST"}),
		signal("database-evidence", traceID, "database", "checkout-service", time.Unix(12, 0), map[string]string{
			"span.parent_id": "http-b",
			"db.system.name": "postgresql",
		}),
	}

	graph, err := Build(traceID, signals)
	if err != nil {
		t.Fatalf("Build() retornou erro: %v", err)
	}

	assertEdge(t, graph, "span:http-b", "span:database", RelationQueries, []string{"database-evidence", "http-b-evidence"})
	assertNoEdge(t, graph, "span:http-a", "span:database", RelationQueries)
}

// TestBuildAssociaHTTPEDBSemParentDeFormaDeterministica cobre telemetria antiga sem parentesco normalizado.
func TestBuildAssociaHTTPEDBSemParentDeFormaDeterministica(t *testing.T) {
	t.Parallel()

	traceID := "trace-fallback"
	http := signal("http-evidence", traceID, "http", "checkout-service", time.Unix(10, 0), map[string]string{"http.request.method": "POST"})
	database := signal("database-evidence", traceID, "database", "checkout-service", time.Unix(11, 0), map[string]string{"db.system.name": "postgresql"})

	first, err := Build(traceID, []domain.Signal{database, http})
	if err != nil {
		t.Fatalf("Build() retornou erro: %v", err)
	}
	second, err := Build(traceID, []domain.Signal{http, database})
	if err != nil {
		t.Fatalf("Build() retornou erro ao inverter a entrada: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("grafo depende da ordem de entrada:\nprimeiro: %#v\nsegundo: %#v", first, second)
	}
	assertEdge(t, first, "span:http", "span:database", RelationQueries, []string{"database-evidence", "http-evidence"})
}

// TestBuildOrdenaNosArestasEEvidencias garante uma saída reproduzível para CLI e snapshots.
func TestBuildOrdenaNosArestasEEvidencias(t *testing.T) {
	t.Parallel()

	traceID := "trace-order"
	signals := []domain.Signal{
		signal("evidence-z", traceID, "span-z", "checkout-service", time.Unix(12, 0), map[string]string{"db.system.name": "postgresql"}),
		signal("evidence-a", traceID, "span-a", "checkout-service", time.Unix(10, 0), map[string]string{"http.request.method": "POST"}),
	}

	graph, err := Build(traceID, signals)
	if err != nil {
		t.Fatalf("Build() retornou erro: %v", err)
	}

	assertSorted(t, nodeIDs(graph.Nodes), "nós")
	assertSorted(t, edgeKeys(graph.Edges), "arestas")
	for _, node := range graph.Nodes {
		assertSorted(t, node.EvidenceIDs, "evidências do nó "+node.ID)
	}
	for _, edge := range graph.Edges {
		assertSorted(t, edge.EvidenceIDs, "evidências da aresta "+edge.From+"->"+edge.To)
	}
}

// TestBuildRejeitaTraceIDVazio impede grafos sem uma identidade consultável.
func TestBuildRejeitaTraceIDVazio(t *testing.T) {
	t.Parallel()

	_, err := Build("  ", nil)
	if err == nil {
		t.Fatal("Build() deveria rejeitar traceID vazio")
	}
	if !strings.Contains(err.Error(), "trace") {
		t.Fatalf("erro não explica o traceID inválido: %v", err)
	}
}

// TestBuildRejeitaSinaisDeOutroTrace evita misturar fluxos distribuídos no mesmo grafo.
func TestBuildRejeitaSinaisDeOutroTrace(t *testing.T) {
	t.Parallel()

	_, err := Build("trace-a", []domain.Signal{
		signal("evidence-a", "trace-a", "span-a", "checkout-service", time.Unix(10, 0), nil),
		signal("evidence-b", "trace-b", "span-b", "checkout-service", time.Unix(11, 0), nil),
	})
	if err == nil {
		t.Fatal("Build() deveria rejeitar sinais de trace_ids diferentes")
	}
	if !strings.Contains(err.Error(), "trace-b") {
		t.Fatalf("erro não identifica o trace divergente: %v", err)
	}
}

func signal(id, traceID, spanID, serviceName string, timestamp time.Time, attributes map[string]string) domain.Signal {
	return domain.Signal{
		ID:          id,
		Type:        domain.SignalTypeSpan,
		ServiceName: serviceName,
		Timestamp:   timestamp,
		TraceID:     traceID,
		SpanID:      spanID,
		Attributes:  attributes,
	}
}

func assertNode(t *testing.T, graph Graph, id string, kind NodeKind, label string, evidenceIDs []string) {
	t.Helper()
	for _, node := range graph.Nodes {
		if node.ID != id {
			continue
		}
		if node.Kind != kind || node.Label != label || !reflect.DeepEqual(node.EvidenceIDs, evidenceIDs) {
			t.Fatalf("nó %q = %#v, esperava kind=%q label=%q evidências=%#v", id, node, kind, label, evidenceIDs)
		}
		return
	}
	t.Fatalf("nó %q não encontrado em %#v", id, graph.Nodes)
}

func assertEdge(t *testing.T, graph Graph, from, to string, relation Relation, evidenceIDs []string) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.From != from || edge.To != to || edge.Relation != relation {
			continue
		}
		if !reflect.DeepEqual(edge.EvidenceIDs, evidenceIDs) {
			t.Fatalf("evidências da aresta %s->%s = %#v, esperava %#v", from, to, edge.EvidenceIDs, evidenceIDs)
		}
		return
	}
	t.Fatalf("aresta %s -%s-> %s não encontrada em %#v", from, relation, to, graph.Edges)
}

func assertNoEdge(t *testing.T, graph Graph, from, to string, relation Relation) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.From == from && edge.To == to && edge.Relation == relation {
			t.Fatalf("aresta inesperada encontrada: %#v", edge)
		}
	}
}

func assertSorted(t *testing.T, values []string, subject string) {
	t.Helper()
	for index := 1; index < len(values); index++ {
		if values[index-1] > values[index] {
			t.Fatalf("%s sem ordenação determinística: %#v", subject, values)
		}
	}
}

func nodeIDs(nodes []Node) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	return ids
}

func edgeKeys(edges []Edge) []string {
	keys := make([]string, 0, len(edges))
	for _, edge := range edges {
		keys = append(keys, edge.From+"\x00"+string(edge.Relation)+"\x00"+edge.To)
	}
	return keys
}
