package application

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
	"github.com/faultmap/faultmap/internal/telemetry/normalizer"
)

// SignalStore define a persistência mínima necessária para o caso de uso de ingestão.
type SignalStore interface {
	Save(ctx context.Context, signals []domain.Signal) (int, error)
}

// IngestionResult informa quantos sinais foram normalizados e persistidos em uma importação.
type IngestionResult struct {
	Normalized int
	Persisted  int
}

// IngestTelemetryFile normaliza um arquivo OTLP JSON e persiste seus sinais de forma idempotente.
func IngestTelemetryFile(ctx context.Context, inputPath string, store SignalStore) (IngestionResult, error) {
	if err := contextError(ctx); err != nil {
		return IngestionResult{}, err
	}
	if strings.TrimSpace(inputPath) == "" {
		return IngestionResult{}, fmt.Errorf("ingerir arquivo de telemetria: caminho de entrada é obrigatório")
	}
	if store == nil {
		return IngestionResult{}, fmt.Errorf("ingerir arquivo de telemetria: repositório de sinais é obrigatório")
	}

	input, err := os.Open(inputPath)
	if err != nil {
		return IngestionResult{}, fmt.Errorf("abrir arquivo de telemetria %q: %w", inputPath, err)
	}

	signals, parseErr := normalizer.ParseOTLPJSON(ctx, input)
	closeErr := input.Close()
	if parseErr != nil {
		return IngestionResult{}, fmt.Errorf("normalizar arquivo de telemetria %q: %w", inputPath, parseErr)
	}
	if closeErr != nil {
		return IngestionResult{}, fmt.Errorf("fechar arquivo de telemetria %q: %w", inputPath, closeErr)
	}
	if err := contextError(ctx); err != nil {
		return IngestionResult{}, err
	}

	persisted, err := store.Save(ctx, signals)
	if err != nil {
		return IngestionResult{}, fmt.Errorf("persistir sinais normalizados: %w", err)
	}
	return IngestionResult{Normalized: len(signals), Persisted: persisted}, nil
}

func contextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("ingerir arquivo de telemetria: contexto cancelado: %w", err)
	}
	return nil
}
