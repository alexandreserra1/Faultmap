package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/faultmap/faultmap/internal/evidencegraph"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TraceSignalReader define a consulta limitada necessária para investigar um trace.
type TraceSignalReader interface {
	ListByTraceID(ctx context.Context, traceID string, limit int) ([]domain.Signal, error)
}

// TraceInvestigation reúne os sinais originais e o grafo derivado para explicação segura.
type TraceInvestigation struct {
	TraceID string
	Signals []domain.Signal
	Graph   evidencegraph.Graph
}

// BlameTrace carrega um trace uma única vez e constrói suas relações sem acessar outra infraestrutura.
func BlameTrace(ctx context.Context, traceID string, limit int, reader TraceSignalReader) (TraceInvestigation, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return TraceInvestigation{}, fmt.Errorf("investigar trace: trace ID é obrigatório")
	}
	if limit <= 0 {
		return TraceInvestigation{}, fmt.Errorf("investigar trace: limite deve ser maior que zero")
	}

	signals, err := reader.ListByTraceID(ctx, traceID, limit)
	if err != nil {
		return TraceInvestigation{}, fmt.Errorf("investigar trace %q: carregar sinais: %w", traceID, err)
	}
	graph, err := evidencegraph.Build(traceID, signals)
	if err != nil {
		return TraceInvestigation{}, fmt.Errorf("investigar trace %q: construir grafo: %w", traceID, err)
	}
	return TraceInvestigation{TraceID: traceID, Signals: signals, Graph: graph}, nil
}
