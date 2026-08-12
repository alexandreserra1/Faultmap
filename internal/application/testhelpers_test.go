package application

// Este arquivo reúne apenas auxiliares compartilhados pelos testes do pacote.
// Ele nasceu como o teste do diagnóstico de serviço único; quando esse caminho
// foi removido por não ter mais uso em produção, os auxiliares permaneceram.

import (
	"context"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
	"strconv"
	"time"

	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

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
