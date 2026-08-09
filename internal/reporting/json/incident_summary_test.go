package jsonreport_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	jsonreport "github.com/faultmap/faultmap/internal/reporting/json"
)

type summaryDocument struct {
	SchemaVersion string `json:"schema_version"`
	IncidentID    string `json:"incident_id"`
	GeneratedAt   string `json:"generated_at"`
	ServiceName   string `json:"service_name"`
	Environment   string `json:"environment"`
	Status        string `json:"status"`
	Baseline      *struct {
		StartedAt   string `json:"started_at"`
		EndedAt     string `json:"ended_at"`
		SignalCount *int   `json:"signal_count"`
	} `json:"baseline"`
	IncidentWindow struct {
		StartedAt   string `json:"started_at"`
		EndedAt     string `json:"ended_at"`
		SignalCount *int   `json:"signal_count"`
	} `json:"incident_window"`
	FindingCount   int `json:"finding_count"`
	SuspectCount   int `json:"suspect_count"`
	PrimarySuspect *struct {
		ID         string  `json:"id"`
		Label      string  `json:"label"`
		Score      float64 `json:"score"`
		Confidence string  `json:"confidence"`
	} `json:"primary_suspect"`
}

func summarySnapshot() application.PersistedDiagnosis {
	baselineStart := time.Date(2024, time.May, 4, 9, 0, 0, 0, time.UTC)
	baselineEnd := time.Date(2024, time.May, 4, 9, 30, 0, 0, time.UTC)
	baselineCount := 120
	incidentCount := 40

	diagnosis := rankingSnapshot()
	diagnosis.Incident.IncidentStart = time.Date(2024, time.May, 4, 10, 0, 0, 0, time.UTC)
	diagnosis.Incident.IncidentEnd = time.Date(2024, time.May, 4, 10, 20, 0, 0, time.UTC)
	diagnosis.BaselineStart = &baselineStart
	diagnosis.BaselineEnd = &baselineEnd
	diagnosis.BaselineSignalCount = &baselineCount
	diagnosis.IncidentSignalCount = &incidentCount
	diagnosis.Findings = []detection.Finding{
		{Rule: "error_rate_spike", ServiceName: "checkout-service", Score: 0.56, Confidence: detection.ConfidenceHigh},
		{Rule: "recent_deploy", ServiceName: "checkout-service", Score: 0.35, Confidence: detection.ConfidenceHigh},
	}
	return diagnosis
}

func TestRenderIncidentSummaryResumeIdentificacaoJanelasEContagens(t *testing.T) {
	var output bytes.Buffer
	instante := time.Date(2024, time.May, 4, 11, 0, 0, 0, time.UTC)

	if err := jsonreport.RenderIncidentSummary(&output, summarySnapshot(), instante); err != nil {
		t.Fatalf("RenderIncidentSummary retornou erro: %v", err)
	}

	var document summaryDocument
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatalf("saída não é JSON válido: %v", err)
	}

	if document.SchemaVersion == "" || document.IncidentID != "checkout-service-prod-2024" {
		t.Errorf("identificação incompleta: %+v", document)
	}
	if document.GeneratedAt != "2024-05-04T11:00:00Z" {
		t.Errorf("generated_at = %q, esperado o instante injetado", document.GeneratedAt)
	}
	if document.ServiceName != "checkout-service" || document.Environment != "prod" || document.Status != "closed" {
		t.Errorf("campos do incidente inesperados: %+v", document)
	}
	if document.Baseline == nil || document.Baseline.StartedAt != "2024-05-04T09:00:00Z" ||
		document.Baseline.SignalCount == nil || *document.Baseline.SignalCount != 120 {
		t.Errorf("janela baseline inesperada: %+v", document.Baseline)
	}
	if document.IncidentWindow.StartedAt != "2024-05-04T10:00:00Z" ||
		document.IncidentWindow.EndedAt != "2024-05-04T10:20:00Z" ||
		document.IncidentWindow.SignalCount == nil || *document.IncidentWindow.SignalCount != 40 {
		t.Errorf("janela do incidente inesperada: %+v", document.IncidentWindow)
	}
	if document.FindingCount != 2 || document.SuspectCount != 2 {
		t.Errorf("contagens = %d findings / %d suspeitos", document.FindingCount, document.SuspectCount)
	}
	if document.PrimarySuspect == nil || document.PrimarySuspect.ID != "checkout-service" ||
		document.PrimarySuspect.Score != 0.91 || document.PrimarySuspect.Confidence != "alta" {
		t.Errorf("suspeito principal inesperado: %+v", document.PrimarySuspect)
	}
}

// O resumo é intencionalmente enxuto: não pode carregar evidências nem
// contribuições, que já vivem em report.md, ranking.json e no relatório completo.
func TestRenderIncidentSummaryNaoDuplicaRelatorioCompleto(t *testing.T) {
	var output bytes.Buffer
	if err := jsonreport.RenderIncidentSummary(&output, summarySnapshot(), time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("RenderIncidentSummary retornou erro: %v", err)
	}

	var document map[string]any
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatalf("saída não é JSON válido: %v", err)
	}
	for _, proibido := range []string{"findings", "ranking", "suspects", "evidence", "contributions"} {
		if _, presente := document[proibido]; presente {
			t.Errorf("resumo não deveria conter a chave %q", proibido)
		}
	}
}

func TestRenderIncidentSummarySemMetadadosCompletosOmiteBaseline(t *testing.T) {
	var output bytes.Buffer
	diagnosis := summarySnapshot()
	diagnosis.BaselineStart = nil
	diagnosis.BaselineEnd = nil
	diagnosis.BaselineSignalCount = nil

	if err := jsonreport.RenderIncidentSummary(&output, diagnosis, time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("RenderIncidentSummary retornou erro: %v", err)
	}
	var document summaryDocument
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatalf("saída não é JSON válido: %v", err)
	}
	if document.Baseline != nil {
		t.Errorf("baseline deveria ser nulo em snapshot legado, obtido %+v", document.Baseline)
	}
}

func TestRenderIncidentSummarySemSuspeitosOmiteSuspeitoPrincipal(t *testing.T) {
	var output bytes.Buffer
	diagnosis := summarySnapshot()
	diagnosis.Suspects = nil

	if err := jsonreport.RenderIncidentSummary(&output, diagnosis, time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("RenderIncidentSummary retornou erro: %v", err)
	}
	var document summaryDocument
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatalf("saída não é JSON válido: %v", err)
	}
	if document.PrimarySuspect != nil {
		t.Errorf("primary_suspect deveria ser nulo, obtido %+v", document.PrimarySuspect)
	}
	if document.SuspectCount != 0 {
		t.Errorf("suspect_count = %d, esperado 0", document.SuspectCount)
	}
}

func TestRenderIncidentSummaryProduzBytesIdenticosEmDuasExecucoes(t *testing.T) {
	instante := time.Date(2024, time.May, 4, 11, 0, 0, 0, time.UTC)
	var primeira, segunda bytes.Buffer

	if err := jsonreport.RenderIncidentSummary(&primeira, summarySnapshot(), instante); err != nil {
		t.Fatalf("primeira execução retornou erro: %v", err)
	}
	if err := jsonreport.RenderIncidentSummary(&segunda, summarySnapshot(), instante); err != nil {
		t.Fatalf("segunda execução retornou erro: %v", err)
	}
	if !bytes.Equal(primeira.Bytes(), segunda.Bytes()) {
		t.Error("mesmo snapshot deveria produzir bytes idênticos")
	}
}
