// Package timeline serializa um snapshot persistido como uma sequência
// cronológica de eventos, sem reexecutar detectores nem reler telemetria.
package timeline

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
)

const schemaVersion = "1"

// findingsAreNotTimestamped explica por que os findings compartilham o mesmo
// instante. O snapshot grava as janelas da investigação, não o momento em que
// cada evidência apareceu; ancorá-los no início do incidente é a aproximação
// honesta possível e precisa ficar declarada no artefato.
const findingsAreNotTimestamped = "Findings não possuem instante próprio no snapshot: eles são ancorados ao início da janela do incidente."

const missingBaselineLimitation = "Metadados de baseline ausentes neste snapshot: os eventos da janela baseline não puderam ser reconstruídos."

type document struct {
	SchemaVersion string   `json:"schema_version"`
	IncidentID    string   `json:"incident_id"`
	ServiceName   string   `json:"service_name"`
	Environment   string   `json:"environment"`
	Events        []event  `json:"events"`
	Limitations   []string `json:"limitations"`
}

type event struct {
	At          string   `json:"at"`
	Type        string   `json:"type"`
	Summary     string   `json:"summary"`
	TimeSource  string   `json:"time_source"`
	RuleID      string   `json:"rule_id,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	SignalIDs   []string `json:"signal_ids,omitempty"`
	ChangeIDs   []string `json:"change_ids,omitempty"`
	SignalCount *int     `json:"signal_count,omitempty"`
}

// ordered mantém a posição pretendida de cada evento quando os instantes
// empatam, preservando uma ordem estável entre execuções.
type ordered struct {
	event    event
	instant  time.Time
	sequence int
	tieBreak string
}

// Render escreve o timeline.json do snapshot recebido. A saída é determinística
// e derivada apenas do que foi persistido, sem consultar sinais novamente.
func Render(writer io.Writer, diagnosis application.PersistedDiagnosis) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(newDocument(diagnosis)); err != nil {
		return fmt.Errorf("escrever timeline JSON: %w", err)
	}
	return nil
}

func newDocument(diagnosis application.PersistedDiagnosis) document {
	incidentStart := diagnosis.Incident.IncidentStart
	incidentEnd := diagnosis.Incident.IncidentEnd
	entries := make([]ordered, 0, len(diagnosis.Findings)+5)
	limitations := []string{findingsAreNotTimestamped}

	if diagnosis.BaselineStart != nil && diagnosis.BaselineEnd != nil {
		entries = append(entries,
			ordered{
				instant: *diagnosis.BaselineStart, sequence: 0, tieBreak: "baseline_window_start",
				event: event{
					Type: "baseline_window_start", Summary: "Início da janela baseline.",
					TimeSource: "snapshot", SignalCount: copyIntPointer(diagnosis.BaselineSignalCount),
				},
			},
			ordered{
				instant: *diagnosis.BaselineEnd, sequence: 1, tieBreak: "baseline_window_end",
				event: event{
					Type: "baseline_window_end", Summary: "Fim da janela baseline.", TimeSource: "snapshot",
				},
			},
		)
	} else {
		limitations = append(limitations, missingBaselineLimitation)
	}

	entries = append(entries, ordered{
		instant: incidentStart, sequence: 2, tieBreak: "incident_window_start",
		event: event{
			Type: "incident_window_start", Summary: "Início da janela do incidente.",
			TimeSource: "snapshot", SignalCount: copyIntPointer(diagnosis.IncidentSignalCount),
		},
	})

	for _, finding := range diagnosis.Findings {
		entries = append(entries, ordered{
			instant: incidentStart, sequence: 3, tieBreak: finding.Rule + "\x00" + findingSummary(finding),
			event: event{
				Type: "finding", Summary: findingSummary(finding), TimeSource: "incident_window_start",
				RuleID: finding.Rule, Confidence: string(finding.Confidence),
				SignalIDs: sortedStrings(collectSignalIDs(finding)),
				ChangeIDs: sortedStrings(collectChangeIDs(finding)),
			},
		})
	}

	entries = append(entries,
		ordered{
			instant: incidentEnd, sequence: 4, tieBreak: "incident_window_end",
			event: event{
				Type: "incident_window_end", Summary: "Fim da janela do incidente.", TimeSource: "snapshot",
			},
		},
		ordered{
			instant: incidentEnd, sequence: 5, tieBreak: "diagnosis",
			event: event{
				Type:       "diagnosis",
				Summary:    fmt.Sprintf("Diagnóstico %s registrado com %d suspeito(s).", diagnosis.Incident.ID, len(diagnosis.Suspects)),
				TimeSource: "snapshot",
			},
		},
	)

	sort.SliceStable(entries, func(first, second int) bool {
		if !entries[first].instant.Equal(entries[second].instant) {
			return entries[first].instant.Before(entries[second].instant)
		}
		if entries[first].sequence != entries[second].sequence {
			return entries[first].sequence < entries[second].sequence
		}
		return entries[first].tieBreak < entries[second].tieBreak
	})

	result := document{
		SchemaVersion: schemaVersion,
		IncidentID:    diagnosis.Incident.ID,
		ServiceName:   diagnosis.Incident.ServiceName,
		Environment:   diagnosis.Incident.Environment,
		Events:        make([]event, 0, len(entries)),
		Limitations:   limitations,
	}
	for _, entry := range entries {
		entry.event.At = entry.instant.UTC().Format(time.RFC3339Nano)
		result.Events = append(result.Events, entry.event)
	}
	return result
}

func findingSummary(finding detection.Finding) string {
	for _, evidence := range finding.Evidence {
		if evidence.Summary != "" {
			return evidence.Summary
		}
	}
	return finding.Rule
}

func collectSignalIDs(finding detection.Finding) []string {
	ids := make([]string, 0)
	for _, evidence := range finding.Evidence {
		ids = append(ids, evidence.SignalIDs...)
	}
	return ids
}

func collectChangeIDs(finding detection.Finding) []string {
	ids := make([]string, 0)
	for _, evidence := range finding.Evidence {
		ids = append(ids, evidence.ChangeIDs...)
	}
	return ids
}

func sortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func copyIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}
