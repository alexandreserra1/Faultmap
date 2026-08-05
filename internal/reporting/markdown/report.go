// Package markdown renderiza snapshots persistidos como relatórios legíveis e auditáveis.
package markdown

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

// Render escreve um relatório Markdown determinístico sem recalcular o snapshot recebido.
func Render(writer io.Writer, diagnosis application.PersistedDiagnosis) error {
	var output strings.Builder
	fmt.Fprintf(&output, "# Diagnóstico do incidente `%s`\n\n", inlineCode(diagnosis.Incident.ID))
	fmt.Fprintf(&output, "**Serviço:** %s  \n", markdownText(diagnosis.Incident.ServiceName))
	fmt.Fprintf(&output, "**Status:** %s  \n", markdownText(diagnosis.Incident.Status))
	if diagnosis.MetadataComplete() {
		fmt.Fprintf(
			&output,
			"**Baseline:** %d sinais · %s → %s  \n",
			*diagnosis.BaselineSignalCount,
			markdownTime(*diagnosis.BaselineStart),
			markdownTime(*diagnosis.BaselineEnd),
		)
		fmt.Fprintf(
			&output,
			"**Incidente:** %d sinais · %s → %s\n",
			*diagnosis.IncidentSignalCount,
			markdownTime(diagnosis.Incident.IncidentStart),
			markdownTime(diagnosis.Incident.IncidentEnd),
		)
	} else {
		output.WriteString("**Baseline:** metadados indisponíveis para este snapshot legado.  \n")
		fmt.Fprintf(
			&output,
			"**Janela do incidente:** %s → %s\n",
			markdownTime(diagnosis.Incident.IncidentStart),
			markdownTime(diagnosis.Incident.IncidentEnd),
		)
	}

	renderRanking(&output, diagnosis.Suspects)
	renderFindings(&output, diagnosis.Findings)
	renderLimitations(&output, diagnosis.Findings, diagnosis.Suspects)
	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("escrever relatório Markdown: %w", err)
	}
	return nil
}

func renderRanking(output *strings.Builder, suspects []ranking.Suspect) {
	output.WriteString("\n## Ranking de suspeitos\n")
	if len(suspects) == 0 {
		output.WriteString("\nNenhum suspeito ranqueado.\n")
		return
	}
	ordered := append([]ranking.Suspect(nil), suspects...)
	sort.Slice(ordered, func(first, second int) bool {
		if ordered[first].Score != ordered[second].Score {
			return ordered[first].Score > ordered[second].Score
		}
		return ordered[first].ID < ordered[second].ID
	})
	for index, suspect := range ordered {
		fmt.Fprintf(output, "\n### %d. %s\n\n", index+1, markdownText(suspect.Label))
		fmt.Fprintf(output, "- **Score agregado:** %.2f\n", suspect.Score)
		fmt.Fprintf(output, "- **Confiança:** %s\n", markdownText(string(suspect.Confidence)))
		orderedContributions := append([]ranking.ScoreContribution(nil), suspect.Contributions...)
		sort.Slice(orderedContributions, func(first, second int) bool {
			return orderedContributions[first].RuleID < orderedContributions[second].RuleID
		})
		for _, contribution := range orderedContributions {
			fmt.Fprintf(
				output,
				"- `%s`: %s\n",
				inlineCode(contribution.RuleID),
				markdownText(contribution.Reason),
			)
		}
	}
}

func renderFindings(output *strings.Builder, findings []detection.Finding) {
	output.WriteString("\n## Evidências\n")
	if len(findings) == 0 {
		output.WriteString("\nNenhuma anomalia determinística encontrada.\n")
		return
	}
	ordered := append([]detection.Finding(nil), findings...)
	sort.Slice(ordered, func(first, second int) bool {
		if ordered[first].Score != ordered[second].Score {
			return ordered[first].Score > ordered[second].Score
		}
		return ordered[first].Rule < ordered[second].Rule
	})
	for _, finding := range ordered {
		fmt.Fprintf(output, "\n### `%s`\n\n", inlineCode(finding.Rule))
		fmt.Fprintf(output, "- **Serviço:** %s\n", markdownText(finding.ServiceName))
		fmt.Fprintf(output, "- **Score:** %.2f\n", finding.Score)
		fmt.Fprintf(output, "- **Confiança:** %s\n", markdownText(string(finding.Confidence)))
		orderedEvidence := append([]detection.Evidence(nil), finding.Evidence...)
		sort.Slice(orderedEvidence, func(first, second int) bool {
			return orderedEvidence[first].Summary < orderedEvidence[second].Summary
		})
		for _, evidence := range orderedEvidence {
			fmt.Fprintf(output, "- **Evidência:** %s\n", markdownText(evidence.Summary))
		}
	}
}

func renderLimitations(output *strings.Builder, findings []detection.Finding, suspects []ranking.Suspect) {
	limitations := make(map[string]struct{})
	for _, finding := range findings {
		for _, limitation := range finding.Limitations {
			if trimmed := strings.TrimSpace(limitation); trimmed != "" {
				limitations[trimmed] = struct{}{}
			}
		}
	}
	for _, suspect := range suspects {
		for _, limitation := range suspect.Limitations {
			if trimmed := strings.TrimSpace(limitation); trimmed != "" {
				limitations[trimmed] = struct{}{}
			}
		}
	}
	if len(limitations) == 0 {
		return
	}
	ordered := make([]string, 0, len(limitations))
	for limitation := range limitations {
		ordered = append(ordered, limitation)
	}
	sort.Strings(ordered)
	output.WriteString("\n## Limitações\n\n")
	for _, limitation := range ordered {
		fmt.Fprintf(output, "- %s\n", markdownText(limitation))
	}
}

func markdownTime(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05 MST")
}

func inlineCode(value string) string {
	return strings.ReplaceAll(singleLine(value), "`", "'")
}

func markdownText(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "`", "\\`",
	)
	return replacer.Replace(singleLine(value))
}

func singleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
