package sqlite

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestListServicesSharingTracesDescobreVizinhos cobre a expansão de escopo: a
// partir de um serviço de entrada, o Faultmap precisa descobrir quem mais
// participou dos mesmos traces durante o incidente. Sem isso o ranking nunca
// tem mais de um suspeito e não responde "por onde começar".
func TestListServicesSharingTracesDescobreVizinhos(t *testing.T) {
	t.Parallel()

	database := openRetentionDatabase(t)
	signals := NewSignalRepository(database)
	repository := NewScopeRepository(database)
	base := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)

	if _, err := signals.Save(context.Background(), []domain.Signal{
		signalNoTrace("checkout-1", "checkout-service", "trace-a", base),
		signalNoTrace("payment-1", "payment-service", "trace-a", base.Add(time.Second)),
		signalNoTrace("postgres-1", "payment-service", "trace-a", base.Add(2*time.Second)),
		signalNoTrace("checkout-2", "checkout-service", "trace-b", base.Add(3*time.Second)),
		signalNoTrace("search-1", "search-service", "trace-b", base.Add(4*time.Second)),
		// Serviço sem trace em comum: não pertence ao raio do incidente.
		signalNoTrace("relatorio-1", "report-service", "trace-z", base.Add(5*time.Second)),
		// Fora da janela: não deve entrar no escopo.
		signalNoTrace("checkout-antigo", "checkout-service", "trace-antigo", base.Add(-time.Hour)),
		signalNoTrace("legado-1", "legacy-service", "trace-antigo", base.Add(-time.Hour)),
	}); err != nil {
		t.Fatalf("Save() erro = %v", err)
	}

	services, traceCount, err := repository.ListServicesSharingTraces(
		context.Background(), "checkout-service", base, base.Add(10*time.Second), 100,
	)
	if err != nil {
		t.Fatalf("ListServicesSharingTraces() erro = %v", err)
	}
	esperado := []string{"checkout-service", "payment-service", "search-service"}
	if !reflect.DeepEqual(services, esperado) {
		t.Fatalf("serviços = %v, esperado %v", services, esperado)
	}
	if traceCount != 2 {
		t.Fatalf("traces considerados = %d, esperado 2", traceCount)
	}
}

// TestListServicesSharingTracesRespeitaLimite mantém a expansão previsível em
// um trace muito ramificado, que poderia puxar boa parte do sistema.
func TestListServicesSharingTracesRespeitaLimite(t *testing.T) {
	t.Parallel()

	database := openRetentionDatabase(t)
	repository := NewScopeRepository(database)
	base := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)

	lote := []domain.Signal{signalNoTrace("entrada", "aaa-service", "trace-a", base)}
	for _, nome := range []string{"bbb-service", "ccc-service", "ddd-service", "eee-service"} {
		lote = append(lote, signalNoTrace(nome+"-1", nome, "trace-a", base.Add(time.Second)))
	}
	if _, err := NewSignalRepository(database).Save(context.Background(), lote); err != nil {
		t.Fatalf("Save() erro = %v", err)
	}

	services, _, err := repository.ListServicesSharingTraces(
		context.Background(), "aaa-service", base, base.Add(time.Minute), 3,
	)
	if err != nil {
		t.Fatalf("ListServicesSharingTraces() erro = %v", err)
	}
	if len(services) != 3 {
		t.Fatalf("serviços = %v, esperado exatamente 3 pelo limite", services)
	}
	// A ordem estável garante que o mesmo escopo seja descoberto a cada execução.
	if services[0] != "aaa-service" {
		t.Fatalf("serviço de entrada precisa estar no escopo: %v", services)
	}
}

// TestListServicesInWindowEnumeraTodos cobre o modo para quem não tem por onde
// começar e precisa varrer a janela inteira.
func TestListServicesInWindowEnumeraTodos(t *testing.T) {
	t.Parallel()

	database := openRetentionDatabase(t)
	repository := NewScopeRepository(database)
	base := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)

	if _, err := NewSignalRepository(database).Save(context.Background(), []domain.Signal{
		signalNoTrace("b-1", "b-service", "trace-a", base),
		signalNoTrace("a-1", "a-service", "trace-b", base.Add(time.Second)),
		signalNoTrace("a-2", "a-service", "trace-c", base.Add(2*time.Second)),
		signalNoTrace("fora", "z-service", "trace-d", base.Add(-time.Hour)),
	}); err != nil {
		t.Fatalf("Save() erro = %v", err)
	}

	services, err := repository.ListServicesInWindow(context.Background(), base, base.Add(time.Minute), 100)
	if err != nil {
		t.Fatalf("ListServicesInWindow() erro = %v", err)
	}
	if !reflect.DeepEqual(services, []string{"a-service", "b-service"}) {
		t.Fatalf("serviços = %v, esperado [a-service b-service]", services)
	}
}

// TestListByServicesAndWindowCarregaEmUmaConsulta garante que os sinais de
// vários serviços sejam lidos de uma vez, e não com uma consulta por serviço.
func TestListByServicesAndWindowCarregaEmUmaConsulta(t *testing.T) {
	t.Parallel()

	database := openRetentionDatabase(t)
	signals := NewSignalRepository(database)
	base := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)

	if _, err := signals.Save(context.Background(), []domain.Signal{
		signalNoTrace("c-1", "checkout-service", "trace-a", base),
		signalNoTrace("p-1", "payment-service", "trace-a", base.Add(time.Second)),
		signalNoTrace("x-1", "outro-service", "trace-b", base.Add(2*time.Second)),
	}); err != nil {
		t.Fatalf("Save() erro = %v", err)
	}

	carregados, err := signals.ListByServicesAndWindow(
		context.Background(),
		[]string{"checkout-service", "payment-service"},
		base, base.Add(time.Minute), 100,
	)
	if err != nil {
		t.Fatalf("ListByServicesAndWindow() erro = %v", err)
	}
	if len(carregados) != 2 {
		t.Fatalf("sinais = %d, esperado 2", len(carregados))
	}
	for _, sinal := range carregados {
		if sinal.ServiceName == "outro-service" {
			t.Fatal("serviço fora do escopo foi carregado")
		}
	}
}

// TestListByServicesAndWindowRejeitaEscopoVazio evita varredura sem fronteira.
func TestListByServicesAndWindowRejeitaEscopoVazio(t *testing.T) {
	t.Parallel()

	repository := NewSignalRepository(openRetentionDatabase(t))
	base := time.Date(2026, time.August, 8, 12, 0, 0, 0, time.UTC)
	if _, err := repository.ListByServicesAndWindow(
		context.Background(), nil, base, base.Add(time.Minute), 100,
	); err == nil {
		t.Fatal("ListByServicesAndWindow() erro = nil para escopo vazio")
	}
}

func signalNoTrace(id, serviceName, traceID string, timestamp time.Time) domain.Signal {
	signal := testSignal(id, serviceName, timestamp)
	signal.TraceID = traceID
	return signal
}
