// Package ranking agrega findings determinísticos em suspeitos ordenados e auditáveis.
package ranking

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/detection"
)

// Weights define a contribuição máxima de cada classe de evidência no score final.
type Weights struct {
	ErrorRateDelta      float64
	DeploymentProximity float64
	DatabaseEvidence    float64
	GraphProximity      float64
	LatencyDelta        float64
	// LogCorrelation permaneceu configurado e sem uso enquanto o produto não
	// ingeria logs. Ele passa a financiar a regra de correlação de logs.
	LogCorrelation float64
}

// Config define os pesos e o limite de suspeitos devolvidos pelo motor.
type Config struct {
	Weights Weights
	TopN    int
}

// ScoreContribution registra como um finding alterou o score de um suspeito.
type ScoreContribution struct {
	RuleID string
	Value  float64
	Reason string
}

// Suspect representa um serviço priorizado e preserva todas as contribuições e limitações usadas no cálculo.
type Suspect struct {
	// Kind distingue um serviço de um commit. Snapshots gravados antes de
	// existirem outros tipos são lidos como serviço.
	Kind          detection.SubjectKind
	ID            string
	Label         string
	Score         float64
	Confidence    detection.Confidence
	Contributions []ScoreContribution
	Limitations   []string
}

// Rank agrega findings conhecidos por serviço, aplica pesos e devolve o top N em ordem determinística.
func Rank(findings []detection.Finding, config Config) ([]Suspect, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	bySubject := make(map[string]*suspectAccumulator)
	for _, finding := range findings {
		class, known := weightClassForRule(finding.Rule)
		weight := weightForClass(class, config.Weights)
		kind, identifier, label := finding.Subject()
		if !known || weight == 0 || identifier == "" {
			continue
		}

		// A chave combina tipo e identificador: um commit e um serviço podem
		// carregar o mesmo nome sem serem a mesma coisa.
		key := string(kind) + "\x00" + identifier
		accumulator := bySubject[key]
		if accumulator == nil {
			accumulator = &suspectAccumulator{
				kind:          kind,
				identifier:    identifier,
				label:         label,
				serviceName:   strings.TrimSpace(finding.ServiceName),
				byWeightClass: make(map[string]float64),
				confidence:    detection.ConfidenceHigh,
				limitations:   make(map[string]struct{}),
			}
			bySubject[key] = accumulator
		}

		findingScore := clamp(finding.Score)
		value := findingScore * weight
		accumulator.byWeightClass[class] += value
		accumulator.contributions = append(accumulator.contributions, ScoreContribution{
			RuleID: finding.Rule,
			Value:  value,
			Reason: contributionReason(finding, findingScore, weight, value),
		})
		if finding.Confidence == detection.ConfidenceLow {
			accumulator.confidence = detection.ConfidenceLow
		}
		for _, limitation := range finding.Limitations {
			if trimmed := strings.TrimSpace(limitation); trimmed != "" {
				accumulator.limitations[trimmed] = struct{}{}
			}
		}
	}

	// O teto de um commit é o sintoma do serviço onde ele foi implantado, então
	// os serviços precisam estar todos somados antes de qualquer commit ser
	// pontuado.
	measuredByService := make(map[string]float64, len(bySubject))
	for _, accumulator := range bySubject {
		if accumulator.kind == detection.SubjectCommit {
			continue
		}
		measuredByService[accumulator.identifier] = accumulator.measured(config.Weights)
	}

	suspects := make([]Suspect, 0, len(bySubject))
	for _, accumulator := range bySubject {
		score := clamp(accumulator.score(config.Weights, measuredByService))
		// Um sujeito cujo score inteiro foi limitado a zero não é suspeito de
		// nada: só havia proximidade, e proximidade sozinha não acusa ninguém.
		if score == 0 {
			continue
		}
		sort.Slice(accumulator.contributions, func(first, second int) bool {
			return accumulator.contributions[first].RuleID < accumulator.contributions[second].RuleID
		})
		suspects = append(suspects, Suspect{
			Kind:          accumulator.kind,
			ID:            accumulator.identifier,
			Label:         accumulator.label,
			Score:         score,
			Confidence:    accumulator.confidence,
			Contributions: accumulator.contributions,
			Limitations:   sortedSet(accumulator.limitations),
		})
	}

	sort.Slice(suspects, func(first, second int) bool {
		if suspects[first].Score != suspects[second].Score {
			return suspects[first].Score > suspects[second].Score
		}
		return suspects[first].ID < suspects[second].ID
	})
	if len(suspects) > config.TopN {
		suspects = suspects[:config.TopN]
	}
	return suspects, nil
}

type suspectAccumulator struct {
	kind       detection.SubjectKind
	identifier string
	label      string
	// serviceName é o serviço a que o sujeito pertence. Para um serviço é ele
	// mesmo; para um commit é o serviço onde ele foi implantado, que é de quem
	// o teto do commit vem emprestado.
	serviceName string
	// byWeightClass acumula separadamente o que cada classe de peso somou, para
	// que o total de uma classe possa ser limitado ao peso configurado para ela.
	byWeightClass map[string]float64
	confidence    detection.Confidence
	contributions []ScoreContribution
	limitations   map[string]struct{}
}

// score soma as classes já limitadas ao respectivo peso.
//
// Várias regras compartilham a mesma classe de peso — quatro delas dividem
// graph_proximity. Somar livremente faria a evidência estrutural valer mais que
// o aumento de erros apenas por existirem mais regras daquele tipo, e a
// proporção mudaria a cada detector acrescentado, sem ninguém decidir. O teto
// mantém o significado dos pesos configurados no YAML.
//
// As contribuições individuais continuam todas visíveis na explicação: o que é
// limitado é o total da classe, não o que é apresentado.
func (accumulator *suspectAccumulator) score(weights Weights, measuredByService map[string]float64) float64 {
	measured, support := accumulator.split(weights)
	return measured + accumulator.limitSupport(support, measured, measuredByService)
}

// measured devolve só a parcela vinda de evidência medida.
func (accumulator *suspectAccumulator) measured(weights Weights) float64 {
	medido, _ := accumulator.split(weights)
	return medido
}

// split separa o que foi medido do que é apoio, já limitado por classe.
func (accumulator *suspectAccumulator) split(weights Weights) (measured, support float64) {
	for class, accumulated := range accumulator.byWeightClass {
		capped := math.Min(accumulated, weightForClass(class, weights))
		if class == classDeploymentProximity {
			support += capped
			continue
		}
		measured += capped
	}
	return measured, support
}

// limitSupport impede que a evidência de apoio pese mais que a evidência que
// ela apoia.
//
// Proximidade de mudança — deploy ou migração — não é sintoma: ela não mede
// nada do serviço, só constata que algo mudou por perto. Medindo na demo-shop,
// uma latência que regrediu de 4 ms para 12 ms valia 0.07 e a migração recente
// valia 0.20 sozinha: o apoio superava em três vezes tudo o que ele deveria
// apenas sustentar.
//
// A causa está na janela. `1 - idade/24h` dá score máximo a qualquer mudança
// recente, e "houve migração há pouco" carrega muito menos informação em 24
// horas do que em uma. O teto relativo corrige sem inventar constante nova, e se
// ajusta sozinho: sintoma forte, o apoio pesa; sintoma marginal, o apoio fica
// marginal junto.
//
// Sem sintoma algum o apoio vira zero, e o serviço deixa de ser suspeito. Isso
// fecha estruturalmente a exposição que deployment_proximity sempre teve —
// disparar sozinho em sistema saudável quando houve deploy na última hora —, que
// nunca apareceu porque nenhum cenário do modo difícil ingere deployments.
//
// O commit não é isento: o teto dele é o do serviço onde foi implantado.
//
// Isentá-lo foi a primeira tentativa, pelo raciocínio de que um commit só tem
// evidência de mudança por construção — nasce do mesmo finding que acusa o
// serviço — e capá-lo contra a própria evidência o zeraria sempre. O mecanismo
// estava certo e a conclusão errada: o teto do commit não é o dele, é o do
// serviço. A acusação do commit deriva da do serviço.
//
// O cenário deploy-inofensivo pegou o estrago: num sistema saudável com um
// deploy recente, o commit aparecia em PRIMEIRO lugar, acima de um serviço com
// regressão de latência medida. Com o teto emprestado do serviço, um commit
// implantado em serviço saudável deixa de ser suspeito, e um implantado em
// serviço que está falhando continua sendo apontado — que é a informação mais
// acionável de um incidente causado por deploy. O teto compara sintoma com apoio, e o commit não é um
// serviço com sintomas: ele é a mudança.
//
// O custo é assumido: uma migração que foi a causa única, com sintoma pequeno
// mas real, fica presa ao tamanho do sintoma. É a mesma troca conservadora de
// exceedsSamplingNoise — preferir calar sinal fraco a apresentar ruído como
// evidência.
func (accumulator *suspectAccumulator) limitSupport(
	support, measured float64,
	measuredByService map[string]float64,
) float64 {
	if accumulator.kind == detection.SubjectCommit {
		return math.Min(support, measuredByService[accumulator.serviceName])
	}
	return math.Min(support, measured)
}

// Validate rejeita limites e pesos que violariam o contrato normalizado antes de qualquer processamento.
func (config Config) Validate() error {
	if config.TopN <= 0 {
		return fmt.Errorf("ranquear suspeitos: top N deve ser maior que zero")
	}
	weights := []float64{
		config.Weights.ErrorRateDelta,
		config.Weights.DeploymentProximity,
		config.Weights.DatabaseEvidence,
		config.Weights.GraphProximity,
		config.Weights.LatencyDelta,
	}
	for _, weight := range weights {
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 || weight > 1 {
			return fmt.Errorf("ranquear suspeitos: pesos devem estar entre 0 e 1")
		}
	}
	return nil
}

// Classes de peso. Elas correspondem aos campos do YAML e existem para que
// várias regras da mesma natureza compartilhem um orçamento comum de score.
const (
	classErrorRate           = "error_rate_delta"
	classLatency             = "latency_delta"
	classDatabaseEvidence    = "database_evidence"
	classGraphProximity      = "graph_proximity"
	classDeploymentProximity = "deployment_proximity"
	classLogCorrelation      = "log_correlation"
)

// weightClassForRule associa cada regra à classe de peso que a financia.
func weightClassForRule(rule string) (string, bool) {
	switch rule {
	case detection.RuleErrorRateDelta:
		return classErrorRate, true
	case detection.RuleLatencyDelta:
		return classLatency, true
	// A cauda divide a classe com o p95 porque as duas descrevem o mesmo banco
	// sob a mesma janela: uma degradação uniforme dispara as duas, e o teto por
	// classe impede que o mesmo fato seja pago duas vezes.
	case detection.RuleDatabaseTimeout, detection.RuleDatabaseError,
		detection.RuleDatabaseLatencyDelta, detection.RuleDatabaseLatencyTail:
		return classDatabaseEvidence, true
	case detection.RuleTraceCorrelation, detection.RuleRetryStorm,
		detection.RuleDependencyFailure, detection.RuleTraceBreak:
		return classGraphProximity, true
	// A mudança de schema divide a classe com o deployment porque, na prática,
	// costuma ser o mesmo evento: a migração acompanha o deploy. Com pesos
	// separados, um único deploy com migração somaria duas vezes e passaria à
	// frente de um serviço que está de fato falhando. O teto por classe resolve
	// isso sem que o produto precise decidir qual dos dois sinais é "o real".
	case detection.RuleDeploymentProximity, detection.RuleVersionRegression,
		detection.RuleSchemaChangeProximity:
		return classDeploymentProximity, true
	case detection.RuleLogCorrelation:
		return classLogCorrelation, true
	default:
		return "", false
	}
}

func weightForClass(class string, weights Weights) float64 {
	switch class {
	case classErrorRate:
		return weights.ErrorRateDelta
	case classLatency:
		return weights.LatencyDelta
	case classDatabaseEvidence:
		return weights.DatabaseEvidence
	case classGraphProximity:
		return weights.GraphProximity
	case classDeploymentProximity:
		return weights.DeploymentProximity
	case classLogCorrelation:
		return weights.LogCorrelation
	default:
		return 0
	}
}

func contributionReason(finding detection.Finding, findingScore, weight, value float64) string {
	reason := fmt.Sprintf("score %.2f × peso %.2f = %.2f", findingScore, weight, value)
	if len(finding.Evidence) > 0 && strings.TrimSpace(finding.Evidence[0].Summary) != "" {
		reason += "; " + strings.TrimSpace(finding.Evidence[0].Summary)
	}
	return reason
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
