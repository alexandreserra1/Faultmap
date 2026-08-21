package detection

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

func TestRunDetectsRelevantChanges(t *testing.T) {
	t.Parallel()

	input := Input{
		ServiceName: "checkout-service",
		Baseline: append(
			httpSignals("baseline", 10, 0, 100),
			databaseTimeoutSignals("baseline-database", 5, 0, 50)...,
		),
		Incident: append(
			httpSignals("incident", 10, 6, 800),
			databaseTimeoutSignals("database", 5, 3, 1_500)...,
		),
	}

	findings := Run(input)

	assertFinding(t, findings, RuleErrorRateDelta, ConfidenceHigh)
	assertFinding(t, findings, RuleLatencyDelta, ConfidenceHigh)
	databaseFinding := assertFinding(t, findings, RuleDatabaseTimeout, ConfidenceHigh)
	if len(databaseFinding.Evidence) == 0 {
		t.Fatal("detector de banco deveria incluir evidências")
	}
	for _, finding := range findings {
		if finding.ServiceName != "checkout-service" {
			t.Fatalf("serviço inesperado: %q", finding.ServiceName)
		}
		if finding.Score < 0 || finding.Score > 1 {
			t.Fatalf("score fora do intervalo [0,1]: %f", finding.Score)
		}
		if !contains(finding.Limitations, "não comprova causalidade") {
			t.Fatalf("finding %q deve declarar que não comprova causalidade: %#v", finding.Rule, finding.Limitations)
		}
	}
}

func TestRunDoesNotCreateFindingsWithoutEvidence(t *testing.T) {
	t.Parallel()

	findings := Run(Input{
		ServiceName: "checkout-service",
		Baseline:    httpSignals("baseline", 10, 0, 100),
		Incident: append(
			httpSignals("incident", 10, 0, 100),
			databaseTimeoutSignals("database", 5, 0, 100)...,
		),
	})

	if len(findings) != 0 {
		t.Fatalf("esperava nenhum finding sem evidência, recebeu %#v", findings)
	}
}

func TestRunReducesConfidenceForSmallSamples(t *testing.T) {
	t.Parallel()

	findings := Run(Input{
		ServiceName: "checkout-service",
		Baseline:    httpSignals("baseline", 1, 0, 100),
		Incident: append(
			httpSignals("incident", 1, 1, 2_000),
			databaseTimeoutSignals("database", 1, 1, 1_900)...,
		),
	})

	for _, rule := range []string{RuleErrorRateDelta, RuleLatencyDelta, RuleDatabaseTimeout} {
		finding := assertFinding(t, findings, rule, ConfidenceLow)
		if !contains(finding.Limitations, "amostra pequena") {
			t.Fatalf("finding %q deveria declarar amostra pequena: %#v", rule, finding.Limitations)
		}
	}
}

// TestRunExplicaProtocolosEStatus torna a evidência terminal verificável por pessoas.
func TestRunExplicaProtocolosEStatus(t *testing.T) {
	t.Parallel()

	findings := Run(Input{
		ServiceName: "checkout-service",
		Baseline: append(
			httpSignals("baseline", 1, 0, 120),
			databaseTimeoutSignals("baseline-database", 1, 0, 50)...,
		),
		Incident: append(
			httpSignals("incident", 1, 1, 2300),
			databaseTimeoutSignals("incident-database", 1, 1, 2100)...,
		),
	})

	errorRate := assertFinding(t, findings, RuleErrorRateDelta, ConfidenceLow)
	if !contains([]string{errorRate.Evidence[0].Summary}, "HTTP 500") {
		t.Fatalf("evidência de erro não informa HTTP 500: %q", errorRate.Evidence[0].Summary)
	}
	database := assertFinding(t, findings, RuleDatabaseTimeout, ConfidenceLow)
	if !contains([]string{database.Evidence[0].Summary}, "PostgreSQL") {
		t.Fatalf("evidência de banco não informa PostgreSQL: %q", database.Evidence[0].Summary)
	}
	if !contains([]string{database.Evidence[0].Summary}, "1 timeout observado") {
		t.Fatalf("evidência de banco não usa singular natural: %q", database.Evidence[0].Summary)
	}
}

// TestDatabaseTimeoutIgnoraErroGenerico impede que o detector confunda falhas sem timeout.
func TestDatabaseTimeoutIgnoraErroGenerico(t *testing.T) {
	t.Parallel()

	baseline := databaseTimeoutSignals("baseline", 5, 0, 50)
	incident := databaseTimeoutSignals("incident", 5, 0, 100)
	incident[0].Severity = "ERROR"
	incident[0].Attributes["error.type"] = "constraint_violation"

	_, found := DetectDatabaseTimeout(Input{
		ServiceName: "checkout-service",
		Baseline:    baseline,
		Incident:    incident,
	})
	if found {
		t.Fatal("DetectDatabaseTimeout() encontrou timeout em erro genérico")
	}
}

// TestTraceCorrelationRelacionaTimeoutEImpactoHTTPNoMesmoTrace protege a hipótese contra correlações apenas temporais.
func TestTraceCorrelationRelacionaTimeoutEImpactoHTTPNoMesmoTrace(t *testing.T) {
	t.Parallel()

	baseline := tracePairs("baseline", 5, false, false, 120)
	incident := tracePairs("incident", 5, true, true, 2_000)

	finding, found := DetectTraceCorrelation(Input{
		ServiceName: "checkout-service",
		Baseline:    baseline,
		Incident:    incident,
	})
	if !found {
		t.Fatal("DetectTraceCorrelation() não relacionou timeout e erro HTTP pertencentes ao mesmo trace")
	}
	if finding.Rule != RuleTraceCorrelation {
		t.Fatalf("regra = %q, esperava %q", finding.Rule, RuleTraceCorrelation)
	}
	if finding.Confidence != ConfidenceHigh {
		t.Fatalf("confiança = %q, esperava %q para cinco traces correlacionados", finding.Confidence, ConfidenceHigh)
	}
	if len(finding.Evidence) != 1 {
		t.Fatalf("evidências = %d, esperava uma evidência quantitativa consolidada", len(finding.Evidence))
	}
	evidence := finding.Evidence[0]
	if !contains([]string{evidence.Summary}, "5 de 5") {
		t.Fatalf("evidência não quantifica os traces correlacionados: %q", evidence.Summary)
	}
	if !contains([]string{evidence.Summary}, "mesmo trace") {
		t.Fatalf("evidência não explicita a correlação pelo mesmo trace_id: %q", evidence.Summary)
	}
	if !contains([]string{evidence.Summary}, "hipótese") {
		t.Fatalf("evidência deveria apresentar hipótese, sem afirmar causa: %q", evidence.Summary)
	}
	if !contains(finding.Limitations, "não comprova causalidade") {
		t.Fatalf("finding deveria preservar a limitação causal: %#v", finding.Limitations)
	}
}

// TestTraceCorrelationNaoRelacionaSinaisDeTracesDiferentes evita atribuir um erro HTTP a outro fluxo distribuído.
func TestTraceCorrelationNaoRelacionaSinaisDeTracesDiferentes(t *testing.T) {
	t.Parallel()

	incident := tracePairs("incident", 5, true, true, 2_000)
	for index := range incident {
		if incident[index].Attributes["db.system.name"] != "" {
			incident[index].TraceID = "database-" + incident[index].TraceID
		}
	}

	_, found := DetectTraceCorrelation(Input{
		ServiceName: "checkout-service",
		Baseline:    tracePairs("baseline", 5, false, false, 120),
		Incident:    incident,
	})
	if found {
		t.Fatal("DetectTraceCorrelation() correlacionou timeout e erro HTTP de trace_ids diferentes")
	}
}

// TestTraceCorrelationAceitaLatenciaAltaSemErroHTTP cobre impacto percebido mesmo quando a resposta termina com sucesso.
func TestTraceCorrelationAceitaLatenciaAltaSemErroHTTP(t *testing.T) {
	t.Parallel()

	finding, found := DetectTraceCorrelation(Input{
		ServiceName: "checkout-service",
		Baseline:    tracePairs("baseline", 5, false, false, 100),
		Incident:    tracePairs("incident", 5, true, false, 2_000),
	})
	if !found {
		t.Fatal("DetectTraceCorrelation() não relacionou timeout e alta latência HTTP no mesmo trace")
	}
	if finding.Rule != RuleTraceCorrelation {
		t.Fatalf("regra = %q, esperava %q", finding.Rule, RuleTraceCorrelation)
	}
}

// TestTraceCorrelationReduzConfiancaParaAmostraPequena deixa explícita a cautela estatística da hipótese.
func TestTraceCorrelationReduzConfiancaParaAmostraPequena(t *testing.T) {
	t.Parallel()

	finding, found := DetectTraceCorrelation(Input{
		ServiceName: "checkout-service",
		Baseline:    tracePairs("baseline", 1, false, false, 100),
		Incident:    tracePairs("incident", 1, true, true, 2_000),
	})
	if !found {
		t.Fatal("DetectTraceCorrelation() deveria produzir hipótese para um trace correlacionado")
	}
	if finding.Confidence != ConfidenceLow {
		t.Fatalf("confiança = %q, esperava %q para um trace correlacionado", finding.Confidence, ConfidenceLow)
	}
	if !contains(finding.Limitations, "amostra pequena") {
		t.Fatalf("finding deveria declarar amostra pequena: %#v", finding.Limitations)
	}
}

func assertFinding(t *testing.T, findings []Finding, rule string, confidence Confidence) Finding {
	t.Helper()
	for _, finding := range findings {
		if finding.Rule != rule {
			continue
		}
		if finding.Confidence != confidence {
			t.Fatalf("confiança de %q = %q, esperava %q", rule, finding.Confidence, confidence)
		}
		return finding
	}
	t.Fatalf("finding %q não encontrado em %#v", rule, findings)
	return Finding{}
}

func httpSignals(prefix string, count, errorCount int, durationMS float64) []domain.Signal {
	signals := make([]domain.Signal, 0, count)
	for index := range count {
		statusCode := "201"
		severity := "INFO"
		if index < errorCount {
			statusCode = "500"
			severity = "ERROR"
		}
		signals = append(signals, domain.Signal{
			ID:          fmt.Sprintf("%s-http-%d", prefix, index),
			ServiceName: "checkout-service",
			Severity:    severity,
			Attributes: map[string]string{
				"http.response.status_code": statusCode,
			},
			Measurements: map[string]float64{"duration_ms": durationMS},
		})
	}
	return signals
}

func databaseTimeoutSignals(prefix string, count, timeoutCount int, durationMS float64) []domain.Signal {
	signals := make([]domain.Signal, 0, count)
	for index := range count {
		attributes := map[string]string{
			"db.system.name":    "postgresql",
			"db.operation.name": "INSERT",
		}
		severity := "INFO"
		if index < timeoutCount {
			attributes["error.type"] = "timeout"
			severity = "ERROR"
		}
		signals = append(signals, domain.Signal{
			ID:           fmt.Sprintf("%s-db-%d", prefix, index),
			ServiceName:  "checkout-service",
			Severity:     severity,
			Attributes:   attributes,
			Measurements: map[string]float64{"duration_ms": durationMS},
		})
	}
	return signals
}

// tracePairs cria pares HTTP/PostgreSQL com trace_id compartilhado para exercitar a correlação sem depender da ordem dos spans.
func tracePairs(prefix string, count int, databaseTimeout, httpError bool, httpDurationMS float64) []domain.Signal {
	signals := make([]domain.Signal, 0, count*2)
	for index := range count {
		traceID := fmt.Sprintf("%s-trace-%d", prefix, index)
		statusCode := "201"
		httpSeverity := "INFO"
		if httpError {
			statusCode = "500"
			httpSeverity = "ERROR"
		}
		databaseAttributes := map[string]string{
			"db.system.name":    "postgresql",
			"db.operation.name": "INSERT",
		}
		databaseSeverity := "INFO"
		if databaseTimeout {
			databaseAttributes["error.type"] = "timeout"
			databaseSeverity = "ERROR"
		}
		signals = append(signals,
			domain.Signal{
				ID:           fmt.Sprintf("%s-http-%d", prefix, index),
				ServiceName:  "checkout-service",
				TraceID:      traceID,
				Severity:     httpSeverity,
				Attributes:   map[string]string{"http.response.status_code": statusCode},
				Measurements: map[string]float64{"duration_ms": httpDurationMS},
			},
			domain.Signal{
				ID:           fmt.Sprintf("%s-database-%d", prefix, index),
				ServiceName:  "checkout-service",
				TraceID:      traceID,
				Severity:     databaseSeverity,
				Attributes:   databaseAttributes,
				Measurements: map[string]float64{"duration_ms": httpDurationMS - 20},
			},
		)
	}
	return signals
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), strings.ToLower(want)) {
			return true
		}
	}
	return false
}

// TestErrorRateDeltaIgnoraVariaçãoDeAmostragem reproduz o falso positivo
// encontrado no modo difícil: com 25% de erro permanente, uma janela sorteou 3
// falhas em 16 e a outra 4 em 16. O sistema não mudou; só a amostra mudou.
// Uma falha a mais em dezesseis não pode virar hipótese de regressão.
func TestErrorRateDeltaIgnoraVariaçãoDeAmostragem(t *testing.T) {
	t.Parallel()

	finding, found := DetectErrorRateDelta(Input{
		ServiceName: "checkout-service",
		Baseline:    httpSignals("baseline", 16, 3, 100),
		Incident:    httpSignals("incident", 16, 4, 100),
	})
	if found {
		t.Fatalf("detector acusou ruído de amostragem: %s", finding.Evidence[0].Summary)
	}
}

// TestErrorRateDeltaIgnoraUmaFalhaIsolada cobre o caso mais comum de ruído:
// uma janela perfeita e um único erro na outra.
func TestErrorRateDeltaIgnoraUmaFalhaIsolada(t *testing.T) {
	t.Parallel()

	if _, found := DetectErrorRateDelta(Input{
		ServiceName: "checkout-service",
		Baseline:    httpSignals("baseline", 20, 0, 100),
		Incident:    httpSignals("incident", 20, 1, 100),
	}); found {
		t.Fatal("detector acusou regressão a partir de uma única falha isolada")
	}
}

// TestErrorRateDeltaAcusaRegressãoReal garante que a proteção contra ruído não
// silenciou o que o produto existe para encontrar.
func TestErrorRateDeltaAcusaRegressãoReal(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                       string
		baselineCount, baselineErr int
		incidentCount, incidentErr int
	}{
		{name: "saudável para totalmente quebrado", baselineCount: 16, baselineErr: 0, incidentCount: 16, incidentErr: 16},
		{name: "saudável para metade falhando", baselineCount: 20, baselineErr: 0, incidentCount: 20, incidentErr: 10},
		{name: "ruído baixo para maioria falhando", baselineCount: 20, baselineErr: 1, incidentCount: 20, incidentErr: 15},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if _, found := DetectErrorRateDelta(Input{
				ServiceName: "checkout-service",
				Baseline:    httpSignals("baseline", testCase.baselineCount, testCase.baselineErr, 100),
				Incident:    httpSignals("incident", testCase.incidentCount, testCase.incidentErr, 100),
			}); !found {
				t.Fatal("detector silenciou uma regressão real")
			}
		})
	}
}

// TestErrorRateDeltaIgnoraDiferençaIrrelevanteEmVolumeAlto evita o alarme que
// só existe porque há muitos dados: meio ponto percentual não muda a operação.
func TestErrorRateDeltaIgnoraDiferençaIrrelevanteEmVolumeAlto(t *testing.T) {
	t.Parallel()

	if _, found := DetectErrorRateDelta(Input{
		ServiceName: "checkout-service",
		Baseline:    httpSignals("baseline", 20_000, 200, 100),
		Incident:    httpSignals("incident", 20_000, 280, 100),
	}); found {
		t.Fatal("detector acusou diferença irrelevante apenas por causa do volume")
	}
}

// TestDetectoresEntendemConvençãoAntigaDeStatusHTTP cobre o defeito encontrado
// ao ligar o Faultmap a uma aplicação FastAPI real instrumentada
// automaticamente: ela emite "http.status_code", o nome anterior da convenção
// OpenTelemetry, enquanto a demo escrita por nós usa "http.response.status_code".
// Os detectores enxergavam zero sinais HTTP e ficavam cegos para a aplicação
// inteira, mesmo com falha total.
func TestDetectoresEntendemConvençãoAntigaDeStatusHTTP(t *testing.T) {
	t.Parallel()

	input := Input{
		ServiceName: "strideredge-api",
		Baseline:    legacyHTTPSignals("baseline", 20, 0, 10),
		Incident:    legacyHTTPSignals("incident", 20, 20, 900),
	}

	if _, found := DetectErrorRateDelta(input); !found {
		t.Fatal("detector de erro ficou cego para spans com http.status_code")
	}
	if _, found := DetectLatencyDelta(input); !found {
		t.Fatal("detector de latência ficou cego para spans com http.status_code")
	}
}

// TestErrorRateIgnoraSpansInternosDaInstrumentação garante que a taxa de erro
// conte requisições, não spans. A instrumentação automática do FastAPI emite um
// span interno "http send" que também carrega o código de resposta; contá-lo
// dobraria o denominador e faria 100% de falha ser reportado como 50%.
func TestErrorRateIgnoraSpansInternosDaInstrumentação(t *testing.T) {
	t.Parallel()

	baseline := legacyHTTPSignals("baseline", 20, 0, 10)
	incident := legacyHTTPSignals("incident", 20, 20, 900)
	// Cada requisição real vem acompanhada do seu span interno correspondente.
	incident = append(incident, internalHTTPSendSignals("incident-send", 20, 200)...)
	baseline = append(baseline, internalHTTPSendSignals("baseline-send", 20, 200)...)

	finding, found := DetectErrorRateDelta(Input{
		ServiceName: "strideredge-api", Baseline: baseline, Incident: incident,
	})
	if !found {
		t.Fatal("detector não encontrou a regressão total")
	}
	if finding.Evidence[0].IncidentValue != 1 {
		t.Fatalf("taxa de erro do incidente = %.2f, esperado 1.00; spans internos diluíram a conta",
			finding.Evidence[0].IncidentValue)
	}
}

// legacyHTTPSignals reproduz spans de servidor na convenção anterior.
func legacyHTTPSignals(prefix string, count, errorCount int, durationMS float64) []domain.Signal {
	signals := make([]domain.Signal, 0, count)
	for index := range count {
		statusCode := "200"
		if index < errorCount {
			statusCode = "500"
		}
		signals = append(signals, domain.Signal{
			ID:          fmt.Sprintf("%s-legacy-%d", prefix, index),
			ServiceName: "strideredge-api",
			Attributes: map[string]string{
				"http.status_code": statusCode,
				"span.kind":        "SPAN_KIND_SERVER",
				"span.name":        "GET /api/v1/form",
			},
			Measurements: map[string]float64{"duration_ms": durationMS},
		})
	}
	return signals
}

// internalHTTPSendSignals reproduz o span interno que a instrumentação ASGI
// emite por requisição, carregando o mesmo código de resposta do span principal.
func internalHTTPSendSignals(prefix string, count int, statusCode int) []domain.Signal {
	signals := make([]domain.Signal, 0, count)
	for index := range count {
		signals = append(signals, domain.Signal{
			ID:          fmt.Sprintf("%s-internal-%d", prefix, index),
			ServiceName: "strideredge-api",
			Attributes: map[string]string{
				"http.status_code": strconv.Itoa(statusCode),
				"span.kind":        "SPAN_KIND_INTERNAL",
				"span.name":        "GET /api/v1/form http send",
			},
			Measurements: map[string]float64{"duration_ms": 0.02},
		})
	}
	return signals
}

// TestLatencyDeltaIgnoraOscilaçãoIrrelevante cobre um falso positivo observado
// em um sistema saudável: o p95 subiu fração de milissegundo e o Faultmap
// relatou "aumentou de 3 ms para 3 ms". A oscilação normal de um serviço rápido
// não é regressão.
func TestLatencyDeltaIgnoraOscilaçãoIrrelevante(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		baseline, incident float64
	}{
		{name: "fração de milissegundo", baseline: 2.6, incident: 3.1},
		{name: "um milissegundo em serviço rápido", baseline: 3, incident: 4},
		{name: "aumento pequeno em serviço lento", baseline: 300, incident: 303},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if finding, found := DetectLatencyDelta(Input{
				ServiceName: "checkout-service",
				Baseline:    httpSignals("baseline", 20, 0, testCase.baseline),
				Incident:    httpSignals("incident", 20, 0, testCase.incident),
			}); found {
				t.Fatalf("detector acusou oscilação irrelevante: %s", finding.Evidence[0].Summary)
			}
		})
	}
}

// TestLatencyDeltaAcusaRegressãoReal garante que o piso não silenciou o que o
// detector existe para encontrar.
func TestLatencyDeltaAcusaRegressãoReal(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		baseline, incident float64
	}{
		{name: "dezenas de vezes mais lento", baseline: 9, incident: 158},
		{name: "serviço rápido que dobra com folga", baseline: 4, incident: 40},
		{name: "serviço lento que piora muito", baseline: 300, incident: 900},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if _, found := DetectLatencyDelta(Input{
				ServiceName: "checkout-service",
				Baseline:    httpSignals("baseline", 20, 0, testCase.baseline),
				Incident:    httpSignals("incident", 20, 0, testCase.incident),
			}); !found {
				t.Fatal("detector silenciou uma regressão real de latência")
			}
		})
	}
}

// TestProveniênciaNãoDependeDaOrdemDeChegada cobre um defeito encontrado por
// teste aleatório: signalIDs preservava a ordem de chegada dos sinais, então a
// amostra de proveniência impressa pelo `explain suspect` mudava conforme a
// ordem em que a telemetria vinha do banco.
//
// O produto promete mesma entrada, mesma saída. Isso só se sustentava porque a
// consulta ordena — uma garantia que vive em outra camada e pode mudar sem que
// ninguém relacione as duas coisas.
func TestProveniênciaNãoDependeDaOrdemDeChegada(t *testing.T) {
	t.Parallel()

	baseline := make([]domain.Signal, 0, 20)
	incidente := make([]domain.Signal, 0, 20)
	instante := time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC)
	for índice := 0; índice < 20; índice++ {
		status := "200"
		if índice < 8 {
			status = "500"
		}
		baseline = append(baseline, domain.Signal{
			ID: fmt.Sprintf("base-%02d", índice), Type: domain.SignalTypeSpan,
			ServiceName: "checkout-service", Timestamp: instante.Add(time.Duration(índice) * time.Second),
			TraceID:      fmt.Sprintf("t-base-%02d", índice),
			Attributes:   map[string]string{"http.response.status_code": "200", "span.kind": "SPAN_KIND_SERVER"},
			Measurements: map[string]float64{"duration_ms": 20},
		})
		incidente = append(incidente, domain.Signal{
			ID: fmt.Sprintf("inc-%02d", índice), Type: domain.SignalTypeSpan,
			ServiceName: "checkout-service", Timestamp: instante.Add(time.Hour + time.Duration(índice)*time.Second),
			TraceID:      fmt.Sprintf("t-inc-%02d", índice),
			Attributes:   map[string]string{"http.response.status_code": status, "span.kind": "SPAN_KIND_SERVER"},
			Measurements: map[string]float64{"duration_ms": 20},
		})
	}

	referência := Run(Input{ServiceName: "checkout-service", Baseline: baseline, Incident: incidente})
	if len(referência) == 0 {
		t.Fatal("nenhum finding; o teste não verificaria proveniência")
	}

	// A ordem invertida é suficiente e determinística: se a saída depende da
	// ordem de entrada, inverter já expõe.
	invertido := func(original []domain.Signal) []domain.Signal {
		copiado := append([]domain.Signal(nil), original...)
		for início, fim := 0, len(copiado)-1; início < fim; início, fim = início+1, fim-1 {
			copiado[início], copiado[fim] = copiado[fim], copiado[início]
		}
		return copiado
	}
	obtido := Run(Input{ServiceName: "checkout-service", Baseline: invertido(baseline), Incident: invertido(incidente)})

	for índice := range referência {
		for posição := range referência[índice].Evidence {
			esperada := strings.Join(referência[índice].Evidence[posição].SignalIDs, ",")
			atual := strings.Join(obtido[índice].Evidence[posição].SignalIDs, ",")
			if esperada != atual {
				t.Fatalf("regra %s: proveniência mudou com a ordem de chegada\n  antes:  %s\n  depois: %s",
					referência[índice].Rule, esperada, atual)
			}
		}
	}
}
