package jsonreport

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/faultmap/faultmap/internal/application"
)

const rankingSchemaVersion = "1"

type rankingDocument struct {
	SchemaVersion string    `json:"schema_version"`
	IncidentID    string    `json:"incident_id"`
	GeneratedAt   string    `json:"generated_at"`
	Suspects      []suspect `json:"suspects"`
}

// RenderRanking escreve o artefato ranking.json: apenas o ranking de suspeitos,
// sem os findings brutos do relatório completo. O instante é recebido como
// parâmetro — e não lido do relógio — para que o artefato seja reproduzível
// byte a byte a partir do mesmo snapshot.
func RenderRanking(writer io.Writer, diagnosis application.PersistedDiagnosis, generatedAt time.Time) error {
	document := rankingDocument{
		SchemaVersion: rankingSchemaVersion,
		IncidentID:    diagnosis.Incident.ID,
		GeneratedAt:   reportTime(generatedAt),
		Suspects:      make([]suspect, 0, len(diagnosis.Suspects)),
	}
	for _, source := range orderedSuspects(diagnosis.Suspects) {
		document.Suspects = append(document.Suspects, reportSuspect(source))
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return fmt.Errorf("escrever ranking JSON: %w", err)
	}
	return nil
}
