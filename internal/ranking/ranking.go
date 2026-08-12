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

	suspects := make([]Suspect, 0, len(bySubject))
	for _, accumulator := range bySubject {
		sort.Slice(accumulator.contributions, func(first, second int) bool {
			return accumulator.contributions[first].RuleID < accumulator.contributions[second].RuleID
		})
		suspects = append(suspects, Suspect{
			Kind:          accumulator.kind,
			ID:            accumulator.identifier,
			Label:         accumulator.label,
			Score:         clamp(accumulator.score(config.Weights)),
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
func (accumulator *suspectAccumulator) score(weights Weights) float64 {
	total := 0.0
	for class, accumulated := range accumulator.byWeightClass {
		total += math.Min(accumulated, weightForClass(class, weights))
	}
	return total
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
)

// weightClassForRule associa cada regra à classe de peso que a financia.
func weightClassForRule(rule string) (string, bool) {
	switch rule {
	case detection.RuleErrorRateDelta:
		return classErrorRate, true
	case detection.RuleLatencyDelta:
		return classLatency, true
	case detection.RuleDatabaseTimeout, detection.RuleDatabaseError,
		detection.RuleDatabaseLatencyDelta:
		return classDatabaseEvidence, true
	case detection.RuleTraceCorrelation, detection.RuleRetryStorm,
		detection.RuleDependencyFailure, detection.RuleTraceBreak:
		return classGraphProximity, true
	case detection.RuleDeploymentProximity, detection.RuleVersionRegression:
		return classDeploymentProximity, true
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
