package terminal

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

func TestRenderSignalsMostraResumoSeguroEOrdenado(t *testing.T) {
	t.Parallel()

	firstTimestamp := time.Date(2025, time.December, 1, 12, 1, 0, 0, time.UTC)
	secondTimestamp := firstTimestamp.Add(2 * time.Second)
	signals := []domain.Signal{
		{
			ID:         "trace-b:span-b",
			Timestamp:  secondTimestamp,
			TraceID:    "trace-b",
			Severity:   "info",
			Attributes: map[string]string{"span.name": "INSERT orders", "db.system.name": "postgresql", "db.operation.name": "INSERT", "db.statement": "INSERT INTO orders (card_number) VALUES ('4111')"},
			Measurements: map[string]float64{
				"duration_ms": 50,
			},
		},
		{
			ID:         "trace-a:span-a",
			Timestamp:  firstTimestamp,
			TraceID:    "trace-a",
			Severity:   "error",
			Attributes: map[string]string{"span.name": "POST /checkout", "http.response.status_code": "500", "error.type": "database_timeout", "authorization": "Bearer segredo"},
			Measurements: map[string]float64{
				"duration_ms": 2300,
			},
		},
	}

	var output bytes.Buffer
	if err := RenderSignals(&output, "checkout-service", signals); err != nil {
		t.Fatalf("RenderSignals() error = %v", err)
	}

	const expected = "Telemetria de checkout-service — 2 sinais\n\n" +
		"2025-12-01 12:01:00 UTC  ERRO  POST /checkout\n" +
		"  HTTP 500 · erro database_timeout · duração 2300 ms · trace trace-a\n\n" +
		"2025-12-01 12:01:02 UTC  INFO  INSERT orders\n" +
		"  Banco PostgreSQL · operação INSERT · duração 50 ms · trace trace-b\n"
	if got := output.String(); got != expected {
		t.Errorf("RenderSignals() =\n%s\nqueremos:\n%s", got, expected)
	}
	if strings.Contains(output.String(), "segredo") || strings.Contains(output.String(), "card_number") {
		t.Errorf("RenderSignals() exibiu atributo sensível: %q", output.String())
	}
}

func TestRenderSignalsTrataListaVazia(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if err := RenderSignals(&output, "checkout-service", nil); err != nil {
		t.Fatalf("RenderSignals() error = %v", err)
	}

	const expected = "Telemetria de checkout-service — nenhum sinal encontrado.\n"
	if got := output.String(); got != expected {
		t.Errorf("RenderSignals() = %q, queremos %q", got, expected)
	}
}

func TestRenderSignalsRetornaErroDeEscrita(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("destino indisponível")
	err := RenderSignals(failingWriter{err: expectedErr}, "checkout-service", nil)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("RenderSignals() error = %v, queremos causa %v", err, expectedErr)
	}
}

type failingWriter struct {
	err error
}

func (writer failingWriter) Write([]byte) (int, error) {
	return 0, writer.err
}
