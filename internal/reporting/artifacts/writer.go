// Package artifacts grava em disco o conjunto de saídas do faultmap-out a
// partir de um snapshot já persistido, sem recalcular detectores nem consultar
// telemetria.
package artifacts

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/evidencegraph"
	jsonreport "github.com/faultmap/faultmap/internal/reporting/json"
	"github.com/faultmap/faultmap/internal/reporting/markdown"
	"github.com/faultmap/faultmap/internal/reporting/mermaid"
	"github.com/faultmap/faultmap/internal/reporting/timeline"
)

// Nomes dos arquivos exigidos pelo documento normativo do diretório de saída.
const (
	// ReportFileName é o relatório legível em Markdown.
	ReportFileName = "report.md"
	// RankingFileName é o ranking de suspeitos em JSON.
	RankingFileName = "ranking.json"
	// EvidenceGraphFileName é o grafo de evidências em sintaxe Mermaid.
	EvidenceGraphFileName = "evidence-graph.mmd"
	// IncidentSummaryFileName é o resumo enxuto do incidente em JSON.
	IncidentSummaryFileName = "incident-summary.json"
	// TimelineFileName é a linha do tempo do incidente em JSON.
	TimelineFileName = "timeline.json"
)

// filePermission restringe a leitura ao dono: artefatos de diagnóstico podem
// conter nomes de serviços, rotas e IDs de sinais internos.
const filePermission os.FileMode = 0o600

// Write grava os cinco artefatos do diretório de saída em directory, que
// precisa existir — a função nunca cria diretórios, para não espalhar arquivos
// fora do faultmap-out esperado. Artefatos anteriores são sobrescritos.
//
// graph é opcional: quando nulo, evidence-graph.mmd é gravado com um grafo
// vazio, mantendo o conjunto de arquivos completo e sintaticamente válido.
// generatedAt é injetado pelo chamador para que duas execuções sobre o mesmo
// snapshot produzam bytes idênticos.
func Write(
	directory string,
	diagnosis application.PersistedDiagnosis,
	graph *evidencegraph.Graph,
	generatedAt time.Time,
) error {
	if strings.TrimSpace(directory) == "" {
		return fmt.Errorf("gravar artefatos: diretório de saída é obrigatório")
	}
	info, err := os.Stat(directory)
	if err != nil {
		return fmt.Errorf("gravar artefatos: inspecionar diretório %q: %w", directory, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("gravar artefatos: %q não é um diretório", directory)
	}

	evidenceGraph := evidencegraph.Graph{}
	if graph != nil {
		evidenceGraph = *graph
	}

	// Cada artefato é renderizado antes de tocar o disco: assim uma falha de
	// serialização não deixa um arquivo truncado no diretório de saída.
	renderers := []struct {
		name   string
		render func(io.Writer) error
	}{
		{ReportFileName, func(writer io.Writer) error { return markdown.Render(writer, diagnosis) }},
		{RankingFileName, func(writer io.Writer) error {
			return jsonreport.RenderRanking(writer, diagnosis, generatedAt)
		}},
		{EvidenceGraphFileName, func(writer io.Writer) error {
			return mermaid.RenderTraceGraph(writer, evidenceGraph)
		}},
		{IncidentSummaryFileName, func(writer io.Writer) error {
			return jsonreport.RenderIncidentSummary(writer, diagnosis, generatedAt)
		}},
		{TimelineFileName, func(writer io.Writer) error { return timeline.Render(writer, diagnosis) }},
	}

	for _, artifact := range renderers {
		var content bytes.Buffer
		if err := artifact.render(&content); err != nil {
			return fmt.Errorf("gravar artefatos: renderizar %q: %w", artifact.name, err)
		}
		path := filepath.Join(directory, artifact.name)
		if err := os.WriteFile(path, content.Bytes(), filePermission); err != nil {
			return fmt.Errorf("gravar artefatos: escrever %q: %w", path, err)
		}
	}
	return nil
}
