package jsonreport

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/faultmap/faultmap/internal/application"
)

const incidentSummarySchemaVersion = "1"

type incidentSummaryDocument struct {
	SchemaVersion  string          `json:"schema_version"`
	IncidentID     string          `json:"incident_id"`
	GeneratedAt    string          `json:"generated_at"`
	ServiceName    string          `json:"service_name"`
	Environment    string          `json:"environment"`
	Status         string          `json:"status"`
	Baseline       *window         `json:"baseline"`
	IncidentWindow window          `json:"incident_window"`
	FindingCount   int             `json:"finding_count"`
	SuspectCount   int             `json:"suspect_count"`
	PrimarySuspect *primarySuspect `json:"primary_suspect"`
}

// primarySuspect carrega somente a identificação e o peso do topo do ranking:
// contribuições e evidências pertencem a ranking.json e ao relatório completo.
type primarySuspect struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Score      float64 `json:"score"`
	Confidence string  `json:"confidence"`
}

// RenderIncidentSummary escreve o artefato incident-summary.json: um resumo
// enxuto do snapshot, pensado para leitura rápida e para integrações que só
// precisam saber qual incidente foi investigado e qual é o suspeito principal.
// Baseline e suspeito principal são nulos quando o snapshot não os possui, para
// distinguir ausência de dado de valor zero. O instante é injetado pelo chamador
// para preservar o determinismo do artefato.
func RenderIncidentSummary(writer io.Writer, diagnosis application.PersistedDiagnosis, generatedAt time.Time) error {
	document := incidentSummaryDocument{
		SchemaVersion: incidentSummarySchemaVersion,
		IncidentID:    diagnosis.Incident.ID,
		GeneratedAt:   reportTime(generatedAt),
		ServiceName:   diagnosis.Incident.ServiceName,
		Environment:   diagnosis.Incident.Environment,
		Status:        diagnosis.Incident.Status,
		IncidentWindow: window{
			StartedAt:   reportTime(diagnosis.Incident.IncidentStart),
			EndedAt:     reportTime(diagnosis.Incident.IncidentEnd),
			SignalCount: copyIntPointer(diagnosis.IncidentSignalCount),
		},
		FindingCount: len(diagnosis.Findings),
		SuspectCount: len(diagnosis.Suspects),
	}
	if diagnosis.BaselineStart != nil && diagnosis.BaselineEnd != nil {
		document.Baseline = &window{
			StartedAt:   reportTime(*diagnosis.BaselineStart),
			EndedAt:     reportTime(*diagnosis.BaselineEnd),
			SignalCount: copyIntPointer(diagnosis.BaselineSignalCount),
		}
	}
	if ordered := orderedSuspects(diagnosis.Suspects); len(ordered) > 0 {
		document.PrimarySuspect = &primarySuspect{
			ID:         ordered[0].ID,
			Label:      ordered[0].Label,
			Score:      ordered[0].Score,
			Confidence: string(ordered[0].Confidence),
		}
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return fmt.Errorf("escrever resumo do incidente JSON: %w", err)
	}
	return nil
}
