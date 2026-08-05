package terminal

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

// RenderDiagnosis escreve hipóteses, evidências e limitações de um incidente de forma auditável.
func RenderDiagnosis(
	writer io.Writer,
	serviceName string,
	baselineSignalCount int,
	incidentSignalCount int,
	findings []detection.Finding,
	suspects []ranking.Suspect,
) error {
	var output strings.Builder
	fmt.Fprintf(&output, "Diagnóstico do incidente — %s\n\n", serviceName)
	fmt.Fprintf(&output, "Baseline: %d sinais · Incidente: %d sinais\n", baselineSignalCount, incidentSignalCount)
	renderDiagnosisAnalysis(&output, findings, suspects)

	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("escrever diagnóstico no terminal: %w", err)
	}
	return nil
}

// renderDiagnosisAnalysis compartilha ranking, findings e limitações entre o
// diagnóstico recém-calculado e a leitura de um snapshot persistido.
func renderDiagnosisAnalysis(output *strings.Builder, findings []detection.Finding, suspects []ranking.Suspect) {
	if len(findings) == 0 {
		output.WriteString("\nNenhuma anomalia determinística foi encontrada nas janelas informadas.\n")
		return
	}
	renderSuspectRanking(output, suspects)
	orderedFindings := append([]detection.Finding(nil), findings...)
	sort.Slice(orderedFindings, func(firstIndex, secondIndex int) bool {
		if orderedFindings[firstIndex].Score != orderedFindings[secondIndex].Score {
			return orderedFindings[firstIndex].Score > orderedFindings[secondIndex].Score
		}
		if orderedFindings[firstIndex].Rule != orderedFindings[secondIndex].Rule {
			return orderedFindings[firstIndex].Rule < orderedFindings[secondIndex].Rule
		}
		return findingSortKey(orderedFindings[firstIndex]) < findingSortKey(orderedFindings[secondIndex])
	})
	generalLimitations := repeatedLimitations(orderedFindings)

	output.WriteString("\nEvidências:\n")
	for _, finding := range orderedFindings {
		fmt.Fprintf(output, "- %s\n", ruleLabel(finding.Rule))
		fmt.Fprintf(output, "  ID da regra: %s\n", finding.Rule)
		fmt.Fprintf(output, "  Score: %.2f\n", finding.Score)
		fmt.Fprintf(output, "  Confiança: %s\n", finding.Confidence)
		orderedEvidence := append([]detection.Evidence(nil), finding.Evidence...)
		sort.Slice(orderedEvidence, func(firstIndex, secondIndex int) bool {
			return orderedEvidence[firstIndex].Summary < orderedEvidence[secondIndex].Summary
		})
		for _, evidence := range orderedEvidence {
			fmt.Fprintf(output, "  Evidência: %s\n", evidence.Summary)
		}
		for _, limitation := range specificLimitations(finding.Limitations, generalLimitations) {
			fmt.Fprintf(output, "  Limitação específica: %s\n", limitation)
		}
	}
	if len(generalLimitations) > 0 {
		output.WriteString("\nLimitações gerais:\n")
		for _, limitation := range sortedKeys(generalLimitations) {
			fmt.Fprintf(output, "- %s\n", limitation)
		}
	}
}

// renderSuspectRanking apresenta o cálculo agregado antes das evidências para
// deixar explícito por que cada serviço recebeu sua posição.
func renderSuspectRanking(output *strings.Builder, suspects []ranking.Suspect) {
	if len(suspects) == 0 {
		return
	}
	output.WriteString("\nRanking de suspeitos:\n")
	for index, suspect := range suspects {
		fmt.Fprintf(output, "%d. %s\n", index+1, suspect.Label)
		fmt.Fprintf(output, "   Score agregado: %.2f\n", suspect.Score)
		fmt.Fprintf(output, "   Confiança: %s\n", suspect.Confidence)
		output.WriteString("   Contribuições:\n")
		for _, contribution := range suspect.Contributions {
			fmt.Fprintf(output, "   - %s: %s\n", contribution.RuleID, contribution.Reason)
		}
	}
}

func ruleLabel(rule string) string {
	switch rule {
	case detection.RuleErrorRateDelta:
		return "Aumento da taxa de erros HTTP"
	case detection.RuleLatencyDelta:
		return "Aumento da latência HTTP"
	case detection.RuleDatabaseTimeout:
		return "Timeout no PostgreSQL"
	case detection.RuleTraceCorrelation:
		return "Timeout PostgreSQL correlacionado a impacto HTTP"
	case detection.RuleDeploymentProximity:
		return "Deployment próximo ao incidente"
	default:
		return "Hipótese detectada"
	}
}

func repeatedLimitations(findings []detection.Finding) map[string]struct{} {
	occurrences := make(map[string]int)
	for _, finding := range findings {
		seenInFinding := make(map[string]struct{})
		for _, limitation := range finding.Limitations {
			if limitation == "" {
				continue
			}
			seenInFinding[limitation] = struct{}{}
		}
		for limitation := range seenInFinding {
			occurrences[limitation]++
		}
	}

	repeated := make(map[string]struct{})
	for limitation, count := range occurrences {
		if count > 1 {
			repeated[limitation] = struct{}{}
		}
	}
	return repeated
}

func specificLimitations(limitations []string, general map[string]struct{}) []string {
	specific := make(map[string]struct{})
	for _, limitation := range limitations {
		if limitation == "" {
			continue
		}
		if _, isGeneral := general[limitation]; !isGeneral {
			specific[limitation] = struct{}{}
		}
	}
	return sortedKeys(specific)
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}

func findingSortKey(finding detection.Finding) string {
	parts := []string{finding.ServiceName, string(finding.Confidence)}
	for _, evidence := range finding.Evidence {
		parts = append(parts, evidence.Summary)
	}
	parts = append(parts, finding.Limitations...)
	sort.Strings(parts[2:])
	return strings.Join(parts, "\x00")
}
