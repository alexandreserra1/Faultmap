package application

import (
	"context"
	"fmt"
	"time"
)

const (
	// maxRetentionBatches limita quantos lotes uma execução pode remover. O teto
	// substitui um laço "até esvaziar", que poderia nunca terminar em um banco
	// recebendo telemetria continuamente, e mantém a duração do comando previsível.
	maxRetentionBatches = 200
	// maxRetentionBatchSize impede uma transação de escrita longa demais para o
	// único escritor do SQLite.
	maxRetentionBatchSize = 10_000
	// DefaultRetentionBatchSize equilibra número de transações e tempo de bloqueio.
	DefaultRetentionBatchSize = 1_000
)

// RetentionRequest descreve uma execução explícita da política de retenção.
// Now é recebido em vez de lido do relógio para tornar a operação reproduzível.
type RetentionRequest struct {
	Retention time.Duration
	Now       time.Time
	BatchSize int
}

// RetentionResult relata o efeito real da limpeza, incluindo se ela foi
// interrompida pelo teto de lotes e ainda existe telemetria expirada.
type RetentionResult struct {
	Cutoff         time.Time
	SignalsRemoved int
	Batches        int
	Truncated      bool
}

// SignalRetentionRemover abstrai a remoção em lote de telemetria expirada.
type SignalRetentionRemover interface {
	DeleteSignalsBefore(ctx context.Context, cutoff time.Time, limit int) (int, error)
}

// ApplyRetention remove telemetria anterior a Now-Retention em lotes limitados.
//
// Efeitos colaterais: a operação apaga sinais definitivamente e não é
// reversível. Snapshots de diagnóstico não são tocados — a decisão está
// documentada no repositório de retenção. Em caso de erro, os lotes já
// confirmados permanecem removidos; repetir o comando simplesmente continua de
// onde parou, sem retry automático nem laço infinito.
func ApplyRetention(
	ctx context.Context,
	request RetentionRequest,
	remover SignalRetentionRemover,
) (RetentionResult, error) {
	if request.Retention <= 0 {
		return RetentionResult{}, fmt.Errorf("aplicar retenção: a retenção deve ser maior que zero")
	}
	if request.BatchSize <= 0 || request.BatchSize > maxRetentionBatchSize {
		return RetentionResult{}, fmt.Errorf(
			"aplicar retenção: o lote deve estar entre 1 e %d", maxRetentionBatchSize,
		)
	}

	result := RetentionResult{Cutoff: request.Now.UTC().Add(-request.Retention)}
	for result.Batches < maxRetentionBatches {
		removed, err := remover.DeleteSignalsBefore(ctx, result.Cutoff, request.BatchSize)
		if err != nil {
			return RetentionResult{}, fmt.Errorf("aplicar retenção: %w", err)
		}
		result.Batches++
		result.SignalsRemoved += removed
		// Um lote incompleto prova que a telemetria expirada acabou; continuar
		// apenas repetiria uma consulta que já devolveu tudo o que existia.
		if removed < request.BatchSize {
			return result, nil
		}
	}
	result.Truncated = true
	return result, nil
}
