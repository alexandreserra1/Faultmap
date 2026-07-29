package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestListSignalsValidaEntradaAntesDeConsultar garante que entradas
// inválidas não chegam ao repositório de telemetria.
func TestListSignalsValidaEntradaAntesDeConsultar(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)
	testCases := []struct {
		name        string
		serviceName string
		start       time.Time
		end         time.Time
		limit       int
	}{
		{
			name:        "serviço vazio",
			serviceName: "  ",
			start:       start,
			end:         start.Add(time.Minute),
			limit:       1,
		},
		{
			name:        "janela vazia",
			serviceName: "checkout",
			start:       start,
			end:         start,
			limit:       1,
		},
		{
			name:        "janela invertida",
			serviceName: "checkout",
			start:       start,
			end:         start.Add(-time.Minute),
			limit:       1,
		},
		{
			name:        "limite não positivo",
			serviceName: "checkout",
			start:       start,
			end:         start.Add(time.Minute),
			limit:       0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			reader := &signalReaderFake{}
			_, err := ListSignals(
				context.Background(),
				testCase.serviceName,
				testCase.start,
				testCase.end,
				testCase.limit,
				reader,
			)
			if err == nil {
				t.Fatal("ListSignals() deveria rejeitar entrada inválida")
			}
			if reader.calls != 0 {
				t.Fatalf("consultas ao repositório = %d, esperado 0", reader.calls)
			}
		})
	}
}

// TestListSignalsEncaminhaConsulta garante que o caso de uso mantém
// o filtro recebido e devolve os sinais entregues pelo repositório.
func TestListSignalsEncaminhaConsulta(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	expected := []domain.Signal{{ID: "signal-1", ServiceName: "checkout"}}
	reader := &signalReaderFake{signals: expected}

	signals, err := ListSignals(context.Background(), "checkout", start, end, 20, reader)
	if err != nil {
		t.Fatalf("ListSignals() erro = %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("consultas ao repositório = %d, esperado 1", reader.calls)
	}
	if reader.serviceName != "checkout" || !reader.start.Equal(start) || !reader.end.Equal(end) || reader.limit != 20 {
		t.Fatalf("filtros encaminhados = %#v, esperado checkout, %v, %v, 20", reader, start, end)
	}
	if len(signals) != 1 || signals[0].ID != "signal-1" {
		t.Fatalf("sinais = %#v, esperado %#v", signals, expected)
	}
}

// TestListSignalsRespeitaContextoCancelado garante que uma consulta
// não é iniciada quando o chamador já cancelou sua operação.
func TestListSignalsRespeitaContextoCancelado(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := &signalReaderFake{}
	start := time.Date(2026, time.July, 28, 10, 0, 0, 0, time.UTC)

	_, err := ListSignals(ctx, "checkout", start, start.Add(time.Minute), 1, reader)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("erro = %v, esperado context.Canceled", err)
	}
	if reader.calls != 0 {
		t.Fatalf("consultas ao repositório = %d, esperado 0", reader.calls)
	}
}

// signalReaderFake registra os filtros enviados ao contrato de leitura sem
// depender de SQLite, permitindo testar só a orquestração da aplicação.
type signalReaderFake struct {
	signals     []domain.Signal
	err         error
	calls       int
	serviceName string
	start       time.Time
	end         time.Time
	limit       int
}

// ListByServiceAndWindow implementa o contrato mínimo de consulta de sinais.
func (reader *signalReaderFake) ListByServiceAndWindow(
	_ context.Context,
	serviceName string,
	start time.Time,
	end time.Time,
	limit int,
) ([]domain.Signal, error) {
	reader.calls++
	reader.serviceName = serviceName
	reader.start = start
	reader.end = end
	reader.limit = limit
	return reader.signals, reader.err
}
