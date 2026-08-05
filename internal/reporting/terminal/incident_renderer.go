package terminal

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/application"
)

// RenderIncidentList escreve uma visão limitada e ordenada dos snapshots persistidos.
func RenderIncidentList(writer io.Writer, incidents []application.IncidentSummary) error {
	if len(incidents) == 0 {
		if _, err := io.WriteString(writer, "Incidentes persistidos — nenhum incidente encontrado.\n"); err != nil {
			return fmt.Errorf("escrever lista de incidentes: %w", err)
		}
		return nil
	}
	ordered := append([]application.IncidentSummary(nil), incidents...)
	sort.Slice(ordered, func(first, second int) bool {
		if !ordered[first].IncidentStart.Equal(ordered[second].IncidentStart) {
			return ordered[first].IncidentStart.After(ordered[second].IncidentStart)
		}
		return ordered[first].ID < ordered[second].ID
	})

	var output strings.Builder
	fmt.Fprintf(&output, "Incidentes persistidos — %d\n\n", len(ordered))
	for _, incident := range ordered {
		fmt.Fprintf(&output, "- %s · %s\n", incident.ID, incident.ServiceName)
		fmt.Fprintf(
			&output,
			"  Status: %s · %s → %s\n",
			incident.Status,
			formatIncidentTime(incident.IncidentStart),
			formatIncidentTime(incident.IncidentEnd),
		)
	}
	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("escrever lista de incidentes: %w", err)
	}
	return nil
}

// RenderPersistedDiagnosis escreve um snapshot sem recalcular detectores e
// declara explicitamente quando metadata legada não está disponível.
func RenderPersistedDiagnosis(writer io.Writer, diagnosis application.PersistedDiagnosis) error {
	var output strings.Builder
	fmt.Fprintf(&output, "Incidente persistido — %s\n", diagnosis.Incident.ID)
	fmt.Fprintf(&output, "Serviço: %s\n", diagnosis.Incident.ServiceName)
	fmt.Fprintf(&output, "Status: %s\n", diagnosis.Incident.Status)
	if diagnosis.MetadataComplete() {
		fmt.Fprintf(
			&output,
			"Baseline: %d sinais · %s → %s\n",
			*diagnosis.BaselineSignalCount,
			formatIncidentTime(*diagnosis.BaselineStart),
			formatIncidentTime(*diagnosis.BaselineEnd),
		)
		fmt.Fprintf(
			&output,
			"Incidente: %d sinais · %s → %s\n",
			*diagnosis.IncidentSignalCount,
			formatIncidentTime(diagnosis.Incident.IncidentStart),
			formatIncidentTime(diagnosis.Incident.IncidentEnd),
		)
	} else {
		output.WriteString("Metadata de baseline e contagens indisponível para este snapshot legado.\n")
		fmt.Fprintf(
			&output,
			"Janela do incidente: %s → %s\n",
			formatIncidentTime(diagnosis.Incident.IncidentStart),
			formatIncidentTime(diagnosis.Incident.IncidentEnd),
		)
	}
	renderDiagnosisAnalysis(&output, diagnosis.Findings, diagnosis.Suspects)
	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("escrever incidente persistido: %w", err)
	}
	return nil
}

func formatIncidentTime(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05 MST")
}
