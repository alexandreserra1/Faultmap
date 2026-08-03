// Package evidencegraph constrói relações auditáveis entre sinais de um trace sem acessar infraestrutura.
package evidencegraph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// NodeKind identifica a função de um nó no grafo de evidências.
type NodeKind string

const (
	// NodeKindService representa o serviço que emitiu os sinais.
	NodeKindService NodeKind = "service"
	// NodeKindTrace representa o fluxo distribuído investigado.
	NodeKindTrace NodeKind = "trace"
	// NodeKindSpan representa uma operação individual observada.
	NodeKindSpan NodeKind = "span"
)

// Relation descreve a ligação determinística entre dois nós.
type Relation string

const (
	// RelationContains liga serviços a traces e traces a spans.
	RelationContains Relation = "contains"
	// RelationQueries liga um span chamador a uma operação de banco filha.
	RelationQueries Relation = "queries"
)

// Node preserva a identidade, o rótulo e os sinais que sustentam um elemento do grafo.
type Node struct {
	ID          string
	Kind        NodeKind
	Label       string
	EvidenceIDs []string
}

// Edge preserva a relação e os sinais que sustentam a ligação entre dois nós.
type Edge struct {
	From        string
	To          string
	Relation    Relation
	EvidenceIDs []string
}

// Graph reúne os nós e relações auditáveis de um único trace.
type Graph struct {
	TraceID string
	Nodes   []Node
	Edges   []Edge
}

// Build constrói um grafo determinístico e rejeita sinais que não pertençam ao trace solicitado.
func Build(traceID string, signals []domain.Signal) (Graph, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return Graph{}, fmt.Errorf("construir grafo: trace ID é obrigatório")
	}

	ordered := append([]domain.Signal(nil), signals...)
	sort.Slice(ordered, func(first, second int) bool {
		if !ordered[first].Timestamp.Equal(ordered[second].Timestamp) {
			return ordered[first].Timestamp.Before(ordered[second].Timestamp)
		}
		return ordered[first].ID < ordered[second].ID
	})
	for _, signal := range ordered {
		if signal.TraceID != traceID {
			return Graph{}, fmt.Errorf("construir grafo do trace %q: sinal %q pertence ao trace %q", traceID, signal.ID, signal.TraceID)
		}
	}

	nodes := make(map[string]*nodeAccumulator)
	edges := make(map[string]*edgeAccumulator)
	allEvidence := signalIDs(ordered)
	addNode(nodes, "trace:"+traceID, NodeKindTrace, traceID, allEvidence)

	bySpanID := make(map[string]domain.Signal)
	httpSignals := make([]domain.Signal, 0)
	databaseSignals := make([]domain.Signal, 0)
	serviceEvidence := make(map[string][]string)
	for _, signal := range ordered {
		serviceName := strings.TrimSpace(signal.ServiceName)
		if serviceName != "" {
			serviceEvidence[serviceName] = append(serviceEvidence[serviceName], signal.ID)
		}
		spanID := strings.TrimSpace(signal.SpanID)
		if spanID == "" {
			continue
		}
		bySpanID[spanID] = signal
		spanNodeID := "span:" + spanID
		addNode(nodes, spanNodeID, NodeKindSpan, spanLabel(signal), []string{signal.ID})
		addEdge(edges, "trace:"+traceID, spanNodeID, RelationContains, []string{signal.ID})
		if isHTTPSignal(signal) {
			httpSignals = append(httpSignals, signal)
		}
		if isDatabaseSignal(signal) {
			databaseSignals = append(databaseSignals, signal)
		}
	}
	for serviceName, evidenceIDs := range serviceEvidence {
		serviceNodeID := "service:" + serviceName
		addNode(nodes, serviceNodeID, NodeKindService, serviceName, evidenceIDs)
		addEdge(edges, serviceNodeID, "trace:"+traceID, RelationContains, evidenceIDs)
	}

	for _, databaseSignal := range databaseSignals {
		parentSpanID := strings.TrimSpace(databaseSignal.Attributes["span.parent_id"])
		if parentSignal, exists := bySpanID[parentSpanID]; parentSpanID != "" && exists && isHTTPSignal(parentSignal) {
			addQueryEdge(edges, parentSignal, databaseSignal)
		}
	}
	if len(httpSignals) == 1 && len(databaseSignals) == 1 && strings.TrimSpace(databaseSignals[0].Attributes["span.parent_id"]) == "" {
		addQueryEdge(edges, httpSignals[0], databaseSignals[0])
	}

	return Graph{TraceID: traceID, Nodes: materializeNodes(nodes), Edges: materializeEdges(edges)}, nil
}

type nodeAccumulator struct {
	node     Node
	evidence map[string]struct{}
}

type edgeAccumulator struct {
	edge     Edge
	evidence map[string]struct{}
}

func addNode(nodes map[string]*nodeAccumulator, id string, kind NodeKind, label string, evidenceIDs []string) {
	accumulator := nodes[id]
	if accumulator == nil {
		accumulator = &nodeAccumulator{node: Node{ID: id, Kind: kind, Label: label}, evidence: make(map[string]struct{})}
		nodes[id] = accumulator
	}
	for _, evidenceID := range evidenceIDs {
		accumulator.evidence[evidenceID] = struct{}{}
	}
}

func addEdge(edges map[string]*edgeAccumulator, from, to string, relation Relation, evidenceIDs []string) {
	key := from + "\x00" + string(relation) + "\x00" + to
	accumulator := edges[key]
	if accumulator == nil {
		accumulator = &edgeAccumulator{edge: Edge{From: from, To: to, Relation: relation}, evidence: make(map[string]struct{})}
		edges[key] = accumulator
	}
	for _, evidenceID := range evidenceIDs {
		accumulator.evidence[evidenceID] = struct{}{}
	}
}

func addQueryEdge(edges map[string]*edgeAccumulator, httpSignal, databaseSignal domain.Signal) {
	addEdge(
		edges,
		"span:"+httpSignal.SpanID,
		"span:"+databaseSignal.SpanID,
		RelationQueries,
		[]string{httpSignal.ID, databaseSignal.ID},
	)
}

func materializeNodes(accumulators map[string]*nodeAccumulator) []Node {
	nodes := make([]Node, 0, len(accumulators))
	for _, accumulator := range accumulators {
		accumulator.node.EvidenceIDs = sortedSet(accumulator.evidence)
		nodes = append(nodes, accumulator.node)
	}
	sort.Slice(nodes, func(first, second int) bool { return nodes[first].ID < nodes[second].ID })
	return nodes
}

func materializeEdges(accumulators map[string]*edgeAccumulator) []Edge {
	edges := make([]Edge, 0, len(accumulators))
	for _, accumulator := range accumulators {
		accumulator.edge.EvidenceIDs = sortedSet(accumulator.evidence)
		edges = append(edges, accumulator.edge)
	}
	sort.Slice(edges, func(first, second int) bool {
		firstKey := edges[first].From + "\x00" + string(edges[first].Relation) + "\x00" + edges[first].To
		secondKey := edges[second].From + "\x00" + string(edges[second].Relation) + "\x00" + edges[second].To
		return firstKey < secondKey
	})
	return edges
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func signalIDs(signals []domain.Signal) []string {
	ids := make([]string, 0, len(signals))
	for _, signal := range signals {
		ids = append(ids, signal.ID)
	}
	sort.Strings(ids)
	return ids
}

func spanLabel(signal domain.Signal) string {
	if label := strings.TrimSpace(signal.Attributes["span.name"]); label != "" {
		return label
	}
	return signal.SpanID
}

func isHTTPSignal(signal domain.Signal) bool {
	return strings.TrimSpace(signal.Attributes["http.request.method"]) != "" || strings.TrimSpace(signal.Attributes["http.response.status_code"]) != ""
}

func isDatabaseSignal(signal domain.Signal) bool {
	return strings.TrimSpace(signal.Attributes["db.system.name"]) != "" || strings.TrimSpace(signal.Attributes["db.system"]) != ""
}
