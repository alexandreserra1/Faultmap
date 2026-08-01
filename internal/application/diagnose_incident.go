package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/faultmap/faultmap/internal/detection"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
	"github.com/faultmap/faultmap/internal/ranking"
)

// Diagnosis reúne as janelas, contagens e hipóteses produzidas para uma investigação de incidente.
type Diagnosis struct {
	ServiceName         string
	Windows             incidentdomain.InvestigationWindow
	BaselineSignalCount int
	IncidentSignalCount int
	Findings            []detection.Finding
	Suspects            []ranking.Suspect
}

// DiagnoseIncident compara sinais das janelas baseline e incidente sem acessar infraestrutura diretamente.
func DiagnoseIncident(
	ctx context.Context,
	serviceName string,
	windows incidentdomain.InvestigationWindow,
	limit int,
	rankingConfig ranking.Config,
	reader SignalReader,
) (Diagnosis, error) {
	if strings.TrimSpace(serviceName) == "" {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: serviço é obrigatório")
	}
	if err := windows.Validate(); err != nil {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: janelas inválidas: %w", err)
	}
	if err := rankingConfig.Validate(); err != nil {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: ranking inválido: %w", err)
	}

	baseline, err := ListSignals(ctx, serviceName, windows.Baseline.Start, windows.Baseline.End, limit, reader)
	if err != nil {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: carregar baseline: %w", err)
	}
	incident, err := ListSignals(ctx, serviceName, windows.Incident.Start, windows.Incident.End, limit, reader)
	if err != nil {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: carregar incidente: %w", err)
	}

	findings := detection.Run(detection.Input{
		ServiceName: serviceName,
		Baseline:    baseline,
		Incident:    incident,
	})
	suspects, err := ranking.Rank(findings, rankingConfig)
	if err != nil {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: ranquear suspeitos: %w", err)
	}
	return Diagnosis{
		ServiceName:         serviceName,
		Windows:             windows,
		BaselineSignalCount: len(baseline),
		IncidentSignalCount: len(incident),
		Findings:            findings,
		Suspects:            suspects,
	}, nil
}
