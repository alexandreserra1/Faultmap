package application

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	"github.com/faultmap/faultmap/internal/detection"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
	"github.com/faultmap/faultmap/internal/ranking"
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

// ListServicesSharingTracesWithAny não descobre nada além do primeiro nível
// neste fake: os testes que exercitam saltos usam scopeReaderNiveis.
func (fake *scopeReaderFake) ListServicesSharingTracesWithAny(
	_ context.Context, serviceNames []string, _ time.Time, _ time.Time, _ int,
) ([]string, error) {
	fake.chamadas++
	fake.escopoPedido = append([]string(nil), serviceNames...)
	return nil, nil
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
	deployments       []changedomain.Deployment
	mensagens         map[string]string
	consultas         int
	consultasDeCommit int
}

// ListCommitMessagesBySHA registra quantas vezes foi chamada, para que o teste
// possa exigir uma única busca para todo o escopo.
func (fake *scopedDeploymentReaderFake) ListCommitMessagesBySHA(
	_ context.Context, _ []string, _ int,
) (map[string]string, error) {
	fake.consultasDeCommit++
	return fake.mensagens, nil
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

type scopeReaderNiveis struct {
	// porOrigem devolve, para cada serviço consultado, os que dividem traces com ele.
	porOrigem map[string][]string
	rodadas   int
}

func (fake *scopeReaderNiveis) ListServicesSharingTraces(
	_ context.Context, entryService string, _ time.Time, _ time.Time, _ int,
) ([]string, int, error) {
	fake.rodadas++
	return fake.porOrigem[entryService], 7, nil
}

func (fake *scopeReaderNiveis) ListServicesSharingTracesWithAny(
	_ context.Context, serviceNames []string, _ time.Time, _ time.Time, _ int,
) ([]string, error) {
	fake.rodadas++
	descobertos := make([]string, 0)
	visto := make(map[string]struct{})
	for _, servico := range serviceNames {
		for _, vizinho := range fake.porOrigem[servico] {
			if _, repetido := visto[vizinho]; repetido {
				continue
			}
			visto[vizinho] = struct{}{}
			descobertos = append(descobertos, vizinho)
		}
	}
	return descobertos, nil
}

func (fake *scopeReaderNiveis) ListServicesInWindow(
	_ context.Context, _ time.Time, _ time.Time, _ int,
) ([]string, error) {
	return nil, nil
}

// TestDiagnoseScopeAlcançaSegundoSalto cobre o serviço que nunca aparece nos
// traces do serviço de entrada mas divide traces com um vizinho — como uma
// rotina que usa a mesma dependência do fluxo do usuário.
func TestDiagnoseScopeAlcançaSegundoSalto(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2026, time.August, 9, 10, 0, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(
		incidentStart, incidentStart.Add(time.Minute), time.Minute,
	)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}

	topologia := map[string][]string{
		"checkout-service": {"checkout-service", "payment-service"},
		"payment-service":  {"checkout-service", "payment-service", "ledger-service"},
		"ledger-service":   {"payment-service", "ledger-service"},
	}

	umSalto, err := DiagnoseIncidentInScope(context.Background(), ScopedDiagnosisRequest{
		EntryService: "checkout-service", Windows: windows, Limit: 500,
		MaxServices: 10, Depth: 1, Ranking: testRankingConfig(),
	}, &scopeReaderNiveis{porOrigem: topologia}, &scopedSignalReaderFake{}, nil)
	if err != nil {
		t.Fatalf("um salto erro = %v", err)
	}
	for _, servico := range umSalto.Scope.Services {
		if servico == "ledger-service" {
			t.Fatal("ledger não deveria ser alcançado a um salto")
		}
	}

	doisSaltos, err := DiagnoseIncidentInScope(context.Background(), ScopedDiagnosisRequest{
		EntryService: "checkout-service", Windows: windows, Limit: 500,
		MaxServices: 10, Depth: 2, Ranking: testRankingConfig(),
	}, &scopeReaderNiveis{porOrigem: topologia}, &scopedSignalReaderFake{}, nil)
	if err != nil {
		t.Fatalf("dois saltos erro = %v", err)
	}

	encontrouLedger := false
	for _, servico := range doisSaltos.Scope.Services {
		if servico == "ledger-service" {
			encontrouLedger = true
		}
	}
	if !encontrouLedger {
		t.Fatalf("escopo com dois saltos = %v, esperado conter ledger-service", doisSaltos.Scope.Services)
	}

	// A distância precisa ser registrada para que a saída possa explicá-la.
	if doisSaltos.Scope.Distances["checkout-service"] != 0 {
		t.Fatalf("distância da entrada = %d, esperado 0", doisSaltos.Scope.Distances["checkout-service"])
	}
	if doisSaltos.Scope.Distances["payment-service"] != 1 {
		t.Fatalf("distância do payment = %d, esperado 1", doisSaltos.Scope.Distances["payment-service"])
	}
	if doisSaltos.Scope.Distances["ledger-service"] != 2 {
		t.Fatalf("distância do ledger = %d, esperado 2", doisSaltos.Scope.Distances["ledger-service"])
	}
}

// TestDiagnoseScopeParaQuandoNãoHáNovosServiços evita rodadas inúteis de
// consulta quando a topologia já foi coberta antes de atingir a profundidade.
func TestDiagnoseScopeParaQuandoNãoHáNovosServiços(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2026, time.August, 9, 10, 0, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(
		incidentStart, incidentStart.Add(time.Minute), time.Minute,
	)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}

	// Topologia fechada: a partir do primeiro nível não há nada novo a descobrir.
	fake := &scopeReaderNiveis{porOrigem: map[string][]string{
		"a-service": {"a-service", "b-service"},
		"b-service": {"a-service", "b-service"},
	}}

	if _, err := DiagnoseIncidentInScope(context.Background(), ScopedDiagnosisRequest{
		EntryService: "a-service", Windows: windows, Limit: 500,
		MaxServices: 10, Depth: 5, Ranking: testRankingConfig(),
	}, fake, &scopedSignalReaderFake{}, nil); err != nil {
		t.Fatalf("DiagnoseIncidentInScope() erro = %v", err)
	}

	// Uma rodada para o primeiro nível e uma que não trouxe novidade; nunca cinco.
	if fake.rodadas > 2 {
		t.Fatalf("rodadas de descoberta = %d, esperado parar ao esgotar a topologia", fake.rodadas)
	}
}

// TestDiagnoseScopeAcusaOCommitImplantado cobre, no nível da aplicação, o
// caminho completo da mudança: o deployment é carregado, a mensagem do commit é
// buscada em lote, e o commit aparece no ranking como suspeito próprio, ao lado
// do serviço afetado.
func TestDiagnoseScopeAcusaOCommitImplantado(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(
		incidentStart, incidentStart.Add(time.Minute), time.Minute,
	)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}

	baseline := make([]domain.Signal, 0, 20)
	incidente := make([]domain.Signal, 0, 20)
	for indice := 0; indice < 20; indice++ {
		baseline = append(baseline, sinalComVersao("base", indice, incidentStart.Add(-time.Minute), 200, "1.0.0"))
		incidente = append(incidente, sinalComVersao("inc", indice, incidentStart, 500, "abc123def4567890"))
	}

	deployments := &scopedDeploymentReaderFake{
		deployments: []changedomain.Deployment{{
			ID: "deploy-1", ServiceName: "checkout-service", Environment: "producao",
			CommitSHA: "abc123def4567890", State: "success",
			DeployedAt: incidentStart.Add(-5 * time.Minute),
		}},
		mensagens: map[string]string{"abc123def4567890": "Reduce payment timeout"},
	}

	diagnosis, err := DiagnoseIncidentInScope(context.Background(), ScopedDiagnosisRequest{
		EntryService: "checkout-service", Environment: "producao", Windows: windows,
		Limit: 500, MaxServices: 10, Ranking: testRankingConfig(),
	},
		&scopeReaderFake{vizinhos: []string{"checkout-service"}, traceCount: 20},
		&scopedSignalReaderFake{baseline: baseline, incident: incidente},
		deployments,
	)
	if err != nil {
		t.Fatalf("DiagnoseIncidentInScope() erro = %v", err)
	}

	var commit *ranking.Suspect
	for indice := range diagnosis.Suspects {
		if diagnosis.Suspects[indice].Kind == detection.SubjectCommit {
			commit = &diagnosis.Suspects[indice]
		}
	}
	if commit == nil {
		t.Fatalf("o commit não apareceu no ranking: %+v", diagnosis.Suspects)
	}
	if commit.ID != "abc123def4567890" {
		t.Fatalf("identificador do commit = %q", commit.ID)
	}
	if !strings.Contains(commit.Label, "Reduce payment timeout") {
		t.Fatalf("rótulo do commit = %q, esperado conter a mensagem", commit.Label)
	}
	// Uma única busca de mensagens para todo o escopo, nunca uma por serviço.
	if deployments.consultasDeCommit != 1 {
		t.Fatalf("buscas de mensagem = %d, esperado 1", deployments.consultasDeCommit)
	}
}

// TestDiagnoseScopeRejeitaEntradaInválidaAntesDasConsultas preserva a cobertura
// que existia no caminho de serviço único: entrada inválida não pode chegar ao
// banco.
func TestDiagnoseScopeRejeitaEntradaInválidaAntesDasConsultas(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2026, time.August, 11, 10, 0, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(
		incidentStart, incidentStart.Add(time.Minute), time.Minute,
	)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}

	testCases := []struct {
		nome    string
		request ScopedDiagnosisRequest
	}{
		{
			nome: "limite de sinais inválido",
			request: ScopedDiagnosisRequest{
				EntryService: "checkout-service", Windows: windows,
				Limit: 0, MaxServices: 10, Ranking: testRankingConfig(),
			},
		},
		{
			nome: "pesos de ranking inválidos",
			request: ScopedDiagnosisRequest{
				EntryService: "checkout-service", Windows: windows,
				Limit: 500, MaxServices: 10,
				Ranking: ranking.Config{Weights: ranking.Weights{ErrorRateDelta: -1}, TopN: 3},
			},
		},
		{
			nome: "sem serviço de entrada",
			request: ScopedDiagnosisRequest{
				Windows: windows, Limit: 500, MaxServices: 10, Ranking: testRankingConfig(),
			},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.nome, func(t *testing.T) {
			t.Parallel()

			signals := &scopedSignalReaderFake{}
			if _, err := DiagnoseIncidentInScope(
				context.Background(), testCase.request,
				&scopeReaderFake{}, signals, nil,
			); err == nil {
				t.Fatal("erro = nil para entrada inválida")
			}
			if signals.consultas != 0 {
				t.Fatalf("consultas = %d, esperado nenhuma leitura com entrada inválida", signals.consultas)
			}
		})
	}
}

// sinalComVersao monta um span HTTP declarando a versão observada do serviço.
func sinalComVersao(prefixo string, indice int, instante time.Time, status int, versao string) domain.Signal {
	return domain.Signal{
		ID:          prefixo + "-" + versao + "-" + string(rune('a'+indice%26)),
		ServiceName: "checkout-service",
		Timestamp:   instante,
		TraceID:     "trace-" + string(rune('a'+indice%26)),
		Attributes: map[string]string{
			"http.response.status_code": strconv.Itoa(status),
			"service.version":           versao,
		},
		Measurements: map[string]float64{"duration_ms": 12},
	}
}
