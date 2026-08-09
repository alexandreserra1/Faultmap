package application

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

// ErrSuspectNotFound diferencia "este suspeito não faz parte do incidente" de
// falhas de infraestrutura, do mesmo modo que ErrIncidentNotFound.
var ErrSuspectNotFound = errors.New("suspeito não encontrado no incidente")

// SuspectContribution liga uma parcela do score às hipóteses que a sustentam.
// Supported falso significa que o snapshot não preservou nenhum finding daquela
// regra para o serviço: a parcela continua declarada, nunca omitida.
type SuspectContribution struct {
	RuleID    string
	Value     float64
	Reason    string
	Supported bool
	Findings  []detection.Finding
}

// SuspectExplanation reúne, sem recalcular nada, tudo que o snapshot persistido
// registrou sobre um suspeito: parcelas do score, evidências com proveniência e
// limitações declaradas.
type SuspectExplanation struct {
	IncidentID    string
	SuspectID     string
	SuspectLabel  string
	Score         float64
	Confidence    detection.Confidence
	Contributions []SuspectContribution
	// UnscoredFindings contém hipóteses registradas para o serviço que não
	// aparecem em nenhuma contribuição — expor isso evita sugerir que o score
	// esgota o que foi observado.
	UnscoredFindings []detection.Finding
	Limitations      []string
}

// ExplainSuspect explica por que um suspeito foi apontado em um incidente já
// persistido. É uma leitura pura do snapshot: nenhum detector ou ranking roda
// aqui, então o resultado é idêntico ao que `incident show` apresentaria.
func ExplainSuspect(diagnosis PersistedDiagnosis, suspectName string) (SuspectExplanation, error) {
	suspectName = strings.TrimSpace(suspectName)
	if suspectName == "" {
		return SuspectExplanation{}, fmt.Errorf("explicar suspeito: nome do suspeito é obrigatório")
	}

	suspect, found := findSuspect(diagnosis.Suspects, suspectName)
	if !found {
		return SuspectExplanation{}, fmt.Errorf(
			"explicar suspeito %q no incidente %q: %w",
			suspectName,
			diagnosis.Incident.ID,
			ErrSuspectNotFound,
		)
	}

	serviceFindings := findingsForService(diagnosis.Findings, suspect.ID)
	contributions, usedFindings := groupContributions(suspect.Contributions, serviceFindings)

	explanation := SuspectExplanation{
		IncidentID:       diagnosis.Incident.ID,
		SuspectID:        suspect.ID,
		SuspectLabel:     suspect.Label,
		Score:            suspect.Score,
		Confidence:       suspect.Confidence,
		Contributions:    contributions,
		UnscoredFindings: remainingFindings(serviceFindings, usedFindings),
		Limitations:      sortedNonEmpty(suspect.Limitations),
	}
	return explanation, nil
}

// findSuspect aceita ID ou rótulo, ignorando caixa e espaços, porque o operador
// digita o nome do serviço como o vê na tela.
func findSuspect(suspects []ranking.Suspect, name string) (ranking.Suspect, bool) {
	wanted := strings.ToLower(name)
	for _, suspect := range suspects {
		if strings.ToLower(strings.TrimSpace(suspect.ID)) == wanted ||
			strings.ToLower(strings.TrimSpace(suspect.Label)) == wanted {
			return suspect, true
		}
	}
	return ranking.Suspect{}, false
}

func findingsForService(findings []detection.Finding, serviceID string) []detection.Finding {
	wanted := strings.ToLower(strings.TrimSpace(serviceID))
	matched := make([]detection.Finding, 0, len(findings))
	for _, finding := range findings {
		if strings.ToLower(strings.TrimSpace(finding.ServiceName)) == wanted {
			matched = append(matched, normalizeFinding(finding))
		}
	}
	sortFindings(matched)
	return matched
}

// groupContributions associa cada parcela do score aos findings da mesma regra e
// devolve os índices já consumidos, para que nada seja contado duas vezes.
func groupContributions(
	contributions []ranking.ScoreContribution,
	serviceFindings []detection.Finding,
) ([]SuspectContribution, map[int]struct{}) {
	used := make(map[int]struct{})
	grouped := make([]SuspectContribution, 0, len(contributions))
	for _, contribution := range contributions {
		related := make([]detection.Finding, 0, len(serviceFindings))
		for index, finding := range serviceFindings {
			if finding.Rule != contribution.RuleID {
				continue
			}
			related = append(related, finding)
			used[index] = struct{}{}
		}
		grouped = append(grouped, SuspectContribution{
			RuleID:    contribution.RuleID,
			Value:     contribution.Value,
			Reason:    contribution.Reason,
			Supported: len(related) > 0,
			Findings:  related,
		})
	}
	sort.SliceStable(grouped, func(first, second int) bool {
		if grouped[first].Value != grouped[second].Value {
			return grouped[first].Value > grouped[second].Value
		}
		return grouped[first].RuleID < grouped[second].RuleID
	})
	return grouped, used
}

func remainingFindings(serviceFindings []detection.Finding, used map[int]struct{}) []detection.Finding {
	remaining := make([]detection.Finding, 0, len(serviceFindings))
	for index, finding := range serviceFindings {
		if _, consumed := used[index]; consumed {
			continue
		}
		remaining = append(remaining, finding)
	}
	return remaining
}

// normalizeFinding copia o finding e ordena evidências e IDs para que a saída
// seja idêntica entre execuções, independentemente da ordem de persistência.
func normalizeFinding(finding detection.Finding) detection.Finding {
	copied := finding
	copied.Evidence = make([]detection.Evidence, len(finding.Evidence))
	for index, evidence := range finding.Evidence {
		normalized := evidence
		normalized.SignalIDs = sortedNonEmpty(evidence.SignalIDs)
		normalized.ChangeIDs = sortedNonEmpty(evidence.ChangeIDs)
		copied.Evidence[index] = normalized
	}
	sort.SliceStable(copied.Evidence, func(first, second int) bool {
		return copied.Evidence[first].Summary < copied.Evidence[second].Summary
	})
	copied.Limitations = sortedNonEmpty(finding.Limitations)
	return copied
}

func sortFindings(findings []detection.Finding) {
	sort.SliceStable(findings, func(first, second int) bool {
		if findings[first].Score != findings[second].Score {
			return findings[first].Score > findings[second].Score
		}
		return findings[first].Rule < findings[second].Rule
	})
}

// sortedNonEmpty devolve uma cópia ordenada e sem entradas vazias ou repetidas.
func sortedNonEmpty(values []string) []string {
	unique := make(map[string]struct{}, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			unique[trimmed] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
