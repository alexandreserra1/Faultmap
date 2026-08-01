// Package detection avalia sinais normalizados e produz hipóteses auditáveis, sem acessar infraestrutura externa.
package detection

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

const (
	// RuleErrorRateDelta identifica aumento de respostas HTTP com falha.
	RuleErrorRateDelta = "error_rate_delta"
	// RuleLatencyDelta identifica aumento relevante de latência p95 HTTP.
	RuleLatencyDelta = "latency_delta"
	// RuleDatabaseTimeout identifica timeouts ou erros em spans de banco de dados.
	RuleDatabaseTimeout = "database_timeout"

	minimumSampleSize = 5
)

// Confidence expressa o nível de sustentação disponível para um Finding.
type Confidence string

const (
	// ConfidenceLow indica que há evidência, mas o volume observado é pequeno.
	ConfidenceLow Confidence = "baixa"
	// ConfidenceHigh indica que o detector avaliou o volume mínimo de sinais.
	ConfidenceHigh Confidence = "alta"
)

// Evidence preserva os valores e sinais que sustentam um Finding para permitir auditoria posterior.
type Evidence struct {
	Summary       string
	SignalIDs     []string
	BaselineValue float64
	IncidentValue float64
}

// Finding descreve uma hipótese baseada em sinais observados, sem afirmar causalidade.
type Finding struct {
	Rule        string
	ServiceName string
	Score       float64
	Confidence  Confidence
	Evidence    []Evidence
	Limitations []string
}

// Input agrupa os sinais de uma mesma aplicação nas janelas baseline e incidente.
type Input struct {
	ServiceName string
	Baseline    []domain.Signal
	Incident    []domain.Signal
}

// Run executa os detectores iniciais em uma entrada já normalizada e devolve somente hipóteses com evidência.
func Run(input Input) []Finding {
	input.Baseline = signalsForService(input.ServiceName, input.Baseline)
	input.Incident = signalsForService(input.ServiceName, input.Incident)

	findings := make([]Finding, 0, 3)
	if finding, found := DetectErrorRateDelta(input); found {
		findings = append(findings, finding)
	}
	if finding, found := DetectLatencyDelta(input); found {
		findings = append(findings, finding)
	}
	if finding, found := DetectDatabaseTimeout(input); found {
		findings = append(findings, finding)
	}
	return findings
}

// DetectErrorRateDelta compara a taxa de respostas HTTP 5xx entre baseline e incidente.
func DetectErrorRateDelta(input Input) (Finding, bool) {
	baseline := filterHTTPSignals(input.Baseline)
	incident := filterHTTPSignals(input.Incident)
	if len(baseline) == 0 || len(incident) == 0 {
		return Finding{}, false
	}

	baselineRate := errorRate(baseline)
	incidentRate := errorRate(incident)
	delta := incidentRate - baselineRate
	if delta <= 0 {
		return Finding{}, false
	}

	return newFinding(
		RuleErrorRateDelta,
		input.ServiceName,
		delta,
		sampleConfidence(len(baseline), len(incident)),
		[]Evidence{{
			Summary:       fmt.Sprintf("Foram observadas respostas HTTP %s; taxa de erro aumentou de %.2f%% para %.2f%%.", strings.Join(httpErrorStatuses(incident), ", "), baselineRate*100, incidentRate*100),
			SignalIDs:     signalIDs(incident),
			BaselineValue: baselineRate,
			IncidentValue: incidentRate,
		}},
		len(baseline),
		len(incident),
	), true
}

// DetectLatencyDelta compara a latência p95 dos spans HTTP entre baseline e incidente.
func DetectLatencyDelta(input Input) (Finding, bool) {
	baseline := filterHTTPSignals(input.Baseline)
	incident := filterHTTPSignals(input.Incident)
	if len(baseline) == 0 || len(incident) == 0 {
		return Finding{}, false
	}

	baselineP95, baselineOK := percentile95(baseline)
	incidentP95, incidentOK := percentile95(incident)
	if !baselineOK || !incidentOK || incidentP95 <= baselineP95 {
		return Finding{}, false
	}

	deltaScore := (incidentP95 - baselineP95) / incidentP95
	return newFinding(
		RuleLatencyDelta,
		input.ServiceName,
		deltaScore,
		sampleConfidence(len(baseline), len(incident)),
		[]Evidence{{
			Summary:       fmt.Sprintf("A duração p95 (latência) HTTP aumentou de %.0f ms para %.0f ms.", baselineP95, incidentP95),
			SignalIDs:     signalIDs(incident),
			BaselineValue: baselineP95,
			IncidentValue: incidentP95,
		}},
		len(baseline),
		len(incident),
	), true
}

// DetectDatabaseTimeout identifica erros de banco no incidente e os compara à baseline quando disponível.
func DetectDatabaseTimeout(input Input) (Finding, bool) {
	baseline := filterDatabaseSignals(input.Baseline)
	incident := filterDatabaseSignals(input.Incident)
	if len(incident) == 0 {
		return Finding{}, false
	}

	baselineTimeouts := databaseTimeouts(baseline)
	incidentTimeouts := databaseTimeouts(incident)
	if len(incidentTimeouts) == 0 {
		return Finding{}, false
	}

	baselineRate := fraction(len(baselineTimeouts), len(baseline))
	incidentRate := fraction(len(incidentTimeouts), len(incident))
	systems := strings.Join(databaseSystems(incidentTimeouts), ", ")
	return newFinding(
		RuleDatabaseTimeout,
		input.ServiceName,
		incidentRate,
		sampleConfidence(len(baseline), len(incident)),
		[]Evidence{{
			Summary:       databaseTimeoutSummary(len(baselineTimeouts), len(baseline), len(incidentTimeouts), len(incident), systems),
			SignalIDs:     signalIDs(incidentTimeouts),
			BaselineValue: baselineRate,
			IncidentValue: incidentRate,
		}},
		len(baseline),
		len(incident),
	), true
}

// newFinding centraliza as garantias comuns de score, cautela estatística e ausência de afirmação causal.
func newFinding(rule, serviceName string, score float64, confidence Confidence, evidence []Evidence, baselineCount, incidentCount int) Finding {
	limitations := []string{"Correlação entre sinais não comprova causalidade."}
	if confidence == ConfidenceLow {
		limitations = append(limitations, fmt.Sprintf("Amostra pequena: baseline com %d e incidente com %d sinais relevantes; mínimo recomendado de %d por janela.", baselineCount, incidentCount, minimumSampleSize))
	}
	return Finding{
		Rule:        rule,
		ServiceName: serviceName,
		Score:       clamp(score),
		Confidence:  confidence,
		Evidence:    evidence,
		Limitations: limitations,
	}
}

// signalsForService impede que uma entrada acidentalmente mista atribua evidências a outro serviço.
func signalsForService(serviceName string, signals []domain.Signal) []domain.Signal {
	if strings.TrimSpace(serviceName) == "" {
		return signals
	}
	filtered := make([]domain.Signal, 0, len(signals))
	for _, signal := range signals {
		if signal.ServiceName == serviceName {
			filtered = append(filtered, signal)
		}
	}
	return filtered
}

// filterHTTPSignals usa a presença do código de resposta como critério para não misturar spans de dependências nas métricas HTTP.
func filterHTTPSignals(signals []domain.Signal) []domain.Signal {
	filtered := make([]domain.Signal, 0, len(signals))
	for _, signal := range signals {
		if _, exists := signal.Attributes["http.response.status_code"]; exists {
			filtered = append(filtered, signal)
		}
	}
	return filtered
}

// filterDatabaseSignals considera somente spans que declaram explicitamente o sistema de banco observado.
func filterDatabaseSignals(signals []domain.Signal) []domain.Signal {
	filtered := make([]domain.Signal, 0, len(signals))
	for _, signal := range signals {
		if strings.TrimSpace(signal.Attributes["db.system.name"]) != "" {
			filtered = append(filtered, signal)
		}
	}
	return filtered
}

// databaseTimeouts seleciona somente timeouts explicitamente sinalizados, sem inspecionar SQL bruto.
func databaseTimeouts(signals []domain.Signal) []domain.Signal {
	timeouts := make([]domain.Signal, 0, len(signals))
	for _, signal := range signals {
		errorType := strings.ToLower(signal.Attributes["error.type"])
		statusMessage := strings.ToLower(signal.Attributes["status.message"])
		if strings.Contains(errorType, "timeout") || strings.Contains(statusMessage, "timeout") {
			timeouts = append(timeouts, signal)
		}
	}
	return timeouts
}

func databaseTimeoutSummary(baselineTimeouts, baselineCount, incidentTimeouts, incidentCount int, system string) string {
	if incidentTimeouts == 1 {
		return fmt.Sprintf("1 timeout observado entre %d operações %s; baseline tinha %d de %d operações.", incidentCount, system, baselineTimeouts, baselineCount)
	}
	return fmt.Sprintf("%d de %d operações %s tiveram timeout; baseline tinha %d de %d operações.", incidentTimeouts, incidentCount, system, baselineTimeouts, baselineCount)
}

// errorRate calcula somente respostas 5xx, pois outros códigos HTTP não representam falha de servidor para esta regra inicial.
func errorRate(signals []domain.Signal) float64 {
	errors := 0
	for _, signal := range signals {
		statusCode, err := strconv.Atoi(signal.Attributes["http.response.status_code"])
		if err == nil && statusCode >= 500 && statusCode <= 599 {
			errors++
		}
	}
	return fraction(errors, len(signals))
}

func httpErrorStatuses(signals []domain.Signal) []string {
	statuses := make(map[string]struct{})
	for _, signal := range signals {
		statusCode := strings.TrimSpace(signal.Attributes["http.response.status_code"])
		parsed, err := strconv.Atoi(statusCode)
		if err == nil && parsed >= 500 && parsed <= 599 {
			statuses[statusCode] = struct{}{}
		}
	}
	values := make([]string, 0, len(statuses))
	for statusCode := range statuses {
		values = append(values, statusCode)
	}
	sort.Strings(values)
	return values
}

func databaseSystems(signals []domain.Signal) []string {
	systems := make(map[string]struct{})
	for _, signal := range signals {
		system := strings.TrimSpace(signal.Attributes["db.system.name"])
		if system == "" {
			continue
		}
		if strings.EqualFold(system, "postgresql") {
			system = "PostgreSQL"
		}
		systems[system] = struct{}{}
	}
	values := make([]string, 0, len(systems))
	for system := range systems {
		values = append(values, system)
	}
	sort.Strings(values)
	return values
}

// percentile95 seleciona a observação p95 mais conservadora para uma amostra discreta e ordenada.
func percentile95(signals []domain.Signal) (float64, bool) {
	durations := make([]float64, 0, len(signals))
	for _, signal := range signals {
		duration, exists := signal.Measurements["duration_ms"]
		if exists && duration >= 0 {
			durations = append(durations, duration)
		}
	}
	if len(durations) == 0 {
		return 0, false
	}
	sort.Float64s(durations)
	index := int(math.Ceil(float64(len(durations))*0.95)) - 1
	return durations[index], true
}

// sampleConfidence reduz a confiança quando uma das janelas não contém volume mínimo comparável.
func sampleConfidence(baselineCount, incidentCount int) Confidence {
	if baselineCount < minimumSampleSize || incidentCount < minimumSampleSize {
		return ConfidenceLow
	}
	return ConfidenceHigh
}

// fraction evita divisões por zero quando uma janela ainda não tem sinais do tipo avaliado.
func fraction(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

// signalIDs preserva referências aos sinais originais para que a camada de relatório possa explicar a hipótese.
func signalIDs(signals []domain.Signal) []string {
	ids := make([]string, 0, len(signals))
	for _, signal := range signals {
		ids = append(ids, signal.ID)
	}
	return ids
}

// clamp garante o contrato de score mesmo se uma regra futura alterar sua fórmula interna.
func clamp(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
