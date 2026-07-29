package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestIngestTelemetryFileNormalizaEPersiste garante a primeira fatia completa de ingestão.
func TestIngestTelemetryFileNormalizaEPersiste(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join(t.TempDir(), "telemetria.json")
	fixture := `{
  "resourceSpans": [{
    "resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "checkout"}}]},
    "scopeSpans": [{"spans": [{
      "traceId": "00112233445566778899aabbccddeeff",
      "spanId": "0011223344556677",
      "name": "POST /checkout",
      "startTimeUnixNano": "1720000000000000000",
      "endTimeUnixNano": "1720000000250000000",
      "status": {"code": 1}
    }]}]
  }]
}`
	if err := os.WriteFile(fixturePath, []byte(fixture), 0o600); err != nil {
		t.Fatalf("escrever fixture: %v", err)
	}

	store := &signalStoreFake{}
	result, err := IngestTelemetryFile(context.Background(), fixturePath, store)
	if err != nil {
		t.Fatalf("IngestTelemetryFile() erro = %v", err)
	}
	if result.Normalized != 1 || result.Persisted != 1 {
		t.Fatalf("resultado = %#v, esperado um sinal normalizado e persistido", result)
	}
	if len(store.signals) != 1 || store.signals[0].ServiceName != "checkout" {
		t.Fatalf("sinais persistidos = %#v, esperado span do checkout", store.signals)
	}
}

// signalStoreFake registra a chamada de persistência sem usar infraestrutura externa.
type signalStoreFake struct {
	signals []domain.Signal
}

// Save implementa o contrato mínimo usado pelo caso de uso de ingestão.
func (store *signalStoreFake) Save(_ context.Context, signals []domain.Signal) (int, error) {
	store.signals = append(store.signals, signals...)
	return len(signals), nil
}
