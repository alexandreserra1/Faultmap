package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestSignalRepositorySaveInsertsAndDeduplicatesByID garante que retries de
// ingestão não criam sinais duplicados.
func TestSignalRepositorySaveInsertsAndDeduplicatesByID(t *testing.T) {
	t.Parallel()

	repository := openSignalRepository(t)
	signal := testSignal("signal-1", "checkout", time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC))

	inserted, err := repository.Save(context.Background(), []domain.Signal{signal, signal})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if inserted != 1 {
		t.Fatalf("Save() inserted = %d, want 1", inserted)
	}

	inserted, err = repository.Save(context.Background(), []domain.Signal{signal})
	if err != nil {
		t.Fatalf("Save() retry error = %v", err)
	}
	if inserted != 0 {
		t.Fatalf("Save() retry inserted = %d, want 0", inserted)
	}
}

// TestSignalRepositoryListByServiceAndWindowUsesStableOrderAndLimit garante a
// consulta limitada por serviço e janela, ordenada por timestamp e ID.
func TestSignalRepositoryListByServiceAndWindowUsesStableOrderAndLimit(t *testing.T) {
	t.Parallel()

	repository := openSignalRepository(t)
	base := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	inserted, err := repository.Save(context.Background(), []domain.Signal{
		testSignal("signal-b", "checkout", base.Add(time.Minute)),
		testSignal("signal-a", "checkout", base.Add(time.Minute)),
		testSignal("signal-c", "checkout", base.Add(2*time.Minute)),
		testSignal("signal-other-service", "payment", base.Add(time.Minute)),
		testSignal("signal-before-window", "checkout", base.Add(-time.Minute)),
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if inserted != 5 {
		t.Fatalf("Save() inserted = %d, want 5", inserted)
	}

	signals, err := repository.ListByServiceAndWindow(
		context.Background(),
		"checkout",
		base,
		base.Add(3*time.Minute),
		2,
	)
	if err != nil {
		t.Fatalf("ListByServiceAndWindow() error = %v", err)
	}
	if len(signals) != 2 {
		t.Fatalf("ListByServiceAndWindow() returned %d signals, want 2", len(signals))
	}
	if signals[0].ID != "signal-a" || signals[1].ID != "signal-b" {
		t.Fatalf("ListByServiceAndWindow() IDs = [%s %s], want [signal-a signal-b]", signals[0].ID, signals[1].ID)
	}
	if got := signals[0].Attributes["http.route"]; got != "/checkout" {
		t.Fatalf("ListByServiceAndWindow() attributes http.route = %q, want /checkout", got)
	}
	if got := signals[0].Measurements["duration_ms"]; got != 250 {
		t.Fatalf("ListByServiceAndWindow() measurements duration_ms = %v, want 250", got)
	}
}

// TestSignalRepositoryListByTraceIDFiltersOrdersAndLimits garante que a busca
// usa o trace exato, devolve o domínio completo e aplica uma ordem estável.
func TestSignalRepositoryListByTraceIDFiltersOrdersAndLimits(t *testing.T) {
	t.Parallel()

	repository := openSignalRepository(t)
	base := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	requestedTraceID := "trace-' OR 1=1 --"
	first := testSignal("signal-a", "checkout", base.Add(time.Minute))
	first.TraceID = requestedTraceID
	first.SpanID = "span-a"
	first.Severity = "info"
	first.Attributes = map[string]string{"http.method": "POST", "http.route": "/checkout"}
	first.Measurements = map[string]float64{"duration_ms": 120, "http.status_code": 201}
	second := testSignal("signal-b", "checkout", base.Add(time.Minute))
	second.TraceID = requestedTraceID
	second.SpanID = "span-b"
	third := testSignal("signal-c", "checkout", base.Add(2*time.Minute))
	third.TraceID = requestedTraceID
	other := testSignal("signal-other-trace", "checkout", base)
	other.TraceID = "trace-other"

	inserted, err := repository.Save(context.Background(), []domain.Signal{third, second, other, first})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if inserted != 4 {
		t.Fatalf("Save() inserted = %d, want 4", inserted)
	}

	signals, err := repository.ListByTraceID(context.Background(), requestedTraceID, 2)
	if err != nil {
		t.Fatalf("ListByTraceID() error = %v", err)
	}
	if len(signals) != 2 {
		t.Fatalf("ListByTraceID() returned %d signals, want 2", len(signals))
	}
	if !reflect.DeepEqual(signals[0], first) {
		t.Fatalf("ListByTraceID() first signal = %#v, want %#v", signals[0], first)
	}
	if !reflect.DeepEqual(signals[1], second) {
		t.Fatalf("ListByTraceID() second signal = %#v, want %#v", signals[1], second)
	}
}

// TestSignalRepositoryListByTraceIDRejectsInvalidInputBeforeDatabaseAccess
// garante que entradas sem trace ou sem limite não chegam ao pool compartilhado.
func TestSignalRepositoryListByTraceIDRejectsInvalidInputBeforeDatabaseAccess(t *testing.T) {
	t.Parallel()

	repository := NewSignalRepository(nil)
	testCases := []struct {
		name    string
		traceID string
		limit   int
	}{
		{name: "trace vazio", traceID: "", limit: 10},
		{name: "limite zero", traceID: "trace-1", limit: 0},
		{name: "limite negativo", traceID: "trace-1", limit: -1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := repository.ListByTraceID(context.Background(), testCase.traceID, testCase.limit)
			if err == nil {
				t.Fatal("ListByTraceID() error = nil, want validation error")
			}
		})
	}
}

// TestSignalRepositoryListByTraceIDRespectsCanceledContext garante que a
// consulta interrompida preserva context.Canceled para o caso de uso.
func TestSignalRepositoryListByTraceIDRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	repository := openSignalRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repository.ListByTraceID(ctx, "trace-1", 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListByTraceID() error = %v, want context.Canceled", err)
	}
}

func openSignalRepository(t *testing.T) *SignalRepository {
	t.Helper()

	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "faultmap.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close database: %v", closeErr)
		}
	})
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return NewSignalRepository(database)
}

func testSignal(id, serviceName string, timestamp time.Time) domain.Signal {
	return domain.Signal{
		ID:           id,
		Type:         domain.SignalType("span"),
		ServiceName:  serviceName,
		Timestamp:    timestamp,
		TraceID:      "trace-1",
		SpanID:       "span-1",
		Severity:     "error",
		Attributes:   map[string]string{"http.route": "/checkout"},
		Measurements: map[string]float64{"duration_ms": 250},
	}
}
