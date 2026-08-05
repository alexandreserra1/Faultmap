package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
)

const maxIncidentListLimit = 1_000

// ErrIncidentNotFound diferencia ausência de snapshot de falhas de infraestrutura.
var ErrIncidentNotFound = errors.New("incidente não encontrado")

// IncidentSummary contém somente os campos necessários à listagem limitada.
type IncidentSummary struct {
	ID            string
	ServiceName   string
	Status        string
	IncidentStart time.Time
	IncidentEnd   time.Time
}

// PersistedDiagnosis representa um snapshot recuperado. Ponteiros preservam a
// diferença entre zero real e metadata inexistente em registros legados.
type PersistedDiagnosis struct {
	Incident            IncidentSummary
	BaselineStart       *time.Time
	BaselineEnd         *time.Time
	BaselineSignalCount *int
	IncidentSignalCount *int
	Findings            []detection.Finding
	Suspects            []ranking.Suspect
}

// MetadataComplete informa se o snapshot possui janelas e contagens suficientes para reprodução fiel.
func (diagnosis PersistedDiagnosis) MetadataComplete() bool {
	return diagnosis.BaselineStart != nil &&
		diagnosis.BaselineEnd != nil &&
		diagnosis.BaselineSignalCount != nil &&
		diagnosis.IncidentSignalCount != nil
}

// IncidentHistoryReader define as duas consultas delimitadas do histórico persistido.
type IncidentHistoryReader interface {
	List(ctx context.Context, limit int) ([]IncidentSummary, error)
	Get(ctx context.Context, id string) (PersistedDiagnosis, error)
}

// ListIncidents valida o limite antes de delegar a leitura ordenada ao repositório.
func ListIncidents(ctx context.Context, limit int, reader IncidentHistoryReader) ([]IncidentSummary, error) {
	if limit <= 0 || limit > maxIncidentListLimit {
		return nil, fmt.Errorf("listar incidentes: limit deve estar entre 1 e %d", maxIncidentListLimit)
	}
	incidents, err := reader.List(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("listar incidentes: %w", err)
	}
	return incidents, nil
}

// GetIncident valida o identificador e recupera o snapshot sem recalcular detectores.
func GetIncident(ctx context.Context, id string, reader IncidentHistoryReader) (PersistedDiagnosis, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return PersistedDiagnosis{}, fmt.Errorf("consultar incidente: ID é obrigatório")
	}
	diagnosis, err := reader.Get(ctx, id)
	if err != nil {
		return PersistedDiagnosis{}, fmt.Errorf("consultar incidente %q: %w", id, err)
	}
	return diagnosis, nil
}
