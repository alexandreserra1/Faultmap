package application

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/detection"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
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

	diagnosis, err := DiagnoseIncident(context.Background(), "checkout-service", windows, 100, reader)
	if err != nil {
		t.Fatalf("DiagnoseIncident() erro = %v", err)
	}
	if diagnosis.BaselineSignalCount != 2 || diagnosis.IncidentSignalCount != 2 {
		t.Fatalf("contagens = baseline %d, incidente %d; esperado 2 e 2", diagnosis.BaselineSignalCount, diagnosis.IncidentSignalCount)
	}
	for _, rule := range []string{detection.RuleErrorRateDelta, detection.RuleLatencyDelta, detection.RuleDatabaseTimeout} {
		if !hasFinding(diagnosis.Findings, rule) {
			t.Errorf("diagnóstico não contém finding %q: %#v", rule, diagnosis.Findings)
		}
	}
	if len(reader.windows) != 2 {
		t.Fatalf("consultas ao repositório = %d, esperado 2", len(reader.windows))
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

	_, err = DiagnoseIncident(context.Background(), "checkout-service", windows, 0, reader)
	if err == nil {
		t.Fatal("DiagnoseIncident() erro = nil, esperado limite inválido")
	}
	if len(reader.windows) != 0 {
		t.Fatalf("consultas ao repositório = %d, esperado 0", len(reader.windows))
	}
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
