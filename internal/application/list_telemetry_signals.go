package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// SignalReader define a consulta mínima de sinais necessária para apresentar
// telemetria, sem acoplar o caso de uso a uma implementação de persistência.
type SignalReader interface {
	ListByServiceAndWindow(
		ctx context.Context,
		serviceName string,
		start time.Time,
		end time.Time,
		limit int,
	) ([]domain.Signal, error)
}

// ListSignals valida uma janela de consulta e delega a leitura ao
// repositório compartilhado, preservando o contexto do chamador.
func ListSignals(
	ctx context.Context,
	serviceName string,
	start time.Time,
	end time.Time,
	limit int,
	reader SignalReader,
) ([]domain.Signal, error) {
	if ctx == nil {
		return nil, fmt.Errorf("listar sinais de telemetria: contexto é obrigatório")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("listar sinais de telemetria: contexto cancelado: %w", err)
	}
	if strings.TrimSpace(serviceName) == "" {
		return nil, fmt.Errorf("listar sinais de telemetria: serviço é obrigatório")
	}
	if !start.Before(end) {
		return nil, fmt.Errorf("listar sinais de telemetria: início da janela deve ser anterior ao fim")
	}
	if limit <= 0 {
		return nil, fmt.Errorf("listar sinais de telemetria: limite deve ser maior que zero")
	}
	if reader == nil {
		return nil, fmt.Errorf("listar sinais de telemetria: repositório de sinais é obrigatório")
	}

	signals, err := reader.ListByServiceAndWindow(ctx, serviceName, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("listar sinais de telemetria para o serviço %q: %w", serviceName, err)
	}
	return signals, nil
}
