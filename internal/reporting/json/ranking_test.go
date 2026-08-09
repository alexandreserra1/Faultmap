package jsonreport_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/ranking"
	jsonreport "github.com/faultmap/faultmap/internal/reporting/json"
)

func rankingSnapshot() application.PersistedDiagnosis {
	return application.PersistedDiagnosis{
		Incident: application.IncidentSummary{
			ID:          "checkout-service-prod-2024",
			ServiceName: "checkout-service",
			Environment: "prod",
			Status:      "closed",
		},
		// Os suspeitos vêm do snapshot já ranqueados pelo motor. A exportação
		// preserva essa ordem em vez de recalculá-la, para não desfazer o
		// desempate aplicado quando dois serviços empatam em score.
		Suspects: []ranking.Suspect{
			{
				ID:         "checkout-service",
				Label:      "checkout-service",
				Score:      0.91,
				Confidence: detection.ConfidenceHigh,
				Contributions: []ranking.ScoreContribution{
					{RuleID: "recent_deploy", Value: 0.35, Reason: "deploy seis minutos antes"},
					{RuleID: "error_rate_spike", Value: 0.56, Reason: "aumento de erros"},
				},
				Limitations: []string{"proximidade não prova causalidade"},
			},
			{
				ID:         "payments-service",
				Label:      "payments-service",
				Score:      0.42,
				Confidence: detection.ConfidenceLow,
				Contributions: []ranking.ScoreContribution{
					{RuleID: "latency_shift", Value: 0.42, Reason: "p95 dobrou"},
				},
				Limitations: []string{"amostra pequena"},
			},
		},
	}
}

func TestRenderRankingPreservaOrdemDoSnapshotEOrdenaContribuicoesPorRegra(t *testing.T) {
	var output bytes.Buffer
	instante := time.Date(2024, time.May, 4, 10, 30, 0, 0, time.UTC)

	if err := jsonreport.RenderRanking(&output, rankingSnapshot(), instante); err != nil {
		t.Fatalf("RenderRanking retornou erro: %v", err)
	}

	var document struct {
		SchemaVersion string `json:"schema_version"`
		IncidentID    string `json:"incident_id"`
		GeneratedAt   string `json:"generated_at"`
		Suspects      []struct {
			ID            string   `json:"id"`
			Label         string   `json:"label"`
			Score         float64  `json:"score"`
			Confidence    string   `json:"confidence"`
			Limitations   []string `json:"limitations"`
			Contributions []struct {
				RuleID string  `json:"rule_id"`
				Value  float64 `json:"value"`
				Reason string  `json:"reason"`
			} `json:"contributions"`
		} `json:"suspects"`
	}
	if err := json.Unmarshal(output.Bytes(), &document); err != nil {
		t.Fatalf("saída não é JSON válido: %v", err)
	}

	if document.SchemaVersion == "" {
		t.Error("schema_version deveria estar presente")
	}
	if document.IncidentID != "checkout-service-prod-2024" {
		t.Errorf("incident_id = %q", document.IncidentID)
	}
	if document.GeneratedAt != "2024-05-04T10:30:00Z" {
		t.Errorf("generated_at = %q, esperado o instante injetado", document.GeneratedAt)
	}
	if len(document.Suspects) != 2 {
		t.Fatalf("esperados 2 suspeitos, obtidos %d", len(document.Suspects))
	}
	if document.Suspects[0].ID != "checkout-service" {
		t.Errorf("a ordem do snapshot deveria ser preservada, veio %q", document.Suspects[0].ID)
	}
	if document.Suspects[0].Score != 0.91 || document.Suspects[0].Confidence != "alta" {
		t.Errorf("suspeito principal inesperado: %+v", document.Suspects[0])
	}
	if len(document.Suspects[0].Contributions) != 2 ||
		document.Suspects[0].Contributions[0].RuleID != "error_rate_spike" {
		t.Errorf("contribuições deveriam estar ordenadas por rule_id: %+v", document.Suspects[0].Contributions)
	}
	if document.Suspects[0].Contributions[0].Value != 0.56 ||
		document.Suspects[0].Contributions[0].Reason != "aumento de erros" {
		t.Errorf("contribuição perdeu valor ou motivo: %+v", document.Suspects[0].Contributions[0])
	}
	if len(document.Suspects[0].Limitations) != 1 {
		t.Errorf("limitações deveriam ser preservadas: %+v", document.Suspects[0].Limitations)
	}
}

func TestRenderRankingProduzBytesIdenticosEmDuasExecucoes(t *testing.T) {
	instante := time.Date(2024, time.May, 4, 10, 30, 0, 0, time.UTC)
	var primeira, segunda bytes.Buffer

	if err := jsonreport.RenderRanking(&primeira, rankingSnapshot(), instante); err != nil {
		t.Fatalf("primeira execução retornou erro: %v", err)
	}
	if err := jsonreport.RenderRanking(&segunda, rankingSnapshot(), instante); err != nil {
		t.Fatalf("segunda execução retornou erro: %v", err)
	}
	if !bytes.Equal(primeira.Bytes(), segunda.Bytes()) {
		t.Error("mesmo snapshot deveria produzir bytes idênticos")
	}
}

func TestRenderRankingSemSuspeitosEscreveListaVaziaENaoNula(t *testing.T) {
	var output bytes.Buffer
	diagnosis := rankingSnapshot()
	diagnosis.Suspects = nil

	if err := jsonreport.RenderRanking(&output, diagnosis, time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("RenderRanking retornou erro: %v", err)
	}
	if !strings.Contains(output.String(), `"suspects": []`) {
		t.Errorf("esperada lista vazia explícita, obtido:\n%s", output.String())
	}
}

func TestRenderRankingNormalizaInstanteParaUTC(t *testing.T) {
	var output bytes.Buffer
	zona := time.FixedZone("BRT", -3*60*60)

	err := jsonreport.RenderRanking(&output, rankingSnapshot(), time.Date(2024, time.May, 4, 7, 30, 0, 0, zona))
	if err != nil {
		t.Fatalf("RenderRanking retornou erro: %v", err)
	}
	if !strings.Contains(output.String(), `"generated_at": "2024-05-04T10:30:00Z"`) {
		t.Errorf("generated_at deveria ser normalizado para UTC, obtido:\n%s", output.String())
	}
}

// TestRankingPreservaAOrdemDoSnapshot cobre uma divergência encontrada na
// revisão: o terminal apresenta os suspeitos na ordem gravada, mas os
// renderizadores JSON os reordenavam por score e ID. Em um empate — o caso comum
// de um serviço falhar e outro falhar junto por consequência — isso desfazia o
// desempate por profundidade na cadeia e colocava a vítima em primeiro lugar,
// contradizendo a saída do próprio comando.
//
// A ordem persistida É o ranking: ela não deve ser recalculada na exportação.
func TestRankingPreservaAOrdemDoSnapshot(t *testing.T) {
	t.Parallel()

	empatados := application.PersistedDiagnosis{
		Incident: application.IncidentSummary{ID: "inc_empate", ServiceName: "checkout-service"},
		Suspects: []ranking.Suspect{
			// Gravados nesta ordem pelo desempate: a origem antes da vítima.
			{ID: "payment-service", Label: "payment-service", Score: 0.25, Confidence: detection.ConfidenceHigh},
			{ID: "checkout-service", Label: "checkout-service", Score: 0.25, Confidence: detection.ConfidenceHigh},
		},
	}

	var buffer bytes.Buffer
	if err := jsonreport.RenderRanking(&buffer, empatados, time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RenderRanking() erro = %v", err)
	}

	var documento struct {
		Suspects []struct {
			ID string `json:"id"`
		} `json:"suspects"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &documento); err != nil {
		t.Fatalf("decodificar ranking: %v\n%s", err, buffer.String())
	}
	if len(documento.Suspects) != 2 {
		t.Fatalf("suspeitos = %d, esperado 2", len(documento.Suspects))
	}
	if documento.Suspects[0].ID != "payment-service" {
		t.Fatalf("primeiro suspeito = %q, esperado payment-service: a ordem do snapshot foi refeita",
			documento.Suspects[0].ID)
	}
}
