package application

import (
	"context"
	"errors"
	"testing"
	"time"
)

type retentionRemoverStub struct {
	cutoffs    []time.Time
	limits     []int
	removals   []int
	failAtCall int
	callCount  int
}

func (stub *retentionRemoverStub) DeleteSignalsBefore(_ context.Context, cutoff time.Time, limit int) (int, error) {
	stub.cutoffs = append(stub.cutoffs, cutoff)
	stub.limits = append(stub.limits, limit)
	stub.callCount++
	if stub.failAtCall == stub.callCount {
		return 0, errors.New("falha simulada de banco")
	}
	if stub.callCount > len(stub.removals) {
		return 0, nil
	}
	return stub.removals[stub.callCount-1], nil
}

// TestApplyRetentionCalculaCorteEEncerraQuandoNãoHáMaisSinais confirma que a
// limpeza avança por lotes e para assim que um lote volta incompleto.
func TestApplyRetentionCalculaCorteEEncerraQuandoNãoHáMaisSinais(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	remover := &retentionRemoverStub{removals: []int{500, 500, 120}}

	result, err := ApplyRetention(context.Background(), RetentionRequest{
		Retention: 7 * 24 * time.Hour,
		Now:       now,
		BatchSize: 500,
	}, remover)
	if err != nil {
		t.Fatalf("ApplyRetention() erro = %v", err)
	}
	if result.SignalsRemoved != 1_120 {
		t.Fatalf("SignalsRemoved = %d, esperado 1120", result.SignalsRemoved)
	}
	if result.Cutoff != now.Add(-7*24*time.Hour) {
		t.Fatalf("Cutoff = %v, esperado %v", result.Cutoff, now.Add(-7*24*time.Hour))
	}
	if len(remover.limits) != 3 {
		t.Fatalf("lotes executados = %d, esperado 3", len(remover.limits))
	}
	for index, cutoff := range remover.cutoffs {
		if cutoff != result.Cutoff {
			t.Fatalf("lote %d usou corte %v, esperado %v", index, cutoff, result.Cutoff)
		}
	}
}

// TestApplyRetentionRespeitaTetoDeLotes evita que a limpeza vire um laço
// indefinido quando novos sinais expiram durante a própria execução.
func TestApplyRetentionRespeitaTetoDeLotes(t *testing.T) {
	t.Parallel()

	remover := &retentionRemoverStub{}
	for index := 0; index < maxRetentionBatches+10; index++ {
		remover.removals = append(remover.removals, 1)
	}

	result, err := ApplyRetention(context.Background(), RetentionRequest{
		Retention: time.Hour,
		Now:       time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC),
		BatchSize: 1,
	}, remover)
	if err != nil {
		t.Fatalf("ApplyRetention() erro = %v", err)
	}
	if remover.callCount != maxRetentionBatches {
		t.Fatalf("lotes executados = %d, esperado %d", remover.callCount, maxRetentionBatches)
	}
	if !result.Truncated {
		t.Fatal("Truncated = false, esperado true quando o teto de lotes é atingido")
	}
}

// TestApplyRetentionPropagaFalhaSemMascararProgresso mantém a causa original e
// não converte erro de banco em sucesso parcial silencioso.
func TestApplyRetentionPropagaFalhaSemMascararProgresso(t *testing.T) {
	t.Parallel()

	remover := &retentionRemoverStub{removals: []int{10, 0}, failAtCall: 2}

	_, err := ApplyRetention(context.Background(), RetentionRequest{
		Retention: time.Hour,
		Now:       time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC),
		BatchSize: 10,
	}, remover)
	if err == nil {
		t.Fatal("ApplyRetention() erro = nil, esperado propagação da falha")
	}
}

// TestApplyRetentionValidaEntrada impede corte no futuro ou lote sem fronteira.
func TestApplyRetentionValidaEntrada(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		request RetentionRequest
	}{
		{name: "retenção zero", request: RetentionRequest{Retention: 0, BatchSize: 10}},
		{name: "retenção negativa", request: RetentionRequest{Retention: -time.Hour, BatchSize: 10}},
		{name: "lote zero", request: RetentionRequest{Retention: time.Hour, BatchSize: 0}},
		{name: "lote acima do máximo", request: RetentionRequest{Retention: time.Hour, BatchSize: maxRetentionBatchSize + 1}},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := testCase.request
			request.Now = time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
			if _, err := ApplyRetention(context.Background(), request, &retentionRemoverStub{}); err == nil {
				t.Fatalf("ApplyRetention() erro = nil para %s", testCase.name)
			}
		})
	}
}
