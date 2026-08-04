package main

import (
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestInitCommandGeraConfiguraçãoCompleta garante que init não omite seções do MVP.
func TestInitCommandGeraConfiguraçãoCompleta(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	command := newRootCommand()
	command.SetArgs([]string{"init", "--directory", projectDir})
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Execute(); err != nil {
		t.Fatalf("init command erro = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(projectDir, "faultmap.yaml"))
	if err != nil {
		t.Fatalf("ler configuração criada: %v", err)
	}
	for _, section := range []string{"ranking:", "github:", "privacy:"} {
		if !strings.Contains(string(content), section) {
			t.Fatalf("configuração gerada não contém a seção %q", section)
		}
	}
}

// TestIngestFileCommandPersisteFixtureOTLP verifica a primeira fatia completa da CLI de ingestão.
func TestIngestFileCommandPersisteFixtureOTLP(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	initialize := newRootCommand()
	initialize.SetArgs([]string{"init", "--directory", projectDir})
	initialize.SetOut(io.Discard)
	initialize.SetErr(io.Discard)
	if err := initialize.Execute(); err != nil {
		t.Fatalf("init command erro = %v", err)
	}

	fixturePath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "otel", "checkout-normal.json"))
	if err != nil {
		t.Fatalf("resolver caminho da fixture: %v", err)
	}
	var output bytes.Buffer
	ingest := newRootCommand()
	ingest.SetArgs([]string{
		"ingest", "file",
		"--input", fixturePath,
		"--config", filepath.Join(projectDir, "faultmap.yaml"),
	})
	ingest.SetOut(&output)
	ingest.SetErr(io.Discard)
	if err := ingest.Execute(); err != nil {
		t.Fatalf("ingest file erro = %v", err)
	}
	if got := output.String(); got != "Ingeridos 2 sinais; 2 novos.\n" {
		t.Fatalf("saída = %q, esperado resumo de ingestão", got)
	}

	database, err := sql.Open("sqlite", filepath.Join(projectDir, "faultmap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar banco: %v", closeErr)
		}
	})

	var signalCount int
	if err := database.QueryRow("SELECT COUNT(*) FROM signals").Scan(&signalCount); err != nil {
		t.Fatalf("contar sinais: %v", err)
	}
	if signalCount != 2 {
		t.Fatalf("quantidade de sinais = %d, esperado 2", signalCount)
	}
}

// TestTelemetryListCommandExibeSinaisIngeridos garante que a consulta da CLI apresenta
// evidências compreensíveis do serviço, incluindo sucesso, erro e operação de banco.
func TestTelemetryListCommandExibeSinaisIngeridos(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, "faultmap.yaml")
	initialize := newRootCommand()
	initialize.SetArgs([]string{"init", "--directory", projectDir})
	initialize.SetOut(io.Discard)
	initialize.SetErr(io.Discard)
	if err := initialize.Execute(); err != nil {
		t.Fatalf("init command erro = %v", err)
	}

	for _, fixtureName := range []string{"checkout-normal.json", "checkout-error-latency.json"} {
		fixturePath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "otel", fixtureName))
		if err != nil {
			t.Fatalf("resolver caminho da fixture %q: %v", fixtureName, err)
		}

		ingest := newRootCommand()
		ingest.SetArgs([]string{"ingest", "file", "--input", fixturePath, "--config", configPath})
		ingest.SetOut(io.Discard)
		ingest.SetErr(io.Discard)
		if err := ingest.Execute(); err != nil {
			t.Fatalf("ingerir fixture %q: %v", fixtureName, err)
		}
	}

	var output bytes.Buffer
	list := newRootCommand()
	list.SetArgs([]string{
		"telemetry", "list",
		"--config", configPath,
		"--service", "checkout-service",
		"--since", "8760h",
		"--limit", "10",
	})
	list.SetOut(&output)
	list.SetErr(io.Discard)
	if err := list.Execute(); err != nil {
		t.Fatalf("telemetry list erro = %v", err)
	}

	for _, expected := range []string{
		"checkout-service",
		"POST /checkout",
		"INSERT orders",
		"HTTP 201",
		"HTTP 500",
		"PostgreSQL",
		"timeout",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("saída não contém %q:\n%s", expected, output.String())
		}
	}
}

// TestDiagnoseIncidentCommandExplicaEvidências garante que o diagnóstico correlaciona
// o erro HTTP, a degradação de duração e o timeout PostgreSQL, sem omitir a limitação
// de confiança causada pela amostra mínima das fixtures.
func TestDiagnoseIncidentCommandExplicaEvidências(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, "faultmap.yaml")
	initialize := newRootCommand()
	initialize.SetArgs([]string{"init", "--directory", projectDir})
	initialize.SetOut(io.Discard)
	initialize.SetErr(io.Discard)
	if err := initialize.Execute(); err != nil {
		t.Fatalf("init command erro = %v", err)
	}

	for _, fixtureName := range []string{"checkout-normal.json", "checkout-error-latency.json"} {
		fixturePath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "otel", fixtureName))
		if err != nil {
			t.Fatalf("resolver caminho da fixture %q: %v", fixtureName, err)
		}

		ingest := newRootCommand()
		ingest.SetArgs([]string{"ingest", "file", "--input", fixturePath, "--config", configPath})
		ingest.SetOut(io.Discard)
		ingest.SetErr(io.Discard)
		if err := ingest.Execute(); err != nil {
			t.Fatalf("ingerir fixture %q: %v", fixtureName, err)
		}
	}

	var output bytes.Buffer
	diagnose := newRootCommand()
	diagnose.SetArgs([]string{
		"diagnose", "incident",
		"--config", configPath,
		"--service", "checkout-service",
		"--since", "1m",
		"--baseline", "1m",
		"--until", "2025-12-01T10:02:00Z",
		"--limit", "100",
	})
	diagnose.SetOut(&output)
	diagnose.SetErr(io.Discard)
	if err := diagnose.Execute(); err != nil {
		t.Fatalf("diagnose incident erro = %v", err)
	}

	for _, expected := range []string{
		"checkout-service",
		"HTTP 500",
		"duração",
		"PostgreSQL",
		"timeout",
		"Confiança: baixa",
		"Limitações gerais:",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("saída não contém %q:\n%s", expected, output.String())
		}
	}
}

// TestDiagnoseIncidentCommandExplicaAmostraRepresentativa cobre o fluxo completo
// de ingestão e diagnóstico com volume suficiente para sustentar confiança alta,
// ranking agregado e contribuições auditáveis sem ocultar as evidências originais.
func TestDiagnoseIncidentCommandExplicaAmostraRepresentativa(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, "faultmap.yaml")
	initialize := newRootCommand()
	initialize.SetArgs([]string{"init", "--directory", projectDir})
	initialize.SetOut(io.Discard)
	initialize.SetErr(io.Discard)
	if err := initialize.Execute(); err != nil {
		t.Fatalf("init command erro = %v", err)
	}

	for _, fixtureName := range []string{"checkout-baseline-sample.json", "checkout-incident-sample.json"} {
		fixturePath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "otel", fixtureName))
		if err != nil {
			t.Fatalf("resolver caminho da fixture %q: %v", fixtureName, err)
		}

		ingest := newRootCommand()
		ingest.SetArgs([]string{"ingest", "file", "--input", fixturePath, "--config", configPath})
		ingest.SetOut(io.Discard)
		ingest.SetErr(io.Discard)
		if err := ingest.Execute(); err != nil {
			t.Fatalf("ingerir fixture %q: %v", fixtureName, err)
		}
	}

	var output bytes.Buffer
	diagnose := newRootCommand()
	diagnose.SetArgs([]string{
		"diagnose", "incident",
		"--config", configPath,
		"--service", "checkout-service",
		"--since", "1m",
		"--baseline", "1m",
		"--until", "2025-12-01T10:02:00Z",
		"--limit", "100",
	})
	diagnose.SetOut(&output)
	diagnose.SetErr(io.Discard)
	if err := diagnose.Execute(); err != nil {
		t.Fatalf("diagnose incident erro = %v", err)
	}

	diagnosisOutput := output.String()
	for _, expected := range []string{
		"Ranking de suspeitos:",
		"1. checkout-service",
		"Score agregado: 0.40",
		"Contribuições:",
		"error_rate_delta: score 0.40 × peso 0.25 = 0.10",
		"latency_delta: score 0.94 × peso 0.10 = 0.09",
		"database_timeout: score 0.30 × peso 0.20 = 0.06",
		"database_http_trace_correlation: score 1.00 × peso 0.15 = 0.15",
		"Aumento da taxa de erros HTTP",
		"Aumento da latência HTTP",
		"Timeout no PostgreSQL",
		"Timeout PostgreSQL correlacionado a impacto HTTP",
		"ID da regra: database_http_trace_correlation",
		"6 de 6 traces com timeout PostgreSQL também apresentaram erro HTTP 5xx ou latência HTTP acima do p95 da baseline",
		"Confiança: alta",
		"taxa de erro aumentou de 0.00% para 40.00%",
		"duração p95 (latência) HTTP aumentou",
		"6 de 20 operações PostgreSQL tiveram timeout",
		"Limitações gerais:",
	} {
		if !strings.Contains(diagnosisOutput, expected) {
			t.Errorf("saída não contém %q:\n%s", expected, diagnosisOutput)
		}
	}

	const causalLimitation = "Correlação entre sinais não comprova causalidade."
	if occurrences := strings.Count(diagnosisOutput, causalLimitation); occurrences != 1 {
		t.Errorf("limitação geral aparece %d vezes, esperado 1:\n%s", occurrences, diagnosisOutput)
	}
}

// TestBlameTraceCommandExplicaFluxoHTTPPostgreSQL garante que um trace
// correlacionado possa ser investigado sem expor SQL bruto ou atributos livres.
func TestBlameTraceCommandExplicaFluxoHTTPPostgreSQL(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, "faultmap.yaml")
	initialize := newRootCommand()
	initialize.SetArgs([]string{"init", "--directory", projectDir})
	initialize.SetOut(io.Discard)
	initialize.SetErr(io.Discard)
	if err := initialize.Execute(); err != nil {
		t.Fatalf("init command erro = %v", err)
	}

	fixturePath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "otel", "checkout-incident-sample.json"))
	if err != nil {
		t.Fatalf("resolver caminho da fixture: %v", err)
	}
	ingest := newRootCommand()
	ingest.SetArgs([]string{"ingest", "file", "--input", fixturePath, "--config", configPath})
	ingest.SetOut(io.Discard)
	ingest.SetErr(io.Discard)
	if err := ingest.Execute(); err != nil {
		t.Fatalf("ingerir fixture: %v", err)
	}

	const traceID = "30000000000000000000000000000001"
	var output bytes.Buffer
	blame := newRootCommand()
	blame.SetArgs([]string{
		"blame", "trace",
		"--config", configPath,
		"--trace", traceID,
		"--limit", "20",
	})
	blame.SetOut(&output)
	blame.SetErr(io.Discard)
	if err := blame.Execute(); err != nil {
		t.Fatalf("blame trace erro = %v", err)
	}

	traceOutput := output.String()
	for _, expected := range []string{
		"Investigação do trace — " + traceID,
		"Serviço: checkout-service",
		"Grafo de evidências:",
		"POST /checkout",
		"HTTP 500",
		"duração 800 ms",
		"consulta",
		"INSERT orders",
		"PostgreSQL",
		"erro timeout",
		"duração 650 ms",
	} {
		if !strings.Contains(traceOutput, expected) {
			t.Errorf("saída não contém %q:\n%s", expected, traceOutput)
		}
	}
	if strings.Contains(strings.ToUpper(traceOutput), "INSERT INTO") {
		t.Errorf("saída expôs SQL bruto:\n%s", traceOutput)
	}
}

// TestExportGraphCommandGeraMermaidDoTrace cobre a exportação do mesmo grafo
// auditável sem criar uma segunda forma de consultar a telemetria.
func TestExportGraphCommandGeraMermaidDoTrace(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, "faultmap.yaml")
	initialize := newRootCommand()
	initialize.SetArgs([]string{"init", "--directory", projectDir})
	initialize.SetOut(io.Discard)
	initialize.SetErr(io.Discard)
	if err := initialize.Execute(); err != nil {
		t.Fatalf("init command erro = %v", err)
	}

	fixturePath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "otel", "checkout-incident-sample.json"))
	if err != nil {
		t.Fatalf("resolver fixture: %v", err)
	}
	ingest := newRootCommand()
	ingest.SetArgs([]string{"ingest", "file", "--input", fixturePath, "--config", configPath})
	ingest.SetOut(io.Discard)
	ingest.SetErr(io.Discard)
	if err := ingest.Execute(); err != nil {
		t.Fatalf("ingerir fixture: %v", err)
	}

	var output bytes.Buffer
	export := newRootCommand()
	export.SetArgs([]string{
		"export", "graph",
		"--config", configPath,
		"--trace", "30000000000000000000000000000001",
		"--format", "mermaid",
		"--limit", "20",
	})
	export.SetOut(&output)
	export.SetErr(io.Discard)
	if err := export.Execute(); err != nil {
		t.Fatalf("export graph erro = %v", err)
	}

	result := output.String()
	for _, expected := range []string{
		"flowchart TD",
		"checkout-service",
		"POST /checkout",
		"INSERT orders",
		"|contém|",
		"|consulta|",
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("Mermaid não contém %q:\n%s", expected, result)
		}
	}
	if strings.Contains(strings.ToUpper(result), "INSERT INTO") {
		t.Errorf("Mermaid expôs SQL bruto:\n%s", result)
	}
}

// TestInitCommandCreatesMigratedSQLiteWorkspace verifica que init produz um schema utilizável.
func TestInitCommandCreatesMigratedSQLiteWorkspace(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	command := newRootCommand()
	command.SetArgs([]string{"init", "--directory", projectDir})
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Execute(); err != nil {
		t.Fatalf("init command error = %v", err)
	}

	database, err := sql.Open("sqlite", filepath.Join(projectDir, "faultmap.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar banco: %v", closeErr)
		}
	})

	var tableCount int
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name IN ('schema_migrations', 'signals', 'incidents')
	`).Scan(&tableCount); err != nil {
		t.Fatalf("query migrated tables: %v", err)
	}
	if tableCount != 3 {
		t.Fatalf("migrated table count = %d, want 3", tableCount)
	}
}
