// Package jsonreport serializa snapshots persistidos em um contrato JSON versionado.
package jsonreport

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

const schemaVersion = "1"

type report struct {
	SchemaVersion  string    `json:"schema_version"`
	Incident       incident  `json:"incident"`
	Baseline       *window   `json:"baseline"`
	IncidentWindow window    `json:"incident_window"`
	Findings       []finding `json:"findings"`
	Ranking        []suspect `json:"ranking"`
}

type incident struct {
	ID          string `json:"id"`
	ServiceName string `json:"service_name"`
	Environment string `json:"environment"`
	Status      string `json:"status"`
}

type window struct {
	StartedAt   string `json:"started_at"`
	EndedAt     string `json:"ended_at"`
	SignalCount *int   `json:"signal_count"`
}

type finding struct {
	RuleID      string     `json:"rule_id"`
	ServiceName string     `json:"service_name"`
	Score       float64    `json:"score"`
	Confidence  string     `json:"confidence"`
	Evidence    []evidence `json:"evidence"`
	Limitations []string   `json:"limitations"`
}

type evidence struct {
	Summary       string   `json:"summary"`
	SignalIDs     []string `json:"signal_ids"`
	ChangeIDs     []string `json:"change_ids"`
	BaselineValue float64  `json:"baseline_value"`
	IncidentValue float64  `json:"incident_value"`
}

type suspect struct {
	ID            string         `json:"id"`
	Label         string         `json:"label"`
	Score         float64        `json:"score"`
	Confidence    string         `json:"confidence"`
	Contributions []contribution `json:"contributions"`
	Limitations   []string       `json:"limitations"`
}

type contribution struct {
	RuleID string  `json:"rule_id"`
	Value  float64 `json:"value"`
	Reason string  `json:"reason"`
}

// Render escreve um snapshot em JSON sem expor os nomes implícitos dos tipos
// internos, permitindo evoluir o contrato por schema_version.
func Render(writer io.Writer, diagnosis application.PersistedDiagnosis) error {
	document := newReport(diagnosis)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return fmt.Errorf("escrever relatório JSON: %w", err)
	}
	return nil
}

func newReport(diagnosis application.PersistedDiagnosis) report {
	document := report{
		SchemaVersion: schemaVersion,
		Incident: incident{
			ID:          diagnosis.Incident.ID,
			ServiceName: diagnosis.Incident.ServiceName,
			Environment: diagnosis.Incident.Environment,
			Status:      diagnosis.Incident.Status,
		},
		IncidentWindow: window{
			StartedAt:   reportTime(diagnosis.Incident.IncidentStart),
			EndedAt:     reportTime(diagnosis.Incident.IncidentEnd),
			SignalCount: copyIntPointer(diagnosis.IncidentSignalCount),
		},
		Findings: make([]finding, 0, len(diagnosis.Findings)),
		Ranking:  make([]suspect, 0, len(diagnosis.Suspects)),
	}
	if diagnosis.MetadataComplete() {
		document.Baseline = &window{
			StartedAt:   reportTime(*diagnosis.BaselineStart),
			EndedAt:     reportTime(*diagnosis.BaselineEnd),
			SignalCount: copyIntPointer(diagnosis.BaselineSignalCount),
		}
	}

	orderedFindings := append([]detection.Finding(nil), diagnosis.Findings...)
	sort.Slice(orderedFindings, func(first, second int) bool {
		if orderedFindings[first].Rule != orderedFindings[second].Rule {
			return orderedFindings[first].Rule < orderedFindings[second].Rule
		}
		return orderedFindings[first].ServiceName < orderedFindings[second].ServiceName
	})
	for _, source := range orderedFindings {
		document.Findings = append(document.Findings, reportFinding(source))
	}

	orderedSuspects := append([]ranking.Suspect(nil), diagnosis.Suspects...)
	sort.Slice(orderedSuspects, func(first, second int) bool {
		if orderedSuspects[first].Score != orderedSuspects[second].Score {
			return orderedSuspects[first].Score > orderedSuspects[second].Score
		}
		return orderedSuspects[first].ID < orderedSuspects[second].ID
	})
	for _, source := range orderedSuspects {
		document.Ranking = append(document.Ranking, reportSuspect(source))
	}
	return document
}

func reportFinding(source detection.Finding) finding {
	result := finding{
		RuleID: source.Rule, ServiceName: source.ServiceName, Score: source.Score,
		Confidence:  string(source.Confidence),
		Evidence:    make([]evidence, 0, len(source.Evidence)),
		Limitations: sortedStrings(source.Limitations),
	}
	orderedEvidence := append([]detection.Evidence(nil), source.Evidence...)
	sort.Slice(orderedEvidence, func(first, second int) bool {
		return orderedEvidence[first].Summary < orderedEvidence[second].Summary
	})
	for _, sourceEvidence := range orderedEvidence {
		result.Evidence = append(result.Evidence, evidence{
			Summary:       sourceEvidence.Summary,
			SignalIDs:     sortedStrings(sourceEvidence.SignalIDs),
			ChangeIDs:     sortedStrings(sourceEvidence.ChangeIDs),
			BaselineValue: sourceEvidence.BaselineValue,
			IncidentValue: sourceEvidence.IncidentValue,
		})
	}
	return result
}

func reportSuspect(source ranking.Suspect) suspect {
	result := suspect{
		ID: source.ID, Label: source.Label, Score: source.Score,
		Confidence:    string(source.Confidence),
		Contributions: make([]contribution, 0, len(source.Contributions)),
		Limitations:   sortedStrings(source.Limitations),
	}
	ordered := append([]ranking.ScoreContribution(nil), source.Contributions...)
	sort.Slice(ordered, func(first, second int) bool {
		return ordered[first].RuleID < ordered[second].RuleID
	})
	for _, sourceContribution := range ordered {
		result.Contributions = append(result.Contributions, contribution{
			RuleID: sourceContribution.RuleID,
			Value:  sourceContribution.Value,
			Reason: sourceContribution.Reason,
		})
	}
	return result
}

func reportTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	if result == nil {
		result = []string{}
	}
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
