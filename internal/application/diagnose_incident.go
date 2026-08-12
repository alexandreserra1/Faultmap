package application

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/detection"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
	"github.com/faultmap/faultmap/internal/ranking"
)

// Diagnosis reúne as janelas, contagens e hipóteses produzidas para uma investigação de incidente.
type Diagnosis struct {
	ID                  string
	ServiceName         string
	Environment         string
	Windows             incidentdomain.InvestigationWindow
	BaselineSignalCount int
	IncidentSignalCount int
	Findings            []detection.Finding
	Suspects            []ranking.Suspect
	// Scope registra quais serviços foram comparados e como foram descobertos.
	// Fica vazio nas investigações de serviço único anteriores à expansão.
	Scope DiagnosisScope
}

// DiagnosisID deriva uma identidade estável do serviço e das janelas UTC para
// que retries da mesma investigação não criem incidentes duplicados.
func DiagnosisID(serviceName string, windows incidentdomain.InvestigationWindow) string {
	return diagnosisID(serviceName, "", windows)
}

func diagnosisID(serviceName, environment string, windows incidentdomain.InvestigationWindow) string {
	parts := []string{
		strings.TrimSpace(serviceName),
	}
	if environment = strings.TrimSpace(environment); environment != "" {
		parts = append(parts, environment)
	}
	parts = append(parts,
		windows.Baseline.Start.UTC().Format(time.RFC3339Nano),
		windows.Baseline.End.UTC().Format(time.RFC3339Nano),
		windows.Incident.Start.UTC().Format(time.RFC3339Nano),
		windows.Incident.End.UTC().Format(time.RFC3339Nano),
	)
	canonical := strings.Join(parts, "\x00")
	digest := sha256.Sum256([]byte(canonical))
	return "inc_" + hex.EncodeToString(digest[:12])
}
