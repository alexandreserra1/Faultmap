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

	byService := make(map[string]*suspectAccumulator)
	for _, finding := range findings {
		weight, known := weightForRule(finding.Rule, config.Weights)
		serviceName := strings.TrimSpace(finding.ServiceName)
		if !known || weight == 0 || serviceName == "" {
			continue
		}

		accumulator := byService[serviceName]
		if accumulator == nil {
			accumulator = &suspectAccumulator{
				confidence:  detection.ConfidenceHigh,
				limitations: make(map[string]struct{}),
			}
			byService[serviceName] = accumulator
		}

		findingScore := clamp(finding.Score)
		value := findingScore * weight
		accumulator.score += value
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

	suspects := make([]Suspect, 0, len(byService))
	for serviceName, accumulator := range byService {
		sort.Slice(accumulator.contributions, func(first, second int) bool {
			return accumulator.contributions[first].RuleID < accumulator.contributions[second].RuleID
		})
		suspects = append(suspects, Suspect{
			ID:            serviceName,
			Label:         serviceName,
			Score:         clamp(accumulator.score),
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
	score         float64
	confidence    detection.Confidence
	contributions []ScoreContribution
	limitations   map[string]struct{}
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

func weightForRule(rule string, weights Weights) (float64, bool) {
	switch rule {
	case detection.RuleErrorRateDelta:
		return weights.ErrorRateDelta, true
	case detection.RuleLatencyDelta:
		return weights.LatencyDelta, true
	case detection.RuleDatabaseTimeout:
		return weights.DatabaseEvidence, true
	case detection.RuleTraceCorrelation:
		return weights.GraphProximity, true
	case detection.RuleRetryStorm:
		return weights.GraphProximity, true
	case detection.RuleDeploymentProximity:
		return weights.DeploymentProximity, true
	default:
		return 0, false
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
