package application

import (
	"context"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

type scopeReaderFake struct {
	vizinhos     []string
	traceCount   int
	todos        []string
	chamadas     int
	escopoPedido []string
}

func (fake *scopeReaderFake) ListServicesSharingTraces(
	_ context.Context, entryService string, _ time.Time, _ time.Time, _ int,
) ([]string, int, error) {
	fake.chamadas++
	if len(fake.vizinhos) == 0 {
		return []string{entryService}, 0, nil
	}
	return fake.vizinhos, fake.traceCount, nil
}

func (fake *scopeReaderFake) ListServicesInWindow(
	_ context.Context, _ time.Time, _ time.Time, _ int,
) ([]string, error) {
	fake.chamadas++
	return fake.todos, nil
}

type scopedSignalReaderFake struct {
	baseline  []domain.Signal
	incident  []domain.Signal
	consultas int
	escopos   [][]string
}

func (fake *scopedSignalReaderFake) ListByServicesAndWindow(
	_ context.Context, serviceNames []string, _ time.Time, _ time.Time, _ int,
) ([]domain.Signal, error) {
	fake.consultas++
	fake.escopos = append(fake.escopos, append([]string(nil), serviceNames...))
	if fake.consultas == 1 {
		return fake.baseline, nil
	}
	return fake.incident, nil
}

type scopedDeploymentReaderFake struct {
	deployments []changedomain.Deployment
	consultas   int
}

func (fake *scopedDeploymentReaderFake) ListDeploymentsForServices(
	_ context.Context, _ []string, _ string, _ time.Time, _ time.Time, _ int,
) ([]changedomain.Deployment, error) {
	fake.consultas++
	return fake.deployments, nil
}

// TestDiagnoseScopeRanqueiaVáriosServiços é o teste central da mudança: o
// ranking precisa comparar serviços entre si. Antes disso ele nunca tinha mais
// de um suspeito, e a métrica de top-3 era verdadeira por vacuidade.
func TestDiagnoseScopeRanqueiaVáriosServiços(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2026, time.August, 8, 10, 0, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(
		incidentStart, incidentStart.Add(time.Minute), time.Minute,
	)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}

	baseline := make([]domain.Signal, 0, 40)
	incident := make([]domain.Signal, 0, 40)
	for indice := 0; indice < 20; indice++ {
		baseline = append(baseline,
			escopoHTTPSignal("checkout", "checkout-service", indice, incidentStart.Add(-time.Minute), 201, 10),
			escopoHTTPSignal("payment", "payment-service", indice, incidentStart.Add(-time.Minute), 201, 10),
		)
		// Só o payment regride; o checkout apenas fica mais lento por consequência.
		incident = append(incident,
			escopoHTTPSignal("checkout", "checkout-service", indice, incidentStart, 200, 180),
			escopoHTTPSignal("payment", "payment-service", indice, incidentStart, 500, 200),
		)
	}

	scope := &scopeReaderFake{vizinhos: []string{"checkout-service", "payment-service"}, traceCount: 20}
	signals := &scopedSignalReaderFake{baseline: baseline, incident: incident}

	diagnosis, err := DiagnoseIncidentInScope(context.Background(), ScopedDiagnosisRequest{
		EntryService: "checkout-service",
		Windows:      windows,
		Limit:        500,
		MaxServices:  10,
		Ranking:      testRankingConfig(),
	}, scope, signals, nil)
	if err != nil {
		t.Fatalf("DiagnoseIncidentInScope() erro = %v", err)
	}

	if len(diagnosis.Suspects) < 2 {
		t.Fatalf("suspeitos = %d, esperado ao menos 2: o ranking precisa comparar serviços", len(diagnosis.Suspects))
	}
	if diagnosis.Suspects[0].ID != "payment-service" {
		t.Fatalf("primeiro suspeito = %q, esperado payment-service", diagnosis.Suspects[0].ID)
	}
	if diagnosis.Scope.TraceCount != 20 {
		t.Fatalf("traces do escopo = %d, esperado 20", diagnosis.Scope.TraceCount)
	}
	if len(diagnosis.Scope.Services) != 2 {
		t.Fatalf("escopo = %v, esperado dois serviços", diagnosis.Scope.Services)
	}

	// Uma consulta por janela, nunca uma por serviço.
	if signals.consultas != 2 {
		t.Fatalf("consultas de sinais = %d, esperado 2 (uma por janela)", signals.consultas)
	}
	for _, escopo := range signals.escopos {
		if len(escopo) != 2 {
			t.Fatalf("consulta feita com escopo parcial: %v", escopo)
		}
	}
}

// TestDiagnoseScopeConsultaDeploymentsEmLote protege contra N+1 na correlação
// de mudanças, que cresceria junto com o raio do incidente.
func TestDiagnoseScopeConsultaDeploymentsEmLote(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2026, time.August, 8, 10, 0, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(
		incidentStart, incidentStart.Add(time.Minute), time.Minute,
	)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}

	scope := &scopeReaderFake{vizinhos: []string{"a-service", "b-service", "c-service"}, traceCount: 5}
	signals := &scopedSignalReaderFake{}
	deployments := &scopedDeploymentReaderFake{}

	if _, err := DiagnoseIncidentInScope(context.Background(), ScopedDiagnosisRequest{
		EntryService: "a-service",
		Environment:  "demo",
		Windows:      windows,
		Limit:        500,
		MaxServices:  10,
		Ranking:      testRankingConfig(),
	}, scope, signals, deployments); err != nil {
		t.Fatalf("DiagnoseIncidentInScope() erro = %v", err)
	}

	if deployments.consultas != 1 {
		t.Fatalf("consultas de deployments = %d, esperado 1 para os três serviços", deployments.consultas)
	}
}

// TestDiagnoseScopeSemExpansãoMantémUmServiço preserva o modo focado para quem
// já sabe o que quer investigar.
func TestDiagnoseScopeSemExpansãoMantémUmServiço(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2026, time.August, 8, 10, 0, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(
		incidentStart, incidentStart.Add(time.Minute), time.Minute,
	)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}

	scope := &scopeReaderFake{vizinhos: []string{"checkout-service", "payment-service"}, traceCount: 9}
	signals := &scopedSignalReaderFake{}

	diagnosis, err := DiagnoseIncidentInScope(context.Background(), ScopedDiagnosisRequest{
		EntryService: "checkout-service",
		Windows:      windows,
		Limit:        500,
		MaxServices:  10,
		NoExpand:     true,
		Ranking:      testRankingConfig(),
	}, scope, signals, nil)
	if err != nil {
		t.Fatalf("DiagnoseIncidentInScope() erro = %v", err)
	}
	if len(diagnosis.Scope.Services) != 1 || diagnosis.Scope.Services[0] != "checkout-service" {
		t.Fatalf("escopo = %v, esperado apenas o serviço de entrada", diagnosis.Scope.Services)
	}
	if scope.chamadas != 0 {
		t.Fatal("descoberta de escopo foi consultada apesar de --no-expand")
	}
}

// TestDiagnoseScopeIDNãoDependeDoEscopoDescoberto mantém a idempotência: o
// escopo é descoberto a cada execução e pode variar com a telemetria que chegou,
// mas a identidade da investigação é a mesma.
func TestDiagnoseScopeIDNãoDependeDoEscopoDescoberto(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2026, time.August, 8, 10, 0, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(
		incidentStart, incidentStart.Add(time.Minute), time.Minute,
	)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}

	primeiro, err := DiagnoseIncidentInScope(context.Background(), ScopedDiagnosisRequest{
		EntryService: "checkout-service", Windows: windows, Limit: 500, MaxServices: 10,
		Ranking: testRankingConfig(),
	}, &scopeReaderFake{vizinhos: []string{"checkout-service"}}, &scopedSignalReaderFake{}, nil)
	if err != nil {
		t.Fatalf("primeira execução erro = %v", err)
	}
	segundo, err := DiagnoseIncidentInScope(context.Background(), ScopedDiagnosisRequest{
		EntryService: "checkout-service", Windows: windows, Limit: 500, MaxServices: 10,
		Ranking: testRankingConfig(),
	}, &scopeReaderFake{vizinhos: []string{"checkout-service", "payment-service"}}, &scopedSignalReaderFake{}, nil)
	if err != nil {
		t.Fatalf("segunda execução erro = %v", err)
	}
	if primeiro.ID != segundo.ID {
		t.Fatalf("ID mudou com o escopo: %q vs %q", primeiro.ID, segundo.ID)
	}
}

func escopoHTTPSignal(prefixo, servico string, indice int, timestamp time.Time, status int, duracao float64) domain.Signal {
	signal := diagnosisHTTPSignal(prefixo+"-"+timestamp.Format("150405")+"-"+string(rune('a'+indice%26)), timestamp, status, duracao)
	signal.ID = prefixo + "-" + timestamp.Format("150405.000") + "-" + string(rune('a'+indice%26))
	signal.ServiceName = servico
	signal.TraceID = "trace-" + string(rune('a'+indice%26))
	return signal
}

// TestDiagnoseScopeDesempataPeloServiçoMaisProfundo cobre um erro observado na
// matriz E2E: com o payment devolvendo 500 e o checkout devolvendo 502 por
// consequência, os dois empatavam em score e o desempate alfabético colocava a
// vítima em primeiro lugar. Quem está mais fundo na cadeia da requisição é o
// candidato mais plausível à origem.
func TestDiagnoseScopeDesempataPeloServiçoMaisProfundo(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2026, time.August, 8, 10, 0, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(
		incidentStart, incidentStart.Add(time.Minute), time.Minute,
	)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}

	baseline := make([]domain.Signal, 0, 40)
	incident := make([]domain.Signal, 0, 40)
	for indice := 0; indice < 20; indice++ {
		trace := "trace-" + string(rune('a'+indice%26))
		baseline = append(baseline,
			cadeiaSignal("checkout-service", trace, "raiz", "", indice, incidentStart.Add(-time.Minute), 200),
			cadeiaSignal("payment-service", trace, "folha", "raiz", indice, incidentStart.Add(-time.Minute), 200),
		)
		// Ambos falham 100%: o payment na origem, o checkout por propagação.
		incident = append(incident,
			cadeiaSignal("checkout-service", trace, "raiz", "", indice, incidentStart, 502),
			cadeiaSignal("payment-service", trace, "folha", "raiz", indice, incidentStart, 500),
		)
	}

	diagnosis, err := DiagnoseIncidentInScope(context.Background(), ScopedDiagnosisRequest{
		EntryService: "payment-service",
		Windows:      windows,
		Limit:        500,
		MaxServices:  10,
		Ranking:      testRankingConfig(),
	},
		&scopeReaderFake{vizinhos: []string{"checkout-service", "payment-service"}, traceCount: 20},
		&scopedSignalReaderFake{baseline: baseline, incident: incident},
		nil,
	)
	if err != nil {
		t.Fatalf("DiagnoseIncidentInScope() erro = %v", err)
	}

	if len(diagnosis.Suspects) < 2 {
		t.Fatalf("suspeitos = %d, esperado 2", len(diagnosis.Suspects))
	}
	if diagnosis.Suspects[0].Score != diagnosis.Suspects[1].Score {
		t.Fatalf("o teste depende de empate; scores = %.2f e %.2f",
			diagnosis.Suspects[0].Score, diagnosis.Suspects[1].Score)
	}
	if diagnosis.Suspects[0].ID != "payment-service" {
		t.Fatalf("primeiro suspeito = %q, esperado payment-service por estar mais fundo na cadeia",
			diagnosis.Suspects[0].ID)
	}
}

func cadeiaSignal(
	servico, trace, spanID, parentID string, indice int, timestamp time.Time, status int,
) domain.Signal {
	signal := diagnosisHTTPSignal(servico, timestamp, status, 20)
	signal.ID = servico + "-" + trace + "-" + spanID
	signal.ServiceName = servico
	signal.TraceID = trace
	signal.SpanID = spanID
	if parentID != "" {
		signal.Attributes["span.parent_id"] = parentID
	}
	return signal
}
