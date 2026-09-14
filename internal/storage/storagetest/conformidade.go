// Package storagetest roda a mesma bateria de comportamento contra qualquer
// backend de armazenamento do Faultmap.
//
// Ela existe porque dois backends que divergem em silêncio são piores que um
// só: a investigação passaria a depender de onde o banco está, e a promessa de
// diagnóstico determinístico — mesma entrada, mesma acusação — valeria apenas
// para quem usasse o backend em que os testes rodaram. Aqui as duas
// implementações respondem às mesmas perguntas, e uma divergência aparece como
// falha em vez de aparecer em produção.
//
// O arquivo não termina em _test.go de propósito: ele precisa ser importável
// pelos testes de cada backend, que vivem em pacotes diferentes.
package storagetest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	"github.com/faultmap/faultmap/internal/detection"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
	"github.com/faultmap/faultmap/internal/ranking"
	telemetrydomain "github.com/faultmap/faultmap/internal/telemetry/domain"
)

// RepositorioDeSinais reúne o que a telemetria exige de um backend: as quatro
// interfaces que internal/application declara sobre sinais.
type RepositorioDeSinais interface {
	application.SignalStore
	application.SignalReader
	application.TraceSignalReader
	application.ScopedSignalReader
}

// RepositorioDeEscopo descobre quem participou do caminho da requisição.
type RepositorioDeEscopo interface {
	application.ScopeReader
}

// RepositorioDeMudancas cobre commits e deployments vindos do GitHub.
type RepositorioDeMudancas interface {
	application.ChangeWriter
	application.ScopedDeploymentReader
	ListDeployments(
		ctx context.Context, serviceName string, environment string,
		start time.Time, end time.Time, limit int,
	) ([]changedomain.Deployment, error)
}

// RepositorioDeCatalogo cobre a coleta de schema e a leitura das diferenças.
type RepositorioDeCatalogo interface {
	application.SchemaWriter
	application.ScopedSchemaChangeReader
}

// RepositorioDeDiagnostico cobre a escrita e a leitura de snapshots publicados.
type RepositorioDeDiagnostico interface {
	application.DiagnosisStore
	application.IncidentHistoryReader
}

// RepositorioDeRetencao cobre a limpeza de telemetria expirada e a liberação
// dos catálogos de schema.
type RepositorioDeRetencao interface {
	PruneSchemaCatalogsBefore(ctx context.Context, cutoff time.Time, limit int) (int, error)
	CountSchemaChanges(ctx context.Context) (int, error)
	CountIntactCatalogs(ctx context.Context, databaseName string) (int, error)
	application.SignalRetentionRemover
}

// Backend agrupa os repositórios de uma implementação já migrada e vazia.
type Backend struct {
	Sinais      RepositorioDeSinais
	Escopo      RepositorioDeEscopo
	Mudancas    RepositorioDeMudancas
	Catalogo    RepositorioDeCatalogo
	Diagnostico RepositorioDeDiagnostico
	Retencao    RepositorioDeRetencao
}

// Fabrica devolve um backend recém-migrado e vazio, isolado dos demais testes.
// Cada chamada precisa entregar um estado limpo: os casos abaixo assumem que
// nada foi gravado antes deles.
type Fabrica func(t *testing.T) Backend

// Instantes fixos. Datas literais evitam que o resultado do teste dependa do
// relógio da máquina — a mesma razão pela qual o produto normaliza tudo em UTC.
var (
	t0 = time.Date(2026, time.September, 10, 12, 0, 0, 0, time.UTC)
	t1 = t0.Add(1 * time.Minute)
	t2 = t0.Add(2 * time.Minute)
	t3 = t0.Add(3 * time.Minute)
	t4 = t0.Add(4 * time.Minute)
)

// RodarConformidade executa a bateria inteira contra o backend produzido pela
// fábrica. Os dois adaptadores chamam esta função e nada mais.
func RodarConformidade(t *testing.T, novo Fabrica) {
	t.Helper()

	t.Run("Sinais", func(t *testing.T) { rodarSinais(t, novo) })
	t.Run("Escopo", func(t *testing.T) { rodarEscopo(t, novo) })
	t.Run("Mudancas", func(t *testing.T) { rodarMudancas(t, novo) })
	t.Run("Catalogo", func(t *testing.T) { rodarCatalogo(t, novo) })
	t.Run("Diagnostico", func(t *testing.T) { rodarDiagnostico(t, novo) })
	t.Run("Retencao", func(t *testing.T) { rodarRetencao(t, novo) })
	t.Run("Concorrencia", func(t *testing.T) { rodarConcorrencia(t, novo) })
}

// ---------------------------------------------------------------- sinais ---

func rodarSinais(t *testing.T, novo Fabrica) {
	t.Run("SalvarEhIdempotentePeloID", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		sinais := []telemetrydomain.Signal{
			sinal("s-1", "checkout", t0, "trace-a"),
			sinal("s-2", "checkout", t1, "trace-a"),
		}

		inseridos, err := backend.Sinais.Save(ctx, sinais)
		if err != nil {
			t.Fatalf("Save() erro = %v", err)
		}
		if inseridos != 2 {
			t.Fatalf("Save() inseridos = %d, esperado 2", inseridos)
		}

		// O retry do Collector reenvia o mesmo lote. Ignorar o ID já gravado é o
		// que impede a mesma telemetria de contar duas vezes no denominador de
		// uma taxa de erro.
		reinseridos, err := backend.Sinais.Save(ctx, sinais)
		if err != nil {
			t.Fatalf("Save() repetido erro = %v", err)
		}
		if reinseridos != 0 {
			t.Fatalf("Save() repetido inseridos = %d, esperado 0", reinseridos)
		}
	})

	t.Run("SalvarLoteVazioNaoAbreTransacao", func(t *testing.T) {
		backend := novo(t)
		inseridos, err := backend.Sinais.Save(context.Background(), nil)
		if err != nil {
			t.Fatalf("Save() vazio erro = %v", err)
		}
		if inseridos != 0 {
			t.Fatalf("Save() vazio inseridos = %d, esperado 0", inseridos)
		}
	})

	t.Run("JanelaEhSemiabertaEOrdemEhEstavel", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		gravarSinais(t, backend, []telemetrydomain.Signal{
			// Fora por baixo: instante anterior ao início.
			sinal("s-antes", "checkout", t0.Add(-time.Second), "trace-a"),
			// Empate de instante: o desempate por ID é o que torna duas
			// execuções da mesma investigação comparáveis.
			sinal("s-b", "checkout", t1, "trace-a"),
			sinal("s-a", "checkout", t1, "trace-a"),
			sinal("s-c", "checkout", t2, "trace-a"),
			// Fora por cima: o fim da janela é exclusivo.
			sinal("s-depois", "checkout", t3, "trace-a"),
		})

		lidos, err := backend.Sinais.ListByServiceAndWindow(ctx, "checkout", t1, t3, 100)
		if err != nil {
			t.Fatalf("ListByServiceAndWindow() erro = %v", err)
		}
		exigirIDs(t, "ListByServiceAndWindow", lidos, "s-a", "s-b", "s-c")
	})

	t.Run("LimiteCortaSemQuebrarAOrdem", func(t *testing.T) {
		backend := novo(t)
		gravarSinais(t, backend, []telemetrydomain.Signal{
			sinal("s-3", "checkout", t2, "trace-a"),
			sinal("s-1", "checkout", t0, "trace-a"),
			sinal("s-2", "checkout", t1, "trace-a"),
		})

		lidos, err := backend.Sinais.ListByServiceAndWindow(context.Background(), "checkout", t0, t4, 2)
		if err != nil {
			t.Fatalf("ListByServiceAndWindow() erro = %v", err)
		}
		exigirIDs(t, "limite", lidos, "s-1", "s-2")
	})

	t.Run("AtributosEMedicoesSobrevivemAoIdaEVolta", func(t *testing.T) {
		backend := novo(t)
		original := sinal("s-1", "checkout", t0, "trace-a")
		original.Attributes = map[string]string{"http.route": "/checkout", "db.system": "postgresql"}
		original.Measurements = map[string]float64{"duration_ms": 1234.5}
		original.Severity = "ERROR"
		original.SpanID = "span-1"
		gravarSinais(t, backend, []telemetrydomain.Signal{original})

		lidos, err := backend.Sinais.ListByTraceID(context.Background(), "trace-a", 10)
		if err != nil {
			t.Fatalf("ListByTraceID() erro = %v", err)
		}
		if len(lidos) != 1 {
			t.Fatalf("ListByTraceID() devolveu %d sinais, esperado 1", len(lidos))
		}
		lido := lidos[0]
		if !reflect.DeepEqual(lido.Attributes, original.Attributes) {
			t.Errorf("atributos = %v, esperado %v", lido.Attributes, original.Attributes)
		}
		if !reflect.DeepEqual(lido.Measurements, original.Measurements) {
			t.Errorf("medições = %v, esperado %v", lido.Measurements, original.Measurements)
		}
		if lido.Type != telemetrydomain.SignalTypeSpan {
			t.Errorf("tipo = %q, esperado %q", lido.Type, telemetrydomain.SignalTypeSpan)
		}
		if lido.Severity != "ERROR" || lido.SpanID != "span-1" {
			t.Errorf("severidade/span = %q/%q, esperado ERROR/span-1", lido.Severity, lido.SpanID)
		}
		exigirInstanteUTC(t, "timestamp do sinal", lido.Timestamp, t0)
	})

	t.Run("SemAtributosOMapaVoltaVazioENaoNulo", func(t *testing.T) {
		backend := novo(t)
		cru := sinal("s-1", "checkout", t0, "trace-a")
		cru.Attributes = nil
		cru.Measurements = nil
		gravarSinais(t, backend, []telemetrydomain.Signal{cru})

		lidos, err := backend.Sinais.ListByTraceID(context.Background(), "trace-a", 10)
		if err != nil {
			t.Fatalf("ListByTraceID() erro = %v", err)
		}
		if len(lidos) != 1 {
			t.Fatalf("ListByTraceID() devolveu %d sinais, esperado 1", len(lidos))
		}
		if lidos[0].Attributes == nil || len(lidos[0].Attributes) != 0 {
			t.Errorf("atributos = %v, esperado mapa vazio não nulo", lidos[0].Attributes)
		}
		if lidos[0].Measurements == nil || len(lidos[0].Measurements) != 0 {
			t.Errorf("medições = %v, esperado mapa vazio não nulo", lidos[0].Measurements)
		}
	})

	t.Run("EscopoLeTodosOsServicosEmUmaConsulta", func(t *testing.T) {
		backend := novo(t)
		gravarSinais(t, backend, []telemetrydomain.Signal{
			sinal("s-1", "checkout", t0, "trace-a"),
			sinal("s-2", "payments", t1, "trace-a"),
			sinal("s-3", "inventory", t2, "trace-a"),
			sinal("s-4", "nao-investigado", t1, "trace-b"),
		})

		lidos, err := backend.Sinais.ListByServicesAndWindow(
			context.Background(), []string{"checkout", "payments"}, t0, t4, 100)
		if err != nil {
			t.Fatalf("ListByServicesAndWindow() erro = %v", err)
		}
		exigirIDs(t, "escopo", lidos, "s-1", "s-2")
	})

	t.Run("RecusasDeEntradaSaoIguaisNosDoisBackends", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		casos := []struct {
			nome     string
			executar func() error
		}{
			{"limite zero por serviço", func() error {
				_, err := backend.Sinais.ListByServiceAndWindow(ctx, "checkout", t0, t4, 0)
				return err
			}},
			{"janela invertida", func() error {
				_, err := backend.Sinais.ListByServiceAndWindow(ctx, "checkout", t4, t0, 10)
				return err
			}},
			{"trace vazio", func() error {
				_, err := backend.Sinais.ListByTraceID(ctx, "  ", 10)
				return err
			}},
			{"limite zero por trace", func() error {
				_, err := backend.Sinais.ListByTraceID(ctx, "trace-a", 0)
				return err
			}},
			{"escopo sem serviços", func() error {
				_, err := backend.Sinais.ListByServicesAndWindow(ctx, nil, t0, t4, 10)
				return err
			}},
			{"escopo grande demais", func() error {
				_, err := backend.Sinais.ListByServicesAndWindow(ctx, nomesDeServico(51), t0, t4, 10)
				return err
			}},
		}
		for _, caso := range casos {
			if err := caso.executar(); err == nil {
				t.Errorf("%s: erro = nil, esperado recusa", caso.nome)
			}
		}
	})
}

// ---------------------------------------------------------------- escopo ---

func rodarEscopo(t *testing.T, novo Fabrica) {
	t.Run("ExpandePelosTracesQueAtravessaramOServico", func(t *testing.T) {
		backend := novo(t)
		gravarSinais(t, backend, []telemetrydomain.Signal{
			sinal("s-1", "checkout", t0, "trace-a"),
			sinal("s-2", "payments", t1, "trace-a"),
			sinal("s-3", "inventory", t2, "trace-a"),
			// Outro trace, sem o serviço de entrada: não deve entrar no escopo.
			sinal("s-4", "relatorios", t1, "trace-b"),
		})

		servicos, traces, err := backend.Escopo.ListServicesSharingTraces(
			context.Background(), "checkout", t0, t4, 50)
		if err != nil {
			t.Fatalf("ListServicesSharingTraces() erro = %v", err)
		}
		exigirIguais(t, "escopo", servicos, []string{"checkout", "inventory", "payments"})
		if traces != 1 {
			t.Fatalf("traces considerados = %d, esperado 1", traces)
		}
	})

	t.Run("SemTracesOServicoDeEntradaSegueSozinho", func(t *testing.T) {
		backend := novo(t)
		gravarSinais(t, backend, []telemetrydomain.Signal{sinal("s-1", "checkout", t0, "")})

		servicos, traces, err := backend.Escopo.ListServicesSharingTraces(
			context.Background(), "checkout", t0, t4, 50)
		if err != nil {
			t.Fatalf("ListServicesSharingTraces() erro = %v", err)
		}
		exigirIguais(t, "escopo sem traces", servicos, []string{"checkout"})
		if traces != 0 {
			t.Fatalf("traces = %d, esperado 0", traces)
		}
	})

	t.Run("VarreduraEnumeraServicosDaJanela", func(t *testing.T) {
		backend := novo(t)
		gravarSinais(t, backend, []telemetrydomain.Signal{
			sinal("s-1", "payments", t1, "trace-a"),
			sinal("s-2", "checkout", t1, "trace-a"),
			sinal("s-3", "fora-da-janela", t4, "trace-c"),
		})

		servicos, err := backend.Escopo.ListServicesInWindow(context.Background(), t0, t3, 50)
		if err != nil {
			t.Fatalf("ListServicesInWindow() erro = %v", err)
		}
		exigirIguais(t, "varredura", servicos, []string{"checkout", "payments"})
	})

	t.Run("SaltoAdjacenteAlcancaQuemDivideTraceComOEscopo", func(t *testing.T) {
		backend := novo(t)
		gravarSinais(t, backend, []telemetrydomain.Signal{
			sinal("s-1", "checkout", t0, "trace-a"),
			sinal("s-2", "payments", t1, "trace-a"),
			// A rotina nunca aparece nos traces do checkout, mas divide um trace
			// com payments: é exatamente o caso que o segundo nível existe para
			// alcançar.
			sinal("s-3", "payments", t1, "trace-b"),
			sinal("s-4", "rotina-noturna", t2, "trace-b"),
		})

		servicos, err := backend.Escopo.ListServicesSharingTracesWithAny(
			context.Background(), []string{"payments"}, t0, t4, 50)
		if err != nil {
			t.Fatalf("ListServicesSharingTracesWithAny() erro = %v", err)
		}
		exigirIguais(t, "escopo adjacente", servicos, []string{"checkout", "payments", "rotina-noturna"})
	})

	t.Run("RecusasDeEntradaSaoIguaisNosDoisBackends", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		if _, _, err := backend.Escopo.ListServicesSharingTraces(ctx, "  ", t0, t4, 10); err == nil {
			t.Error("serviço de entrada vazio: erro = nil, esperado recusa")
		}
		if _, _, err := backend.Escopo.ListServicesSharingTraces(ctx, "checkout", t0, t4, 0); err == nil {
			t.Error("limite zero: erro = nil, esperado recusa")
		}
		if _, _, err := backend.Escopo.ListServicesSharingTraces(ctx, "checkout", t4, t0, 10); err == nil {
			t.Error("janela invertida: erro = nil, esperado recusa")
		}
		if _, err := backend.Escopo.ListServicesInWindow(ctx, t0, t4, 0); err == nil {
			t.Error("varredura com limite zero: erro = nil, esperado recusa")
		}
		if _, err := backend.Escopo.ListServicesSharingTracesWithAny(ctx, nil, t0, t4, 10); err == nil {
			t.Error("salto sem origem: erro = nil, esperado recusa")
		}
	})
}

// -------------------------------------------------------------- mudanças ---

func rodarMudancas(t *testing.T, novo Fabrica) {
	t.Run("SalvarEhIdempotentePorSHAEPorID", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		snapshot := snapshotDeMudancas()

		primeiro, err := backend.Mudancas.SaveChanges(ctx, snapshot)
		if err != nil {
			t.Fatalf("SaveChanges() erro = %v", err)
		}
		if primeiro.CommitsPersisted != 2 || primeiro.DeploymentsPersisted != 2 {
			t.Fatalf("persistidos = %d commits / %d deployments, esperado 2/2",
				primeiro.CommitsPersisted, primeiro.DeploymentsPersisted)
		}

		segundo, err := backend.Mudancas.SaveChanges(ctx, snapshot)
		if err != nil {
			t.Fatalf("SaveChanges() repetido erro = %v", err)
		}
		if segundo.CommitsPersisted != 0 || segundo.DeploymentsPersisted != 0 {
			t.Fatalf("repetição persistiu = %d commits / %d deployments, esperado 0/0",
				segundo.CommitsPersisted, segundo.DeploymentsPersisted)
		}
	})

	t.Run("DeploymentsVoltamDoMaisRecenteParaOMaisAntigo", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		gravarMudancas(t, backend, snapshotDeMudancas())

		deployments, err := backend.Mudancas.ListDeployments(ctx, "checkout", "staging", t0, t4, 10)
		if err != nil {
			t.Fatalf("ListDeployments() erro = %v", err)
		}
		exigirIDsDeDeployment(t, "ordem descendente", deployments, "deploy-2", "deploy-1")
		// Ref, Task e State vivem no JSON de metadata; o ida e volta prova que o
		// detector continua recebendo o que o renderizador exibe.
		if deployments[0].Ref != "main" || deployments[0].State != "success" {
			t.Errorf("metadata = ref %q / state %q, esperado main/success",
				deployments[0].Ref, deployments[0].State)
		}
		exigirInstanteUTC(t, "deployed_at", deployments[0].DeployedAt, t2)
	})

	t.Run("JanelaDeDeploymentEhSemiaberta", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		gravarMudancas(t, backend, snapshotDeMudancas())

		// deploy-1 em t1 entra pelo início inclusivo; deploy-2 em t2 fica fora
		// pelo fim exclusivo.
		deployments, err := backend.Mudancas.ListDeployments(ctx, "checkout", "staging", t1, t2, 10)
		if err != nil {
			t.Fatalf("ListDeployments() erro = %v", err)
		}
		exigirIDsDeDeployment(t, "janela semiaberta", deployments, "deploy-1")
	})

	t.Run("EscopoLeDeploymentsDeTodosOsServicosEmUmaConsulta", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		gravarMudancas(t, backend, snapshotDeMudancas())

		deployments, err := backend.Mudancas.ListDeploymentsForServices(
			ctx, []string{"checkout", "payments"}, "staging", t0, t4, 10)
		if err != nil {
			t.Fatalf("ListDeploymentsForServices() erro = %v", err)
		}
		exigirIDsDeDeployment(t, "escopo", deployments, "deploy-2", "deploy-1")
	})

	t.Run("MensagensDeCommitVoltamEmLote", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		gravarMudancas(t, backend, snapshotDeMudancas())

		// O SHA inexistente e o vazio são o caso real: o ranking pergunta pelo
		// que os deployments citam, e nem todo commit citado foi coletado.
		mensagens, err := backend.Mudancas.ListCommitMessagesBySHA(
			ctx, []string{"sha-1", "  ", "sha-ausente", "sha-2"}, 10)
		if err != nil {
			t.Fatalf("ListCommitMessagesBySHA() erro = %v", err)
		}
		esperado := map[string]string{
			"sha-1": "aumentar o timeout do pool",
			"sha-2": "remover índice de created_at",
		}
		if !reflect.DeepEqual(mensagens, esperado) {
			t.Errorf("mensagens = %v, esperado %v", mensagens, esperado)
		}
	})

	t.Run("ListaVaziaDeSHANaoVaiAoBanco", func(t *testing.T) {
		backend := novo(t)
		mensagens, err := backend.Mudancas.ListCommitMessagesBySHA(context.Background(), nil, 10)
		if err != nil {
			t.Fatalf("ListCommitMessagesBySHA() vazio erro = %v", err)
		}
		if len(mensagens) != 0 {
			t.Errorf("mensagens = %v, esperado vazio", mensagens)
		}
	})

	t.Run("RecusasDeEntradaSaoIguaisNosDoisBackends", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		if _, err := backend.Mudancas.ListDeployments(ctx, "", "staging", t0, t4, 10); err == nil {
			t.Error("serviço vazio: erro = nil, esperado recusa")
		}
		if _, err := backend.Mudancas.ListDeployments(ctx, "checkout", "staging", t4, t0, 10); err == nil {
			t.Error("janela invertida: erro = nil, esperado recusa")
		}
		if _, err := backend.Mudancas.ListDeployments(ctx, "checkout", "staging", t0, t4, 0); err == nil {
			t.Error("limite zero: erro = nil, esperado recusa")
		}
		if _, err := backend.Mudancas.ListDeployments(ctx, "checkout", "staging", t0, t4, 100_000); err == nil {
			t.Error("limite acima do teto: erro = nil, esperado recusa")
		}
		if _, err := backend.Mudancas.ListDeploymentsForServices(ctx, nil, "staging", t0, t4, 10); err == nil {
			t.Error("escopo vazio: erro = nil, esperado recusa")
		}
	})
}

// -------------------------------------------------------------- catálogo ---

func rodarCatalogo(t *testing.T, novo Fabrica) {
	t.Run("PrimeiraColetaNaoAcusaOSchemaInteiroComoNovo", func(t *testing.T) {
		backend := novo(t)
		resultado, err := backend.Catalogo.SaveSnapshot(context.Background(), coleta("snap-1", t0,
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "integer"),
		))
		if err != nil {
			t.Fatalf("SaveSnapshot() erro = %v", err)
		}
		if resultado.ObjectsCollected != 1 {
			t.Errorf("objetos = %d, esperado 1", resultado.ObjectsCollected)
		}
		if resultado.ChangesPersisted != 0 {
			t.Errorf("mudanças = %d, esperado 0 na linha de base", resultado.ChangesPersisted)
		}
	})

	t.Run("SegundaColetaProduzAsDiferencas", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		gravarColeta(t, backend, coleta("snap-1", t0,
			objeto(changedomain.SchemaObjectIndex, "idx_payments_created_at", "payments", ""),
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "integer"),
		))

		resultado, err := backend.Catalogo.SaveSnapshot(ctx, coleta("snap-2", t1,
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "bigint"),
		))
		if err != nil {
			t.Fatalf("SaveSnapshot() erro = %v", err)
		}
		if resultado.ChangesPersisted != 2 {
			t.Fatalf("mudanças = %d, esperado 2 (índice removido, tipo alterado)", resultado.ChangesPersisted)
		}

		mudancas, err := backend.Catalogo.ListSchemaChangesForScope(
			ctx, []string{"payments"}, nil, t0, t4, 100)
		if err != nil {
			t.Fatalf("ListSchemaChangesForScope() erro = %v", err)
		}
		if len(mudancas) != 2 {
			t.Fatalf("mudanças lidas = %d, esperado 2", len(mudancas))
		}
		for _, mudanca := range mudancas {
			exigirInstanteUTC(t, "observed_after", mudanca.ObservedAfter, t0)
			exigirInstanteUTC(t, "observed_before", mudanca.ObservedBefore, t1)
			if mudanca.TableName != "payments" {
				t.Errorf("tabela = %q, esperado payments", mudanca.TableName)
			}
		}
	})

	t.Run("MudancasSaoAlcancaveisPelaTabelaQuandoABaseNaoEhNomeada", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		gravarColeta(t, backend, coleta("snap-1", t0,
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "integer"),
		))
		gravarColeta(t, backend, coleta("snap-2", t1,
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "bigint"),
		))

		// A telemetria real traz db.collection.name e quase nunca db.namespace:
		// perguntar só pela tabela precisa alcançar a mesma mudança.
		porTabela, err := backend.Catalogo.ListSchemaChangesForScope(ctx, nil, []string{"payments"}, t0, t4, 100)
		if err != nil {
			t.Fatalf("ListSchemaChangesForScope() por tabela erro = %v", err)
		}
		if len(porTabela) != 1 {
			t.Fatalf("mudanças por tabela = %d, esperado 1", len(porTabela))
		}

		// Base e tabela na mesma consulta não podem duplicar a mesma linha.
		porAmbos, err := backend.Catalogo.ListSchemaChangesForScope(
			ctx, []string{"payments"}, []string{"payments"}, t0, t4, 100)
		if err != nil {
			t.Fatalf("ListSchemaChangesForScope() por ambos erro = %v", err)
		}
		if len(porAmbos) != 1 {
			t.Fatalf("mudanças por base e tabela = %d, esperado 1 sem duplicar", len(porAmbos))
		}
	})

	t.Run("EscopoVazioNaoConsultaNemFalha", func(t *testing.T) {
		backend := novo(t)
		mudancas, err := backend.Catalogo.ListSchemaChangesForScope(
			context.Background(), []string{"  "}, nil, t0, t4, 100)
		if err != nil {
			t.Fatalf("ListSchemaChangesForScope() vazio erro = %v", err)
		}
		if len(mudancas) != 0 {
			t.Errorf("mudanças = %d, esperado 0", len(mudancas))
		}
	})

	t.Run("ColetaRepetidaNaoDuplicaMudancas", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		gravarColeta(t, backend, coleta("snap-1", t0,
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "integer"),
		))
		segunda := coleta("snap-2", t1,
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "bigint"),
		)
		gravarColeta(t, backend, segunda)

		// Reenviar a mesma coleta é o retry de `ingest schema`. O ON CONFLICT
		// precisa absorvê-lo sem inflar a contagem de mudanças observadas.
		if _, err := backend.Catalogo.SaveSnapshot(ctx, segunda); err != nil {
			t.Fatalf("SaveSnapshot() repetido erro = %v", err)
		}
		mudancas, err := backend.Catalogo.ListSchemaChangesForScope(ctx, []string{"payments"}, nil, t0, t4, 100)
		if err != nil {
			t.Fatalf("ListSchemaChangesForScope() erro = %v", err)
		}
		if len(mudancas) != 1 {
			t.Fatalf("mudanças = %d, esperado 1 após repetir a coleta", len(mudancas))
		}
	})

	t.Run("ColetaInvalidaEhRecusadaAntesDeQualquerEscrita", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		invalidas := map[string]changedomain.SchemaSnapshot{
			"sem ID":       {DatabaseName: "payments", CapturedAt: t0},
			"sem base":     {ID: "snap-1", CapturedAt: t0},
			"sem instante": {ID: "snap-1", DatabaseName: "payments"},
		}
		for nome, invalida := range invalidas {
			if _, err := backend.Catalogo.SaveSnapshot(ctx, invalida); err == nil {
				t.Errorf("%s: erro = nil, esperado recusa", nome)
			}
		}
	})
}

// ----------------------------------------------------------- diagnóstico ---

func rodarDiagnostico(t *testing.T, novo Fabrica) {
	t.Run("SnapshotEhImutavelPeloID", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		diagnostico := diagnosticoDeTeste(t, "incident-1")

		gravado, err := backend.Diagnostico.Save(ctx, diagnostico)
		if err != nil {
			t.Fatalf("Save() erro = %v", err)
		}
		if !gravado {
			t.Fatal("Save() gravado = false, esperado true na primeira vez")
		}

		// Repetir a investigação com as mesmas janelas produz o mesmo ID. O
		// snapshot original não pode ser substituído: ele é o registro auditável
		// do que foi publicado.
		regravado, err := backend.Diagnostico.Save(ctx, diagnostico)
		if err != nil {
			t.Fatalf("Save() repetido erro = %v", err)
		}
		if regravado {
			t.Fatal("Save() repetido gravado = true, esperado false")
		}
	})

	t.Run("LeituraDevolveIncidenteFindingsERanking", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		original := diagnosticoDeTeste(t, "incident-1")
		gravarDiagnostico(t, backend, original)

		lido, err := backend.Diagnostico.Get(ctx, "incident-1")
		if err != nil {
			t.Fatalf("Get() erro = %v", err)
		}
		if lido.Incident.ID != "incident-1" || lido.Incident.ServiceName != "checkout-service" {
			t.Errorf("incidente = %q/%q, esperado incident-1/checkout-service",
				lido.Incident.ID, lido.Incident.ServiceName)
		}
		if lido.Incident.Status != "diagnosed" {
			t.Errorf("status = %q, esperado diagnosed", lido.Incident.Status)
		}
		// Findings voltam ordenados por regra: é a ordem que o relatório imprime,
		// e ela não pode depender do plano que o banco escolheu.
		if len(lido.Findings) != 2 {
			t.Fatalf("findings = %d, esperado 2", len(lido.Findings))
		}
		if lido.Findings[0].Rule != detection.RuleDatabaseTimeout {
			t.Errorf("primeiro finding = %q, esperado %q", lido.Findings[0].Rule, detection.RuleDatabaseTimeout)
		}
		if len(lido.Findings[0].Evidence) != 1 || lido.Findings[0].Evidence[0].IncidentValue != 0.3 {
			t.Errorf("evidência do primeiro finding = %v, esperado valor 0.3", lido.Findings[0].Evidence)
		}
		if len(lido.Findings[0].Limitations) != 1 {
			t.Errorf("limitações = %v, esperado uma", lido.Findings[0].Limitations)
		}
		if len(lido.Suspects) != 1 || lido.Suspects[0].ID != "checkout-service" {
			t.Errorf("suspeitos = %v, esperado um suspeito checkout-service", lido.Suspects)
		}
		if lido.BaselineSignalCount == nil || *lido.BaselineSignalCount != 40 {
			t.Errorf("baseline = %v, esperado 40", lido.BaselineSignalCount)
		}
		if lido.BaselineStart == nil {
			t.Fatal("baseline_start = nil, esperado o instante gravado")
		}
		exigirInstanteUTC(t, "baseline_start", *lido.BaselineStart, original.Windows.Baseline.Start.UTC())
	})

	t.Run("IncidenteAusenteDevolveErroSentinela", func(t *testing.T) {
		backend := novo(t)
		_, err := backend.Diagnostico.Get(context.Background(), "nao-existe")
		if !errors.Is(err, application.ErrIncidentNotFound) {
			t.Fatalf("Get() erro = %v, esperado ErrIncidentNotFound", err)
		}
	})

	t.Run("ListaVemDoMaisRecenteParaOMaisAntigo", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		antigo := diagnosticoDeTeste(t, "incident-antigo")
		recente := diagnosticoDeTeste(t, "incident-recente")
		recente.Windows = janela(t, t2)
		gravarDiagnostico(t, backend, antigo)
		gravarDiagnostico(t, backend, recente)

		incidentes, err := backend.Diagnostico.List(ctx, 10)
		if err != nil {
			t.Fatalf("List() erro = %v", err)
		}
		if len(incidentes) != 2 {
			t.Fatalf("incidentes = %d, esperado 2", len(incidentes))
		}
		if incidentes[0].ID != "incident-recente" || incidentes[1].ID != "incident-antigo" {
			t.Errorf("ordem = %q, %q; esperado recente antes de antigo",
				incidentes[0].ID, incidentes[1].ID)
		}
		exigirInstanteUTC(t, "started_at", incidentes[0].IncidentStart, t2)
	})

	t.Run("RecusasDeEntradaSaoIguaisNosDoisBackends", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		if _, err := backend.Diagnostico.List(ctx, 0); err == nil {
			t.Error("limite zero: erro = nil, esperado recusa")
		}
		if _, err := backend.Diagnostico.List(ctx, 100_000); err == nil {
			t.Error("limite acima do teto: erro = nil, esperado recusa")
		}
		if _, err := backend.Diagnostico.Get(ctx, "   "); err == nil {
			t.Error("ID vazio: erro = nil, esperado recusa")
		}
		if _, err := backend.Diagnostico.Save(ctx, application.Diagnosis{}); err == nil {
			t.Error("diagnóstico sem ID: erro = nil, esperado recusa")
		}
	})
}

// -------------------------------------------------------------- retenção ---

func rodarRetencao(t *testing.T, novo Fabrica) {
	t.Run("RemoveOsMaisAntigosPrimeiroERespeitaOLote", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		gravarSinais(t, backend, []telemetrydomain.Signal{
			sinal("s-1", "checkout", t0, "trace-a"),
			sinal("s-2", "checkout", t1, "trace-a"),
			sinal("s-3", "checkout", t2, "trace-a"),
		})

		removidos, err := backend.Retencao.DeleteSignalsBefore(ctx, t3, 2)
		if err != nil {
			t.Fatalf("DeleteSignalsBefore() erro = %v", err)
		}
		if removidos != 2 {
			t.Fatalf("removidos = %d, esperado 2", removidos)
		}
		restantes, err := backend.Sinais.ListByServiceAndWindow(ctx, "checkout", t0, t4, 100)
		if err != nil {
			t.Fatalf("ListByServiceAndWindow() erro = %v", err)
		}
		exigirIDs(t, "após retenção", restantes, "s-3")
	})

	t.Run("RepetirAposEsgotarDevolveZero", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		gravarSinais(t, backend, []telemetrydomain.Signal{sinal("s-1", "checkout", t0, "trace-a")})

		if _, err := backend.Retencao.DeleteSignalsBefore(ctx, t2, 100); err != nil {
			t.Fatalf("DeleteSignalsBefore() erro = %v", err)
		}
		removidos, err := backend.Retencao.DeleteSignalsBefore(ctx, t2, 100)
		if err != nil {
			t.Fatalf("DeleteSignalsBefore() repetido erro = %v", err)
		}
		if removidos != 0 {
			t.Fatalf("removidos = %d, esperado 0 quando nada expirou", removidos)
		}
	})

	t.Run("CorteEhExclusivoNoInstanteExato", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()
		gravarSinais(t, backend, []telemetrydomain.Signal{sinal("s-1", "checkout", t1, "trace-a")})

		removidos, err := backend.Retencao.DeleteSignalsBefore(ctx, t1, 100)
		if err != nil {
			t.Fatalf("DeleteSignalsBefore() erro = %v", err)
		}
		if removidos != 0 {
			t.Fatalf("removidos = %d, esperado 0: o corte é estritamente anterior", removidos)
		}
	})

	t.Run("PreservaSnapshotsDeDiagnosticoEOCatalogo", func(t *testing.T) {
		// ADR 0003: DeleteSignalsBefore alcança somente `signals`. Um postmortem
		// consultado depois da janela de retenção continua legível.
		//
		// O catálogo tem política própria desde a ADR 0015, mas por outro método
		// — PruneSchemaCatalogsBefore, coberto logo abaixo. A limpeza de
		// telemetria não pode alcançá-lo de passagem.
		backend := novo(t)
		ctx := context.Background()
		gravarSinais(t, backend, []telemetrydomain.Signal{sinal("s-1", "checkout", t0, "trace-a")})
		gravarDiagnostico(t, backend, diagnosticoDeTeste(t, "incident-1"))
		gravarColeta(t, backend, coleta("snap-1", t0,
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "integer"),
		))
		gravarColeta(t, backend, coleta("snap-2", t1,
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "bigint"),
		))

		if _, err := backend.Retencao.DeleteSignalsBefore(ctx, t4, 100); err != nil {
			t.Fatalf("DeleteSignalsBefore() erro = %v", err)
		}

		if _, err := backend.Diagnostico.Get(ctx, "incident-1"); err != nil {
			t.Errorf("Get() após retenção erro = %v, esperado snapshot preservado", err)
		}
		mudancas, err := backend.Catalogo.ListSchemaChangesForScope(ctx, []string{"payments"}, nil, t0, t4, 100)
		if err != nil {
			t.Fatalf("ListSchemaChangesForScope() após retenção erro = %v", err)
		}
		if len(mudancas) != 1 {
			t.Errorf("mudanças de schema = %d, esperado 1 preservada", len(mudancas))
		}
	})

	t.Run("LiberaCatalogoExpiradoSemApagarAsMudancas", func(t *testing.T) {
		// ADR 0015: o que cresce é o JSON do catálogo — numa base com milhares de
		// objetos passa de meio megabyte por coleta, e a pontuação pessimista da
		// proximidade cobra coleta frequente. As mudanças derivadas são
		// minúsculas e sustentam evidência de diagnósticos já gravados.
		backend := novo(t)
		ctx := context.Background()
		// São três coletas de propósito. A segunda é a que importa: ela produz
		// mudanças E é velha o bastante para ser liberada, enquanto a terceira
		// é a linha de base protegida.
		//
		// Com só duas coletas o caso passava mesmo trocando o UPDATE por um
		// DELETE: as únicas mudanças pertenciam à coleta mais recente, que nunca
		// é liberada, então o ON DELETE CASCADE jamais disparava. O teste
		// afirmava proteger a evidência sem nunca exercitar o caminho que a
		// ameaça.
		gravarColeta(t, backend, coleta("snap-1", t0,
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "integer"),
		))
		gravarColeta(t, backend, coleta("snap-2", t1,
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "bigint"),
		))
		gravarColeta(t, backend, coleta("snap-3", t2,
			objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "text"),
		))

		antes, err := backend.Retencao.CountSchemaChanges(ctx)
		if err != nil {
			t.Fatalf("CountSchemaChanges() erro = %v", err)
		}
		if antes == 0 {
			t.Fatal("o caso precisa de mudanças gravadas para provar que sobrevivem")
		}

		// O corte alcança snap-1 e snap-2; snap-3 é a linha de base preservada.
		liberados, err := backend.Retencao.PruneSchemaCatalogsBefore(ctx, t3, 100)
		if err != nil {
			t.Fatalf("PruneSchemaCatalogsBefore() erro = %v", err)
		}
		if liberados == 0 {
			t.Fatal("nenhum catálogo liberado; a política não teve efeito")
		}

		depois, err := backend.Retencao.CountSchemaChanges(ctx)
		if err != nil {
			t.Fatalf("CountSchemaChanges() erro = %v", err)
		}
		if depois != antes {
			t.Fatalf("mudanças = %d após a liberação, esperado %d: a evidência foi apagada junto", depois, antes)
		}
	})

	t.Run("PreservaAColetaMaisRecenteDeCadaBase", func(t *testing.T) {
		// A coleta mais recente é a linha de base da próxima comparação.
		// Esvaziá-la faria o diff seguinte enxergar catálogo vazio e reportar
		// todo objeto da base como recém-criado — uma migração inventada em cada
		// tabela, no próximo incidente.
		backend := novo(t)
		ctx := context.Background()
		for indice, instante := range []time.Time{t0, t1, t2} {
			gravarColeta(t, backend, coleta(
				"snap-"+strconv.Itoa(indice), instante,
				objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "integer"),
			))
		}

		// Corte muito à frente: sem a regra, TUDO seria esvaziado.
		if _, err := backend.Retencao.PruneSchemaCatalogsBefore(ctx, t4.AddDate(1, 0, 0), 100); err != nil {
			t.Fatalf("PruneSchemaCatalogsBefore() erro = %v", err)
		}
		intactos, err := backend.Retencao.CountIntactCatalogs(ctx, "payments")
		if err != nil {
			t.Fatalf("CountIntactCatalogs() erro = %v", err)
		}
		if intactos != 1 {
			t.Fatalf("catálogos íntegros = %d, esperado exatamente 1 (o mais recente)", intactos)
		}
	})

	t.Run("LiberacaoDeCatalogoEhIdempotente", func(t *testing.T) {
		// Repetir a limpeza não pode contar de novo o que já foi liberado, senão
		// o relatório do comando mentiria a cada execução.
		backend := novo(t)
		ctx := context.Background()
		for indice, instante := range []time.Time{t0, t1, t2} {
			gravarColeta(t, backend, coleta(
				"snap-"+strconv.Itoa(indice), instante,
				objeto(changedomain.SchemaObjectColumn, "payments.amount", "payments", "integer"),
			))
		}

		corte := t4.AddDate(1, 0, 0)
		if _, err := backend.Retencao.PruneSchemaCatalogsBefore(ctx, corte, 100); err != nil {
			t.Fatalf("primeira liberação erro = %v", err)
		}
		repetida, err := backend.Retencao.PruneSchemaCatalogsBefore(ctx, corte, 100)
		if err != nil {
			t.Fatalf("segunda liberação erro = %v", err)
		}
		if repetida != 0 {
			t.Fatalf("a repetição liberou %d catálogos, esperado 0", repetida)
		}
	})

	t.Run("LoteInvalidoEhRecusado", func(t *testing.T) {
		backend := novo(t)
		if _, err := backend.Retencao.DeleteSignalsBefore(context.Background(), t4, 0); err == nil {
			t.Error("lote zero: erro = nil, esperado recusa")
		}
	})
}

// ----------------------------------------------------------------- apoio ---

func sinal(id, servico string, instante time.Time, traceID string) telemetrydomain.Signal {
	return telemetrydomain.Signal{
		ID:           id,
		Type:         telemetrydomain.SignalTypeSpan,
		ServiceName:  servico,
		Timestamp:    instante,
		TraceID:      traceID,
		SpanID:       "span-" + id,
		Attributes:   map[string]string{},
		Measurements: map[string]float64{},
	}
}

func nomesDeServico(quantidade int) []string {
	nomes := make([]string, 0, quantidade)
	for indice := 0; indice < quantidade; indice++ {
		nomes = append(nomes, fmt.Sprintf("servico-%02d", indice))
	}
	return nomes
}

func gravarSinais(t *testing.T, backend Backend, sinais []telemetrydomain.Signal) {
	t.Helper()
	if _, err := backend.Sinais.Save(context.Background(), sinais); err != nil {
		t.Fatalf("preparar sinais: %v", err)
	}
}

func gravarMudancas(t *testing.T, backend Backend, snapshot changedomain.Snapshot) {
	t.Helper()
	if _, err := backend.Mudancas.SaveChanges(context.Background(), snapshot); err != nil {
		t.Fatalf("preparar mudanças: %v", err)
	}
}

func gravarColeta(t *testing.T, backend Backend, snapshot changedomain.SchemaSnapshot) {
	t.Helper()
	if _, err := backend.Catalogo.SaveSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("preparar coleta %q: %v", snapshot.ID, err)
	}
}

func gravarDiagnostico(t *testing.T, backend Backend, diagnostico application.Diagnosis) {
	t.Helper()
	if _, err := backend.Diagnostico.Save(context.Background(), diagnostico); err != nil {
		t.Fatalf("preparar diagnóstico %q: %v", diagnostico.ID, err)
	}
}

func coleta(id string, capturadaEm time.Time, objetos ...changedomain.SchemaObject) changedomain.SchemaSnapshot {
	return changedomain.SchemaSnapshot{
		ID:           id,
		DatabaseName: "payments",
		CapturedAt:   capturadaEm,
		Objects:      objetos,
	}
}

func objeto(kind changedomain.SchemaObjectKind, nome, tabela, detalhe string) changedomain.SchemaObject {
	return changedomain.SchemaObject{Kind: kind, Name: nome, TableName: tabela, Detail: detalhe}
}

func snapshotDeMudancas() changedomain.Snapshot {
	return changedomain.Snapshot{
		Commits: []changedomain.Commit{
			{
				SHA:         "sha-1",
				Repository:  "acme/loja",
				Author:      "alice",
				Message:     "aumentar o timeout do pool",
				CommittedAt: t0,
				Files:       []string{"internal/db/pool.go"},
			},
			{
				SHA:         "sha-2",
				Repository:  "acme/loja",
				Author:      "bruno",
				Message:     "remover índice de created_at",
				CommittedAt: t1,
				Files:       []string{"migrations/0002.sql"},
			},
		},
		Deployments: []changedomain.Deployment{
			{
				ID:          "deploy-1",
				Repository:  "acme/loja",
				Environment: "staging",
				ServiceName: "checkout",
				CommitSHA:   "sha-1",
				DeployedAt:  t1,
				Ref:         "main",
				Task:        "deploy",
				State:       "success",
			},
			{
				ID:          "deploy-2",
				Repository:  "acme/loja",
				Environment: "staging",
				ServiceName: "checkout",
				CommitSHA:   "sha-2",
				DeployedAt:  t2,
				Ref:         "main",
				Task:        "deploy",
				State:       "success",
			},
		},
	}
}

func janela(t *testing.T, inicio time.Time) incidentdomain.InvestigationWindow {
	t.Helper()
	criada, err := incidentdomain.NewInvestigationWindowFromIncident(inicio, inicio.Add(time.Minute), time.Minute)
	if err != nil {
		t.Fatalf("criar janela de investigação: %v", err)
	}
	return criada
}

func diagnosticoDeTeste(t *testing.T, id string) application.Diagnosis {
	t.Helper()
	return application.Diagnosis{
		ID:                  id,
		ServiceName:         "checkout-service",
		Environment:         "staging",
		Windows:             janela(t, t1),
		BaselineSignalCount: 40,
		IncidentSignalCount: 20,
		Findings: []detection.Finding{
			{
				Rule:        detection.RuleDatabaseTimeout,
				ServiceName: "checkout-service",
				Score:       0.3,
				Confidence:  detection.ConfidenceHigh,
				Evidence: []detection.Evidence{{
					Summary:       "6 de 20 operações PostgreSQL tiveram timeout.",
					SignalIDs:     []string{"s-1"},
					BaselineValue: 0,
					IncidentValue: 0.3,
				}},
				Limitations: []string{"Correlação não comprova causalidade."},
			},
			{
				Rule:        detection.RuleErrorRateDelta,
				ServiceName: "checkout-service",
				Score:       0.4,
				Confidence:  detection.ConfidenceHigh,
				Evidence: []detection.Evidence{{
					Summary:       "Taxa de erro aumentou de 0% para 40%.",
					SignalIDs:     []string{"s-2"},
					BaselineValue: 0,
					IncidentValue: 0.4,
				}},
			},
		},
		Suspects: []ranking.Suspect{{
			ID:         "checkout-service",
			Label:      "checkout-service",
			Score:      0.7,
			Confidence: detection.ConfidenceHigh,
		}},
	}
}

func exigirIDs(t *testing.T, contexto string, sinais []telemetrydomain.Signal, esperados ...string) {
	t.Helper()
	obtidos := make([]string, 0, len(sinais))
	for _, lido := range sinais {
		obtidos = append(obtidos, lido.ID)
	}
	exigirIguais(t, contexto, obtidos, esperados)
}

func exigirIDsDeDeployment(t *testing.T, contexto string, deployments []changedomain.Deployment, esperados ...string) {
	t.Helper()
	obtidos := make([]string, 0, len(deployments))
	for _, deployment := range deployments {
		obtidos = append(obtidos, deployment.ID)
	}
	exigirIguais(t, contexto, obtidos, esperados)
}

func exigirIguais(t *testing.T, contexto string, obtidos, esperados []string) {
	t.Helper()
	if len(obtidos) != len(esperados) {
		t.Fatalf("%s: obtido %v, esperado %v", contexto, obtidos, esperados)
	}
	for indice := range esperados {
		if obtidos[indice] != esperados[indice] {
			t.Fatalf("%s: obtido %v, esperado %v", contexto, obtidos, esperados)
		}
	}
}

// exigirInstanteUTC cobra a normalização que separa os dois backends: o SQLite
// guarda o instante como texto e o PostgreSQL como TIMESTAMPTZ, e sem esta
// exigência a mesma janela selecionaria conjuntos diferentes conforme o fuso do
// processo que gravou.
func exigirInstanteUTC(t *testing.T, campo string, obtido, esperado time.Time) {
	t.Helper()
	if !obtido.Equal(esperado) {
		t.Errorf("%s = %v, esperado %v", campo, obtido, esperado)
	}
	if nome, _ := obtido.Zone(); nome != "UTC" {
		t.Errorf("%s voltou no fuso %q, esperado UTC", campo, nome)
	}
}

// rodarConcorrencia cobre a única diferença estrutural declarada entre os
// backends: o SQLite tem um escritor só, o PostgreSQL tem oito conexões no pool.
//
// A ADR 0016 registrava essa divergência como não coberta. Não coberta é onde
// defeito mora: o produto ingere telemetria por HTTP concorrente no `serve`, e
// dois lotes chegando juntos são o caso normal, não o excepcional. O que estes
// casos exigem é que o resultado observável seja o mesmo nos dois — ninguém
// perdido, nada duplicado — qualquer que seja a forma como cada banco serializa
// a escrita por baixo.
func rodarConcorrencia(t *testing.T, novo Fabrica) {
	t.Run("EscritasSimultaneasNaoPerdemNemDuplicamSinais", func(t *testing.T) {
		backend := novo(t)
		ctx := context.Background()

		const escritores = 8
		const porEscritor = 25
		var grupo sync.WaitGroup
		erros := make(chan error, escritores)
		for escritor := range escritores {
			grupo.Add(1)
			go func(escritor int) {
				defer grupo.Done()
				lote := make([]telemetrydomain.Signal, 0, porEscritor)
				for indice := range porEscritor {
					lote = append(lote, sinal(
						fmt.Sprintf("w%d-s%d", escritor, indice),
						"checkout", t0.Add(time.Duration(indice)*time.Second), "trace-a",
					))
				}
				if _, err := backend.Sinais.Save(ctx, lote); err != nil {
					erros <- err
				}
			}(escritor)
		}
		grupo.Wait()
		close(erros)
		for err := range erros {
			t.Fatalf("Save() concorrente erro = %v", err)
		}

		lidos, err := backend.Sinais.ListByServiceAndWindow(ctx, "checkout", t0, t4, 10_000)
		if err != nil {
			t.Fatalf("ListByServiceAndWindow() erro = %v", err)
		}
		if len(lidos) != escritores*porEscritor {
			t.Fatalf("sinais = %d, esperado %d: escrita concorrente perdeu ou duplicou",
				len(lidos), escritores*porEscritor)
		}
		vistos := make(map[string]struct{}, len(lidos))
		for _, s := range lidos {
			if _, repetido := vistos[s.ID]; repetido {
				t.Fatalf("sinal %q gravado duas vezes", s.ID)
			}
			vistos[s.ID] = struct{}{}
		}
	})

	t.Run("IdempotenciaSobrevivAEscritaSimultaneaDoMesmoLote", func(t *testing.T) {
		// O mesmo lote chegando por duas conexões ao mesmo tempo é o retry de
		// ingestão OTLP, e a garantia do ON CONFLICT DO NOTHING precisa valer
		// sob concorrência e não só em sequência.
		backend := novo(t)
		ctx := context.Background()
		lote := []telemetrydomain.Signal{
			sinal("dup-1", "checkout", t0, "trace-a"),
			sinal("dup-2", "checkout", t1, "trace-a"),
		}

		var grupo sync.WaitGroup
		erros := make(chan error, 4)
		for range 4 {
			grupo.Add(1)
			go func() {
				defer grupo.Done()
				if _, err := backend.Sinais.Save(ctx, lote); err != nil {
					erros <- err
				}
			}()
		}
		grupo.Wait()
		close(erros)
		for err := range erros {
			t.Fatalf("Save() concorrente do mesmo lote erro = %v", err)
		}

		lidos, err := backend.Sinais.ListByServiceAndWindow(ctx, "checkout", t0, t4, 100)
		if err != nil {
			t.Fatalf("ListByServiceAndWindow() erro = %v", err)
		}
		if len(lidos) != len(lote) {
			t.Fatalf("sinais = %d, esperado %d: a idempotência não valeu sob concorrência",
				len(lidos), len(lote))
		}
	})
}
