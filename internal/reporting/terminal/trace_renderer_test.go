package terminal

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/evidencegraph"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestRenderTraceInvestigationExibeSomenteDetalhesSeguros garante uma sequência
// compreensível sem imprimir atributos arbitrários ou SQL bruto.
func TestRenderTraceInvestigationExibeSomenteDetalhesSeguros(t *testing.T) {
	t.Parallel()

	const traceID = "trace-1"
	signals := []domain.Signal{
		{
			ID: "http", ServiceName: "checkout-service", TraceID: traceID, SpanID: "http-span", Timestamp: time.Unix(1, 0), Severity: "error",
			Attributes:   map[string]string{"span.name": "POST /checkout", "http.response.status_code": "500", "authorization": "segredo"},
			Measurements: map[string]float64{"duration_ms": 800},
		},
		{
			ID: "db", ServiceName: "checkout-service", TraceID: traceID, SpanID: "db-span", Timestamp: time.Unix(2, 0), Severity: "error",
			Attributes:   map[string]string{"span.name": "INSERT orders", "db.system.name": "postgresql", "db.operation.name": "INSERT", "error.type": "timeout", "db.statement": "INSERT INTO orders VALUES (segredo)"},
			Measurements: map[string]float64{"duration_ms": 650},
		},
	}
	graph := evidencegraph.Graph{TraceID: traceID, Edges: []evidencegraph.Edge{{
		From: "span:http-span", To: "span:db-span", Relation: evidencegraph.RelationQueries,
	}}}

	var output bytes.Buffer
	if err := RenderTraceInvestigation(&output, traceID, signals, graph); err != nil {
		t.Fatalf("RenderTraceInvestigation() erro = %v", err)
	}
	result := output.String()
	for _, expected := range []string{
		"Investigação do trace — trace-1",
		"Serviço: checkout-service",
		"Grafo de evidências:",
		"POST /checkout",
		"HTTP 500",
		"duração 800 ms",
		"consulta → INSERT orders",
		"PostgreSQL",
		"operação INSERT",
		"erro timeout",
		"duração 650 ms",
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("saída não contém %q:\n%s", expected, result)
		}
	}
	for _, secret := range []string{"segredo", "INSERT INTO"} {
		if strings.Contains(result, secret) {
			t.Errorf("saída contém dado bloqueado %q:\n%s", secret, result)
		}
	}
}
