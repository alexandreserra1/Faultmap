package application

import (
	"context"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/evidencegraph"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestBlameTraceCarregaUmaVezEConstroiGrafo garante que a investigação reutiliza
// uma única leitura limitada e entrega os sinais junto da estrutura explicável.
func TestBlameTraceCarregaUmaVezEConstroiGrafo(t *testing.T) {
	t.Parallel()

	const traceID = "trace-1"
	reader := &traceReaderFake{signals: []domain.Signal{
		traceSignal("http", traceID, "http-span", time.Unix(1, 0), map[string]string{"http.request.method": "POST"}),
		traceSignal("db", traceID, "db-span", time.Unix(2, 0), map[string]string{"db.system.name": "postgresql", "span.parent_id": "http-span"}),
	}}

	investigation, err := BlameTrace(context.Background(), traceID, 20, reader)
	if err != nil {
		t.Fatalf("BlameTrace() erro = %v", err)
	}
	if reader.calls != 1 || reader.traceID != traceID || reader.limit != 20 {
		t.Fatalf("leitura = calls %d trace %q limit %d", reader.calls, reader.traceID, reader.limit)
	}
	if len(investigation.Signals) != 2 || investigation.Graph.TraceID != traceID {
		t.Fatalf("investigação incompleta: %#v", investigation)
	}
	if !hasRelation(investigation.Graph, evidencegraph.RelationQueries) {
		t.Fatalf("grafo não contém relação queries: %#v", investigation.Graph.Edges)
	}
}

// TestBlameTraceRejeitaEntradaAntesDoRepositorio evita I/O para uma investigação impossível.
func TestBlameTraceRejeitaEntradaAntesDoRepositorio(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name    string
		traceID string
		limit   int
	}{
		{name: "trace vazio", traceID: " ", limit: 20},
		{name: "limite inválido", traceID: "trace-1", limit: 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			reader := &traceReaderFake{}
			if _, err := BlameTrace(context.Background(), testCase.traceID, testCase.limit, reader); err == nil {
				t.Fatal("BlameTrace() erro = nil")
			}
			if reader.calls != 0 {
				t.Fatalf("consultas = %d, esperado zero", reader.calls)
			}
		})
	}
}

type traceReaderFake struct {
	signals []domain.Signal
	calls   int
	traceID string
	limit   int
}

// ListByTraceID implementa a única leitura limitada necessária à investigação.
func (reader *traceReaderFake) ListByTraceID(_ context.Context, traceID string, limit int) ([]domain.Signal, error) {
	reader.calls++
	reader.traceID = traceID
	reader.limit = limit
	return reader.signals, nil
}

func traceSignal(id, traceID, spanID string, timestamp time.Time, attributes map[string]string) domain.Signal {
	return domain.Signal{
		ID: id, Type: domain.SignalTypeSpan, ServiceName: "checkout-service",
		TraceID: traceID, SpanID: spanID, Timestamp: timestamp, Attributes: attributes,
	}
}

func hasRelation(graph evidencegraph.Graph, relation evidencegraph.Relation) bool {
	for _, edge := range graph.Edges {
		if edge.Relation == relation {
			return true
		}
	}
	return false
}
