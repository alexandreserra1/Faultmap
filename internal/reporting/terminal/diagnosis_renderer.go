package terminal

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/detection"
)

// RenderDiagnosis escreve hipóteses, evidências e limitações de um incidente de forma auditável.
func RenderDiagnosis(
	writer io.Writer,
	serviceName string,
	baselineSignalCount int,
	incidentSignalCount int,
	findings []detection.Finding,
) error {
	var output strings.Builder
	fmt.Fprintf(&output, "Diagnóstico do incidente — %s\n\n", serviceName)
	fmt.Fprintf(&output, "Baseline: %d sinais · Incidente: %d sinais\n", baselineSignalCount, incidentSignalCount)

	if len(findings) == 0 {
		output.WriteString("\nNenhuma anomalia determinística foi encontrada nas janelas informadas.\n")
		if _, err := io.WriteString(writer, output.String()); err != nil {
			return fmt.Errorf("escrever diagnóstico no terminal: %w", err)
		}
		return nil
	}

	orderedFindings := append([]detection.Finding(nil), findings...)
	sort.Slice(orderedFindings, func(firstIndex, secondIndex int) bool {
		if orderedFindings[firstIndex].Score != orderedFindings[secondIndex].Score {
			return orderedFindings[firstIndex].Score > orderedFindings[secondIndex].Score
		}
		return orderedFindings[firstIndex].Rule < orderedFindings[secondIndex].Rule
	})

	output.WriteString("\nEvidências:\n")
	for _, finding := range orderedFindings {
		fmt.Fprintf(&output, "- %s · score %.2f\n", finding.Rule, finding.Score)
		fmt.Fprintf(&output, "  Confiança: %s\n", finding.Confidence)
		for _, evidence := range finding.Evidence {
			fmt.Fprintf(&output, "  Evidência: %s\n", evidence.Summary)
		}
		for _, limitation := range finding.Limitations {
			fmt.Fprintf(&output, "  Limitação: %s\n", limitation)
		}
	}

	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("escrever diagnóstico no terminal: %w", err)
	}
	return nil
}
