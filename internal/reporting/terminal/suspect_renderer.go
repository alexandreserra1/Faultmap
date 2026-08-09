package terminal

import (
	"fmt"
	"io"
	"strings"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
)

// RenderSuspectExplanation escreve por que um suspeito foi apontado: cada
// parcela do score, as evidências que a sustentam com seus IDs de origem e as
// limitações declaradas. A saída descreve correlações observadas e nunca afirma
// causalidade.
func RenderSuspectExplanation(writer io.Writer, explanation application.SuspectExplanation) error {
	var output strings.Builder
	fmt.Fprintf(&output, "Por que %s foi apontado — incidente %s\n\n", explanation.SuspectLabel, explanation.IncidentID)
	fmt.Fprintf(&output, "Score agregado: %.2f\n", explanation.Score)
	if explanation.Confidence != "" {
		fmt.Fprintf(&output, "Confiança: %s\n", explanation.Confidence)
	}

	if len(explanation.Contributions) == 0 {
		output.WriteString("\nNenhuma contribuição de score foi registrada para este suspeito no snapshot.\n")
	} else {
		output.WriteString("\nContribuições para o score:\n")
		for _, contribution := range explanation.Contributions {
			fmt.Fprintf(&output, "- %s\n", ruleLabel(contribution.RuleID))
			fmt.Fprintf(&output, "  ID da regra: %s\n", contribution.RuleID)
			fmt.Fprintf(&output, "  Valor somado: %.2f\n", contribution.Value)
			if contribution.Reason != "" {
				fmt.Fprintf(&output, "  Cálculo: %s\n", contribution.Reason)
			}
			if !contribution.Supported {
				// Declarar a lacuna é preferível a escondê-la: o score existe,
				// mas o snapshot não guardou o finding que o justificaria.
				output.WriteString("  Sem evidência registrada no snapshot para esta contribuição.\n")
				continue
			}
			for _, finding := range contribution.Findings {
				renderSuspectFinding(&output, finding, "  ")
			}
		}
	}

	if len(explanation.UnscoredFindings) > 0 {
		output.WriteString("\nHipóteses observadas que não somaram ao score:\n")
		for _, finding := range explanation.UnscoredFindings {
			fmt.Fprintf(&output, "- %s\n", ruleLabel(finding.Rule))
			fmt.Fprintf(&output, "  ID da regra: %s\n", finding.Rule)
			renderSuspectFinding(&output, finding, "  ")
		}
	}

	if len(explanation.Limitations) > 0 {
		output.WriteString("\nLimitações declaradas:\n")
		for _, limitation := range explanation.Limitations {
			fmt.Fprintf(&output, "- %s\n", limitation)
		}
	}
	output.WriteString("\nEstas são correlações observadas nas janelas comparadas, não uma afirmação de causa.\n")

	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("escrever explicação do suspeito no terminal: %w", err)
	}
	return nil
}

// renderSuspectFinding imprime score, confiança, evidências e proveniência de um
// finding. A ordenação já vem estável do caso de uso.
func renderSuspectFinding(output *strings.Builder, finding detection.Finding, indent string) {
	fmt.Fprintf(output, "%sScore do finding: %.2f · Confiança: %s\n", indent, finding.Score, finding.Confidence)
	if len(finding.Evidence) == 0 {
		fmt.Fprintf(output, "%sSem evidência registrada no snapshot para este finding.\n", indent)
	}
	for _, evidence := range finding.Evidence {
		fmt.Fprintf(output, "%sEvidência: %s\n", indent, evidence.Summary)
		fmt.Fprintf(output, "%s  Baseline: %.4f · Incidente: %.4f\n", indent, evidence.BaselineValue, evidence.IncidentValue)
		if len(evidence.SignalIDs) > 0 {
			fmt.Fprintf(output, "%s  %s\n", indent, provenanceSummary("sinal", "sinais", evidence.SignalIDs))
		}
		if len(evidence.ChangeIDs) > 0 {
			fmt.Fprintf(output, "%s  %s\n", indent, provenanceSummary("mudança", "mudanças", evidence.ChangeIDs))
		}
	}
	for _, limitation := range finding.Limitations {
		fmt.Fprintf(output, "%sLimitação: %s\n", indent, limitation)
	}
}

// maxProvenanceSample limita quantos identificadores aparecem na explicação.
// Uma evidência sustentada por dezenas de spans despejava todos eles em uma
// única linha e tornava ilegível justamente a parte que deveria ajudar.
const maxProvenanceSample = 3

// provenanceSummary apresenta uma amostra dos identificadores e a contagem
// total. Nada é perdido: a proveniência completa continua em ranking.json e no
// relatório JSON, que existem para consumo por ferramenta, não para leitura.
func provenanceSummary(singular, plural string, identifiers []string) string {
	total := len(identifiers)
	label := plural
	if total == 1 {
		label = singular
	}
	if total <= maxProvenanceSample {
		return fmt.Sprintf("%d %s: %s", total, label, strings.Join(identifiers, ", "))
	}
	return fmt.Sprintf(
		"%d %s, incluindo: %s",
		total, label, strings.Join(identifiers[:maxProvenanceSample], ", "),
	)
}
