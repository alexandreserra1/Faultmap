package artifacts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	"github.com/faultmap/faultmap/internal/evidencegraph"
	"github.com/faultmap/faultmap/internal/ranking"
	"github.com/faultmap/faultmap/internal/reporting/artifacts"
)

func snapshot() application.PersistedDiagnosis {
	baselineStart := time.Date(2024, time.May, 4, 9, 0, 0, 0, time.UTC)
	baselineEnd := time.Date(2024, time.May, 4, 9, 30, 0, 0, time.UTC)
	baselineCount := 120
	incidentCount := 40

	return application.PersistedDiagnosis{
		Incident: application.IncidentSummary{
			ID:            "checkout-service-prod-2024",
			ServiceName:   "checkout-service",
			Environment:   "prod",
			Status:        "closed",
			IncidentStart: time.Date(2024, time.May, 4, 10, 0, 0, 0, time.UTC),
			IncidentEnd:   time.Date(2024, time.May, 4, 10, 20, 0, 0, time.UTC),
		},
		BaselineStart:       &baselineStart,
		BaselineEnd:         &baselineEnd,
		BaselineSignalCount: &baselineCount,
		IncidentSignalCount: &incidentCount,
		Findings: []detection.Finding{
			{
				Rule: "error_rate_spike", ServiceName: "checkout-service",
				Score: 0.56, Confidence: detection.ConfidenceHigh,
				Evidence: []detection.Evidence{{Summary: "erros subiram", SignalIDs: []string{"s-1"}}},
			},
		},
		Suspects: []ranking.Suspect{
			{
				ID: "checkout-service", Label: "checkout-service", Score: 0.91,
				Confidence:    detection.ConfidenceHigh,
				Contributions: []ranking.ScoreContribution{{RuleID: "error_rate_spike", Value: 0.56, Reason: "aumento de erros"}},
				Limitations:   []string{"proximidade não prova causalidade"},
			},
		},
	}
}

func graph() evidencegraph.Graph {
	return evidencegraph.Graph{
		Nodes: []evidencegraph.Node{
			{ID: "svc:checkout", Kind: evidencegraph.NodeKindService, Label: "checkout-service"},
			{ID: "trace:abc", Kind: evidencegraph.NodeKindTrace, Label: "abc"},
		},
		Edges: []evidencegraph.Edge{
			{From: "svc:checkout", To: "trace:abc", Relation: evidencegraph.RelationContains},
		},
	}
}

func TestWriteGravaOsCincoArtefatosNoDiretorio(t *testing.T) {
	dir := t.TempDir()
	graphValue := graph()

	err := artifacts.Write(dir, snapshot(), &graphValue, time.Date(2024, time.May, 4, 11, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Write retornou erro: %v", err)
	}

	esperados := []string{"report.md", "ranking.json", "evidence-graph.mmd", "incident-summary.json", "timeline.json"}
	for _, nome := range esperados {
		info, err := os.Stat(filepath.Join(dir, nome))
		if err != nil {
			t.Fatalf("artefato %q ausente: %v", nome, err)
		}
		if info.Size() == 0 {
			t.Errorf("artefato %q ficou vazio", nome)
		}
		// Diagnósticos podem conter dados sensíveis: apenas o dono pode ler.
		if info.Mode().Perm() != 0o600 {
			t.Errorf("artefato %q com permissão %v, esperado 0600", nome, info.Mode().Perm())
		}
	}

	conteudo, err := os.ReadFile(filepath.Join(dir, "evidence-graph.mmd"))
	if err != nil {
		t.Fatalf("ler grafo: %v", err)
	}
	if !strings.Contains(string(conteudo), "flowchart TD") {
		t.Errorf("grafo não parece Mermaid:\n%s", conteudo)
	}
}

func TestWriteProduzBytesIdenticosEmDuasExecucoes(t *testing.T) {
	primeiroDir := t.TempDir()
	segundoDir := t.TempDir()
	graphValue := graph()
	instante := time.Date(2024, time.May, 4, 11, 0, 0, 0, time.UTC)

	if err := artifacts.Write(primeiroDir, snapshot(), &graphValue, instante); err != nil {
		t.Fatalf("primeira execução retornou erro: %v", err)
	}
	if err := artifacts.Write(segundoDir, snapshot(), &graphValue, instante); err != nil {
		t.Fatalf("segunda execução retornou erro: %v", err)
	}

	for _, nome := range []string{"report.md", "ranking.json", "evidence-graph.mmd", "incident-summary.json", "timeline.json"} {
		primeiro, err := os.ReadFile(filepath.Join(primeiroDir, nome))
		if err != nil {
			t.Fatalf("ler %q: %v", nome, err)
		}
		segundo, err := os.ReadFile(filepath.Join(segundoDir, nome))
		if err != nil {
			t.Fatalf("ler %q: %v", nome, err)
		}
		if string(primeiro) != string(segundo) {
			t.Errorf("artefato %q não é determinístico", nome)
		}
	}
}

func TestWriteSemGrafoAindaGravaArquivoMermaidVazio(t *testing.T) {
	dir := t.TempDir()

	if err := artifacts.Write(dir, snapshot(), nil, time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("Write retornou erro: %v", err)
	}
	conteudo, err := os.ReadFile(filepath.Join(dir, "evidence-graph.mmd"))
	if err != nil {
		t.Fatalf("ler grafo: %v", err)
	}
	if !strings.Contains(string(conteudo), "flowchart TD") {
		t.Errorf("mesmo sem grafo o arquivo deveria ser Mermaid válido:\n%s", conteudo)
	}
	if strings.Contains(string(conteudo), "-->") {
		t.Errorf("grafo ausente não deveria conter arestas:\n%s", conteudo)
	}
}

func TestWriteFalhaQuandoDiretorioNaoExiste(t *testing.T) {
	ausente := filepath.Join(t.TempDir(), "faultmap-out")

	err := artifacts.Write(ausente, snapshot(), nil, time.Unix(0, 0).UTC())
	if err == nil {
		t.Fatal("esperado erro para diretório inexistente")
	}
	if _, statErr := os.Stat(ausente); statErr == nil {
		t.Error("o diretório não deveria ter sido criado silenciosamente")
	}
}

func TestWriteFalhaQuandoCaminhoNaoEDiretorio(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "faultmap-out")
	if err := os.WriteFile(arquivo, []byte("x"), 0o600); err != nil {
		t.Fatalf("preparar arquivo: %v", err)
	}

	if err := artifacts.Write(arquivo, snapshot(), nil, time.Unix(0, 0).UTC()); err == nil {
		t.Fatal("esperado erro quando o destino não é um diretório")
	}
}

func TestWriteExigeDiretorioInformado(t *testing.T) {
	if err := artifacts.Write("  ", snapshot(), nil, time.Unix(0, 0).UTC()); err == nil {
		t.Fatal("esperado erro para diretório vazio")
	}
}

func TestWriteSobrescreveArtefatosAnteriores(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "report.md")
	if err := os.WriteFile(caminho, []byte("conteúdo antigo muito mais longo do que o novo"), 0o600); err != nil {
		t.Fatalf("preparar artefato antigo: %v", err)
	}

	if err := artifacts.Write(dir, snapshot(), nil, time.Unix(0, 0).UTC()); err != nil {
		t.Fatalf("Write retornou erro: %v", err)
	}
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatalf("ler relatório: %v", err)
	}
	if strings.Contains(string(conteudo), "conteúdo antigo") {
		t.Error("o artefato anterior deveria ter sido truncado")
	}
}

// TestArtefatosConcordamSobreAOrdemDosSuspeitos amarra os renderizadores entre
// si. Cada um mantinha sua própria regra de ordenação, e a divergência só
// aparece no empate: quando um serviço falha e outro falha junto por
// consequência, os dois chegam ao mesmo score e cada arquivo podia eleger um
// primeiro suspeito diferente.
//
// É a mesma armadilha que já custou caro neste projeto — dois pedaços do código
// respondendo diferente sobre o mesmo dado, sem ninguém perceber. A ordem
// gravada no snapshot é a única fonte: nenhum artefato pode recalculá-la.
func TestArtefatosConcordamSobreAOrdemDosSuspeitos(t *testing.T) {
	t.Parallel()

	empatado := snapshot()
	empatado.Suspects = []ranking.Suspect{
		// Ordem gravada pelo motor: a origem antes da vítima. O desempate
		// alfabético colocaria "checkout-service" na frente.
		{ID: "payment-service", Label: "payment-service", Score: 0.25, Confidence: detection.ConfidenceHigh},
		{ID: "checkout-service", Label: "checkout-service", Score: 0.25, Confidence: detection.ConfidenceHigh},
	}

	directory := t.TempDir()
	if err := artifacts.Write(directory, empatado, nil, time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Write() erro = %v", err)
	}

	relatorio, err := os.ReadFile(filepath.Join(directory, artifacts.ReportFileName))
	if err != nil {
		t.Fatalf("ler report.md: %v", err)
	}
	if !strings.Contains(string(relatorio), "### 1. payment-service") {
		t.Fatalf("report.md elegeu outro primeiro suspeito:\n%s", relatorio)
	}

	rankingBytes, err := os.ReadFile(filepath.Join(directory, artifacts.RankingFileName))
	if err != nil {
		t.Fatalf("ler ranking.json: %v", err)
	}
	var rankingDocument struct {
		Suspects []struct {
			ID string `json:"id"`
		} `json:"suspects"`
	}
	if err := json.Unmarshal(rankingBytes, &rankingDocument); err != nil {
		t.Fatalf("decodificar ranking.json: %v\n%s", err, rankingBytes)
	}
	if len(rankingDocument.Suspects) == 0 || rankingDocument.Suspects[0].ID != "payment-service" {
		t.Fatalf("ranking.json elegeu outro primeiro suspeito:\n%s", rankingBytes)
	}

	resumoBytes, err := os.ReadFile(filepath.Join(directory, artifacts.IncidentSummaryFileName))
	if err != nil {
		t.Fatalf("ler incident-summary.json: %v", err)
	}
	var resumoDocument struct {
		PrimarySuspect *struct {
			ID string `json:"id"`
		} `json:"primary_suspect"`
	}
	if err := json.Unmarshal(resumoBytes, &resumoDocument); err != nil {
		t.Fatalf("decodificar incident-summary.json: %v\n%s", err, resumoBytes)
	}
	if resumoDocument.PrimarySuspect == nil || resumoDocument.PrimarySuspect.ID != "payment-service" {
		t.Fatalf("incident-summary.json elegeu outro suspeito principal:\n%s", resumoBytes)
	}
}
