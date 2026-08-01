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
// de ingestão e diagnóstico com volume suficiente para sustentar confiança alta.
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
		"Aumento da taxa de erros HTTP",
		"Aumento da latência HTTP",
		"Timeout no PostgreSQL",
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
