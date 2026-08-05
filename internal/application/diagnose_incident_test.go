package application

import (
	"context"
	"strconv"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	"github.com/faultmap/faultmap/internal/detection"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
	"github.com/faultmap/faultmap/internal/ranking"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestDiagnoseIncidentComparaJanelasEDetectaEvidências verifica a composição determinística do diagnóstico.
func TestDiagnoseIncidentComparaJanelasEDetectaEvidências(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(incidentStart, incidentStart.Add(time.Minute), time.Minute)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}
	reader := &diagnosisReaderFake{
		baseline: []domain.Signal{
			diagnosisHTTPSignal("baseline-http", incidentStart.Add(-time.Minute), 201, 120),
			diagnosisDatabaseSignal("baseline-db", incidentStart.Add(-time.Minute), false, 50),
		},
		incident: []domain.Signal{
			diagnosisHTTPSignal("incident-http", incidentStart, 500, 2300),
			diagnosisDatabaseSignal("incident-db", incidentStart, true, 2100),
		},
	}

	diagnosis, err := DiagnoseIncident(context.Background(), "checkout-service", windows, 100, testRankingConfig(), reader)
	if err != nil {
		t.Fatalf("DiagnoseIncident() erro = %v", err)
	}
	if diagnosis.BaselineSignalCount != 2 || diagnosis.IncidentSignalCount != 2 {
		t.Fatalf("contagens = baseline %d, incidente %d; esperado 2 e 2", diagnosis.BaselineSignalCount, diagnosis.IncidentSignalCount)
	}
	if diagnosis.ID != DiagnosisID("checkout-service", windows) {
		t.Fatalf("diagnosis.ID = %q, esperado ID determinístico", diagnosis.ID)
	}
	for _, rule := range []string{detection.RuleErrorRateDelta, detection.RuleLatencyDelta, detection.RuleDatabaseTimeout} {
		if !hasFinding(diagnosis.Findings, rule) {
			t.Errorf("diagnóstico não contém finding %q: %#v", rule, diagnosis.Findings)
		}
	}
	if len(reader.windows) != 2 {
		t.Fatalf("consultas ao repositório = %d, esperado 2", len(reader.windows))
	}
	if len(diagnosis.Suspects) != 1 || diagnosis.Suspects[0].ID != "checkout-service" {
		t.Fatalf("ranking = %#v, esperado checkout-service como suspeito", diagnosis.Suspects)
	}
	if len(diagnosis.Suspects[0].Contributions) != 3 {
		t.Fatalf("contribuições = %#v, esperado uma por finding conhecido", diagnosis.Suspects[0].Contributions)
	}
}

// TestDiagnoseIncidentWithDeploymentsIncluiMudancaNoRanking garante que a
// consulta externa já persistida participe da mesma análise determinística.
func TestDiagnoseIncidentWithDeploymentsIncluiMudancaNoRanking(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(incidentStart, incidentStart.Add(time.Minute), time.Minute)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}
	baseline := diagnosisHTTPSignal("baseline-http", incidentStart.Add(-time.Minute), 201, 120)
	baseline.Attributes["service.version"] = "1.0.0"
	incident := diagnosisHTTPSignal("incident-http", incidentStart, 500, 2300)
	incident.Attributes["service.version"] = "1.0.1"
	signalReader := &diagnosisReaderFake{baseline: []domain.Signal{baseline}, incident: []domain.Signal{incident}}
	deploymentReader := &deploymentReaderFake{deployments: []changedomain.Deployment{{
		ID: "deployment-42", Repository: "acme/checkout", Environment: "staging",
		ServiceName: "checkout-service", CommitSHA: "1.0.1", DeployedAt: incidentStart.Add(-6 * time.Minute),
	}}}

	diagnosis, err := DiagnoseIncidentWithDeployments(
		context.Background(), "checkout-service", "staging", windows, 100,
		testRankingConfig(), signalReader, deploymentReader,
	)
	if err != nil {
		t.Fatalf("DiagnoseIncidentWithDeployments() erro = %v", err)
	}
	if diagnosis.Environment != "staging" {
		t.Fatalf("Environment = %q", diagnosis.Environment)
	}
	if !hasFinding(diagnosis.Findings, detection.RuleDeploymentProximity) {
		t.Fatalf("findings = %#v, esperado deployment_proximity", diagnosis.Findings)
	}
	if len(deploymentReader.calls) != 1 || deploymentReader.calls[0].serviceName != "checkout-service" || deploymentReader.calls[0].environment != "staging" {
		t.Fatalf("consultas de deployment = %#v", deploymentReader.calls)
	}
	if diagnosis.ID == DiagnosisID("checkout-service", windows) {
		t.Fatal("ID com ambiente colidiu com diagnóstico sem ambiente")
	}
	if len(diagnosis.Suspects) != 1 || !hasContribution(diagnosis.Suspects[0], detection.RuleDeploymentProximity) {
		t.Fatalf("ranking = %#v", diagnosis.Suspects)
	}
}

// TestDiagnoseIncidentRejeitaLimiteInválido impede consultas ilimitadas ao repositório.
func TestDiagnoseIncidentRejeitaLimiteInválido(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(start, start.Add(time.Minute), time.Minute)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}
	reader := &diagnosisReaderFake{}

	_, err = DiagnoseIncident(context.Background(), "checkout-service", windows, 0, testRankingConfig(), reader)
	if err == nil {
		t.Fatal("DiagnoseIncident() erro = nil, esperado limite inválido")
	}
	if len(reader.windows) != 0 {
		t.Fatalf("consultas ao repositório = %d, esperado 0", len(reader.windows))
	}
}

// TestDiagnoseIncidentRejeitaRankingInválidoAntesDasConsultas garante que uma
// falha de configuração não abre trabalho desnecessário no banco.
func TestDiagnoseIncidentRejeitaRankingInválidoAntesDasConsultas(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(start, start.Add(time.Minute), time.Minute)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}
	reader := &diagnosisReaderFake{}

	_, err = DiagnoseIncident(context.Background(), "checkout-service", windows, 100, ranking.Config{}, reader)
	if err == nil {
		t.Fatal("DiagnoseIncident() erro = nil, esperado ranking inválido")
	}
	if len(reader.windows) != 0 {
		t.Fatalf("consultas ao repositório = %d, esperado 0", len(reader.windows))
	}
}

func testRankingConfig() ranking.Config {
	return ranking.Config{
		Weights: ranking.Weights{
			ErrorRateDelta:      0.25,
			DeploymentProximity: 0.20,
			DatabaseEvidence:    0.20,
			GraphProximity:      0.15,
			LatencyDelta:        0.10,
		},
		TopN: 3,
	}
}

type deploymentQuery struct {
	serviceName string
	environment string
	start       time.Time
	end         time.Time
	limit       int
}

type deploymentReaderFake struct {
	deployments []changedomain.Deployment
	calls       []deploymentQuery
}

func (reader *deploymentReaderFake) ListDeployments(
	_ context.Context,
	serviceName string,
	environment string,
	start time.Time,
	end time.Time,
	limit int,
) ([]changedomain.Deployment, error) {
	reader.calls = append(reader.calls, deploymentQuery{
		serviceName: serviceName, environment: environment, start: start, end: end, limit: limit,
	})
	return reader.deployments, nil
}

// diagnosisReaderFake separa as respostas pelas janelas esperadas sem usar infraestrutura.
type diagnosisReaderFake struct {
	baseline []domain.Signal
	incident []domain.Signal
	windows  []incidentdomain.TimeWindow
}

// ListByServiceAndWindow implementa a leitura limitada usada pelo diagnóstico.
func (reader *diagnosisReaderFake) ListByServiceAndWindow(
	_ context.Context,
	_ string,
	start time.Time,
	end time.Time,
	_ int,
) ([]domain.Signal, error) {
	window := incidentdomain.TimeWindow{Start: start, End: end}
	reader.windows = append(reader.windows, window)
	if len(reader.windows) == 1 {
		return reader.baseline, nil
	}
	return reader.incident, nil
}

func diagnosisHTTPSignal(id string, timestamp time.Time, statusCode int, durationMS float64) domain.Signal {
	return domain.Signal{
		ID:          id,
		ServiceName: "checkout-service",
		Timestamp:   timestamp,
		Attributes: map[string]string{
			"http.response.status_code": strconv.Itoa(statusCode),
		},
		Measurements: map[string]float64{"duration_ms": durationMS},
	}
}

func diagnosisDatabaseSignal(id string, timestamp time.Time, failed bool, durationMS float64) domain.Signal {
	attributes := map[string]string{"db.system.name": "postgresql", "db.operation.name": "INSERT"}
	severity := "info"
	if failed {
		attributes["error.type"] = "timeout"
		severity = "error"
	}
	return domain.Signal{
		ID: id, ServiceName: "checkout-service", Timestamp: timestamp, Severity: severity,
		Attributes: attributes, Measurements: map[string]float64{"duration_ms": durationMS},
	}
}

func hasFinding(findings []detection.Finding, rule string) bool {
	for _, finding := range findings {
		if finding.Rule == rule {
			return true
		}
	}
	return false
}

func hasContribution(suspect ranking.Suspect, rule string) bool {
	for _, contribution := range suspect.Contributions {
		if contribution.RuleID == rule {
			return true
		}
	}
	return false
}
