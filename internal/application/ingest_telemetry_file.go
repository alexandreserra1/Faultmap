package application

import (
	"context"
	"fmt"
	"io"
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

// IngestTelemetry normaliza traces OTLP recebidos de qualquer transporte e os
// persiste de forma idempotente usando o repositório compartilhado da aplicação.
func IngestTelemetry(ctx context.Context, reader io.Reader, encoding normalizer.OTLPEncoding, store SignalStore) (IngestionResult, error) {
	if err := contextError(ctx); err != nil {
		return IngestionResult{}, err
	}
	if reader == nil {
		return IngestionResult{}, fmt.Errorf("ingerir telemetria: leitor é obrigatório")
	}
	if store == nil {
		return IngestionResult{}, fmt.Errorf("ingerir telemetria: repositório de sinais é obrigatório")
	}

	signals, err := normalizer.ParseOTLPTraces(ctx, reader, encoding)
	if err != nil {
		return IngestionResult{}, fmt.Errorf("normalizar telemetria: %w", err)
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

	result, ingestErr := IngestTelemetry(ctx, input, normalizer.OTLPEncodingJSON, store)
	closeErr := input.Close()
	if ingestErr != nil {
		return IngestionResult{}, fmt.Errorf("ingerir arquivo de telemetria %q: %w", inputPath, ingestErr)
	}
	if closeErr != nil {
		return IngestionResult{}, fmt.Errorf("fechar arquivo de telemetria %q: %w", inputPath, closeErr)
	}
	return result, nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("ingerir telemetria: contexto é obrigatório")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("ingerir arquivo de telemetria: contexto cancelado: %w", err)
	}
	return nil
}
