package detection

import (
	"fmt"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

func TestDetectRetryStormComparaTentativasPorTrace(t *testing.T) {
	t.Parallel()

	finding, found := DetectRetryStorm(Input{
		ServiceName: "checkout-service",
		Baseline:    retrySignals("baseline", 5, 1, "POST", "/payment"),
		Incident:    retrySignals("incident", 5, 4, "POST", "/payment"),
	})
	if !found {
		t.Fatal("DetectRetryStorm() não detectou aumento de uma para quatro tentativas por trace")
	}
	if finding.Rule != RuleRetryStorm {
		t.Fatalf("regra = %q, esperava %q", finding.Rule, RuleRetryStorm)
	}
	if finding.Confidence != ConfidenceHigh {
		t.Fatalf("confiança = %q, esperava %q com cinco traces em cada janela", finding.Confidence, ConfidenceHigh)
	}
	if finding.Score != 0.75 {
		t.Fatalf("score = %.2f, esperava 0.75 para crescimento de 1 para 4 tentativas", finding.Score)
	}
	if len(finding.Evidence) != 1 {
		t.Fatalf("evidências = %d, esperava uma consolidação da operação", len(finding.Evidence))
	}
	evidence := finding.Evidence[0]
	if evidence.BaselineValue != 1 || evidence.IncidentValue != 4 {
		t.Fatalf("médias baseline/incidente = %.2f/%.2f, esperava 1/4", evidence.BaselineValue, evidence.IncidentValue)
	}
	if len(evidence.SignalIDs) != 20 {
		t.Fatalf("IDs de evidência = %d, esperava os 20 spans repetidos no incidente", len(evidence.SignalIDs))
	}
	for _, fragment := range []string{"POST /payment", "5 de 5 traces", "1.00", "4.00"} {
		if !strings.Contains(evidence.Summary, fragment) {
			t.Fatalf("evidência %q não contém %q", evidence.Summary, fragment)
		}
	}
	if !contains(finding.Limitations, "não comprova causalidade") {
		t.Fatalf("limitação causal ausente: %#v", finding.Limitations)
	}
	if !contains(finding.Limitations, "fan-out") {
		t.Fatalf("limitação sobre repetições legítimas ausente: %#v", finding.Limitations)
	}
}

func TestDetectRetryStormIgnoraTraceIDVazioESpansServidor(t *testing.T) {
	t.Parallel()

	incident := retrySignals("incident", 5, 4, "POST", "/payment")
	for index := range incident {
		incident[index].TraceID = ""
	}
	incident = append(incident, serverRetrySignals("server", 5, 4)...)

	_, found := DetectRetryStorm(Input{
		ServiceName: "checkout-service",
		Baseline:    retrySignals("baseline", 5, 1, "POST", "/payment"),
		Incident:    incident,
	})
	if found {
		t.Fatal("DetectRetryStorm() associou spans sem trace_id ou contou requisições recebidas como retries")
	}
}

func TestDetectRetryStormUsaSomenteIdentidadeSegura(t *testing.T) {
	t.Parallel()

	baseline := databaseRetrySignals("baseline", 5, 1)
	incident := databaseRetrySignals("incident", 5, 3)
	for index := range incident {
		incident[index].Attributes["db.statement"] = fmt.Sprintf("SELECT segredo_%d FROM payments", index)
	}

	finding, found := DetectRetryStorm(Input{ServiceName: "checkout-service", Baseline: baseline, Incident: incident})
	if !found {
		t.Fatal("DetectRetryStorm() deveria agrupar operações de banco pela identidade permitida")
	}
	if strings.Contains(finding.Evidence[0].Summary, "segredo") || strings.Contains(finding.Evidence[0].Summary, "SELECT") {
		t.Fatalf("evidência vazou SQL bruto: %q", finding.Evidence[0].Summary)
	}
	if !strings.Contains(finding.Evidence[0].Summary, "PostgreSQL INSERT payments") {
		t.Fatalf("evidência não descreve a assinatura segura: %q", finding.Evidence[0].Summary)
	}
}

func TestDetectRetryStormExigeVolumeMinimoEIntegracaoNoRun(t *testing.T) {
	t.Parallel()

	inputPequeno := Input{
		ServiceName: "checkout-service",
		Baseline:    retrySignals("baseline-small", 2, 1, "POST", "/payment"),
		Incident:    retrySignals("incident-small", 2, 5, "POST", "/payment"),
	}
	if _, found := DetectRetryStorm(inputPequeno); found {
		t.Fatal("DetectRetryStorm() deveria exigir ao menos três traces comparáveis por janela")
	}

	input := Input{
		ServiceName: "checkout-service",
		Baseline:    retrySignals("baseline", 5, 1, "POST", "/payment"),
		Incident:    retrySignals("incident", 5, 3, "POST", "/payment"),
	}
	assertFinding(t, Run(input), RuleRetryStorm, ConfidenceHigh)
}

func TestDetectRetryStormNaoSinalizaPadraoEstavel(t *testing.T) {
	t.Parallel()

	_, found := DetectRetryStorm(Input{
		ServiceName: "checkout-service",
		Baseline:    retrySignals("baseline", 5, 2, "POST", "/payment"),
		Incident:    retrySignals("incident", 5, 2, "POST", "/payment"),
	})
	if found {
		t.Fatal("DetectRetryStorm() sinalizou uma quantidade estável de tentativas")
	}
}

func retrySignals(prefix string, traceCount, attempts int, method, route string) []domain.Signal {
	signals := make([]domain.Signal, 0, traceCount*attempts)
	for traceIndex := range traceCount {
		for attempt := range attempts {
			signals = append(signals, domain.Signal{
				ID:          fmt.Sprintf("%s-%d-%d", prefix, traceIndex, attempt),
				ServiceName: "checkout-service",
				TraceID:     fmt.Sprintf("%s-trace-%d", prefix, traceIndex),
				Attributes: map[string]string{
					"span.kind":           "SPAN_KIND_CLIENT",
					"http.request.method": method,
					"http.route":          route,
				},
			})
		}
	}
	return signals
}

func serverRetrySignals(prefix string, traceCount, attempts int) []domain.Signal {
	signals := retrySignals(prefix, traceCount, attempts, "POST", "/checkout")
	for index := range signals {
		signals[index].Attributes["span.kind"] = "SPAN_KIND_SERVER"
	}
	return signals
}

func databaseRetrySignals(prefix string, traceCount, attempts int) []domain.Signal {
	signals := make([]domain.Signal, 0, traceCount*attempts)
	for traceIndex := range traceCount {
		for attempt := range attempts {
			signals = append(signals, domain.Signal{
				ID:          fmt.Sprintf("%s-db-%d-%d", prefix, traceIndex, attempt),
				ServiceName: "checkout-service",
				TraceID:     fmt.Sprintf("%s-trace-%d", prefix, traceIndex),
				Attributes: map[string]string{
					"span.kind":          "SPAN_KIND_CLIENT",
					"db.system.name":     "postgresql",
					"db.operation.name":  "INSERT",
					"db.collection.name": "payments",
				},
			})
		}
	}
	return signals
}

// TestRetryStormReconheceBancoSemAtributoDeOperação cobre uma lacuna encontrada
// ao capturar a instrumentação oficial do Node: os 68 spans de banco não
// trouxeram nenhum atributo de operação. A biblioteca coloca a operação no nome
// do span, como "pg.query:SELECT captura", e não em db.operation.
//
// Sem operação, a assinatura do retry desistia do span e uma tempestade de
// retry no banco ficava invisível para toda aplicação instrumentada assim.
// O sistema de banco, sozinho, já identifica a repetição — a operação apenas
// refina o rótulo.
func TestRetryStormReconheceBancoSemAtributoDeOperação(t *testing.T) {
	t.Parallel()

	baseline := bancoNodeRepetido(8, 1)
	incidente := bancoNodeRepetido(8, 4)

	finding, found := DetectRetryStorm(Input{
		ServiceName: "captura-node", Baseline: baseline, Incident: incidente,
	})
	if !found {
		t.Fatal("retry storm no banco ficou invisível sem o atributo de operação")
	}
	if len(finding.Evidence) == 0 {
		t.Fatal("finding sem evidência")
	}
}

// bancoNodeRepetido monta traces em que a mesma operação de banco se repete,
// no formato que a instrumentação do Node produz: sem db.operation, com a
// operação embutida no nome do span.
func bancoNodeRepetido(traces, repeticoes int) []domain.Signal {
	signals := make([]domain.Signal, 0, traces*repeticoes)
	for trace := 0; trace < traces; trace++ {
		for repeticao := 0; repeticao < repeticoes; repeticao++ {
			signals = append(signals, domain.Signal{
				ID:          fmt.Sprintf("node-%02d-%02d", trace, repeticao),
				ServiceName: "captura-node",
				TraceID:     fmt.Sprintf("trace-%02d", trace),
				SpanID:      fmt.Sprintf("span-%02d-%02d", trace, repeticao),
				Attributes: map[string]string{
					"db.system.name": "postgresql",
					"db.namespace":   "captura",
					"span.kind":      "SPAN_KIND_CLIENT",
					"span.name":      "pg.query:SELECT captura",
				},
				Measurements: map[string]float64{"duration_ms": 5},
			})
		}
	}
	return signals
}

// TestRetryStormNãoConfundeOperaçõesDiferentes cobre um risco introduzido ao
// aceitar spans de banco sem atributo de operação: sem ela, todas as chamadas
// de um mesmo sistema colapsavam em uma única assinatura, e um trace que faz
// INSERT e depois SELECT — o padrão mais comum — passava a parecer a mesma
// operação repetida.
//
// A maioria das instrumentações põe a operação no nome do span. Ele é o
// discriminador disponível, e usá-lo evita inventar um valor que a telemetria
// não trouxe.
func TestRetryStormNãoConfundeOperaçõesDiferentes(t *testing.T) {
	t.Parallel()

	// A baseline faz uma escrita por trace. O incidente faz a mesma escrita e
	// acrescenta três leituras — uma mudança legítima do padrão de consultas,
	// não um retry. Com a assinatura colapsada por falta do atributo de
	// operação, as quatro chamadas viram "a mesma operação repetida quatro
	// vezes" e o detector acusaria tempestade de retry onde não há nenhuma.
	baseline := make([]domain.Signal, 0, 8)
	for trace := 0; trace < 8; trace++ {
		baseline = append(baseline, spanDeBancoNomeado("base", trace, 0, "INSERT"))
	}
	incidente := make([]domain.Signal, 0, 32)
	for trace := 0; trace < 8; trace++ {
		incidente = append(incidente, spanDeBancoNomeado("inc", trace, 0, "INSERT"))
		for leitura := 1; leitura <= 3; leitura++ {
			incidente = append(incidente, spanDeBancoNomeado("inc", trace, leitura, "SELECT"))
		}
	}

	if finding, found := DetectRetryStorm(Input{
		ServiceName: "captura", Baseline: baseline, Incident: incidente,
	}); found {
		t.Fatalf("operações distintas foram contadas como repetição: %s", finding.Evidence[0].Summary)
	}
}

// spanDeBancoNomeado monta um span de banco sem atributo de operação, com a
// operação apenas no nome, como fazem as instrumentações de Python e Node.
func spanDeBancoNomeado(prefixo string, trace, indice int, operacao string) domain.Signal {
	return domain.Signal{
		ID:          fmt.Sprintf("%s-%02d-%02d", prefixo, trace, indice),
		ServiceName: "captura",
		TraceID:     fmt.Sprintf("%s-trace-%02d", prefixo, trace),
		SpanID:      fmt.Sprintf("%s-span-%02d-%02d", prefixo, trace, indice),
		Attributes: map[string]string{
			"db.system.name": "postgresql",
			"span.kind":      "SPAN_KIND_CLIENT",
			"span.name":      operacao,
		},
		Measurements: map[string]float64{"duration_ms": 3},
	}
}
