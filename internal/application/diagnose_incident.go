package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
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
	return diagnoseIncident(ctx, serviceName, "", windows, limit, rankingConfig, reader, nil)
}

// DeploymentReader consulta somente deployments já persistidos do serviço e da janela solicitados.
type DeploymentReader interface {
	ListDeployments(
		ctx context.Context,
		serviceName string,
		environment string,
		start time.Time,
		end time.Time,
		limit int,
	) ([]changedomain.Deployment, error)
}

// DiagnoseIncidentWithDeployments acrescenta mudanças persistidas sem realizar rede durante o diagnóstico.
func DiagnoseIncidentWithDeployments(
	ctx context.Context,
	serviceName string,
	environment string,
	windows incidentdomain.InvestigationWindow,
	limit int,
	rankingConfig ranking.Config,
	signalReader SignalReader,
	deploymentReader DeploymentReader,
) (Diagnosis, error) {
	if strings.TrimSpace(environment) == "" {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: ambiente é obrigatório para correlacionar deployments")
	}
	if deploymentReader == nil {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: leitor de deployments é obrigatório")
	}
	return diagnoseIncident(ctx, serviceName, environment, windows, limit, rankingConfig, signalReader, deploymentReader)
}

func diagnoseIncident(
	ctx context.Context,
	serviceName string,
	environment string,
	windows incidentdomain.InvestigationWindow,
	limit int,
	rankingConfig ranking.Config,
	reader SignalReader,
	deploymentReader DeploymentReader,
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

	detectionInput := detection.Input{
		ServiceName: serviceName,
		Baseline:    baseline,
		Incident:    incident,
	}
	findings := detection.Run(detectionInput)
	if deploymentReader != nil {
		deployments, err := deploymentReader.ListDeployments(
			ctx,
			serviceName,
			environment,
			windows.Incident.Start.Add(-detection.DeploymentLookback),
			windows.Incident.Start.Add(time.Nanosecond),
			limit,
		)
		if err != nil {
			return Diagnosis{}, fmt.Errorf("diagnosticar incidente: carregar deployments: %w", err)
		}
		if finding, found := detection.DetectDeploymentProximity(detectionInput, deployments, windows.Incident.Start); found {
			findings = append(findings, finding)
		}
	}
	suspects, err := ranking.Rank(findings, rankingConfig)
	if err != nil {
		return Diagnosis{}, fmt.Errorf("diagnosticar incidente: ranquear suspeitos: %w", err)
	}
	return Diagnosis{
		ID:                  diagnosisID(serviceName, environment, windows),
		ServiceName:         serviceName,
		Environment:         strings.TrimSpace(environment),
		Windows:             windows,
		BaselineSignalCount: len(baseline),
		IncidentSignalCount: len(incident),
		Findings:            findings,
		Suspects:            suspects,
	}, nil
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
