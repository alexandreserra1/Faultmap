package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

// TestIngestGitHubCommandPersisteCommitsEDeploymentsIdempotentemente cobre a
// fronteira HTTP mockada e o banco real sem depender da disponibilidade externa.
func TestIngestGitHubCommandPersisteCommitsEDeploymentsIdempotentemente(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/repos/acme/checkout/commits":
			if _, err := writer.Write([]byte(`[{"sha":"1.0.1","commit":{"message":"Reduce pool","committer":{"date":"2025-12-01T09:54:00Z"}},"author":{"login":"alex"}}]`)); err != nil {
				t.Errorf("responder commits: %v", err)
			}
		case "/repos/acme/checkout/deployments":
			if _, err := writer.Write([]byte(`[{"id":42,"sha":"1.0.1","ref":"main","task":"deploy","environment":"staging","created_at":"2025-12-01T09:55:00Z"}]`)); err != nil {
				t.Errorf("responder deployments: %v", err)
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	projectDir := t.TempDir()
	initialize := newRootCommand()
	initialize.SetArgs([]string{"init", "--directory", projectDir})
	initialize.SetOut(io.Discard)
	initialize.SetErr(io.Discard)
	if err := initialize.Execute(); err != nil {
		t.Fatalf("init erro = %v", err)
	}
	configPath := filepath.Join(projectDir, "faultmap.yaml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ler configuração: %v", err)
	}
	content = bytes.Replace(content, []byte("api_url: https://api.github.com"), []byte("api_url: "+server.URL), 1)
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("atualizar API GitHub do teste: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "test-token")

	execute := func() string {
		t.Helper()
		var output bytes.Buffer
		command := newRootCommand()
		command.SetArgs([]string{
			"ingest", "github", "--config", configPath, "--repo", "acme/checkout",
			"--commits", "--deployments", "--service", "checkout-service",
			"--environment", "staging", "--since", "2h", "--until", "2025-12-01T11:00:00Z", "--limit", "20",
		})
		command.SetOut(&output)
		command.SetErr(io.Discard)
		if err := command.Execute(); err != nil {
			t.Fatalf("ingest github erro = %v", err)
		}
		return output.String()
	}
	if output := execute(); !strings.Contains(output, "1 commits coletados; 1 novos") || !strings.Contains(output, "1 deployments coletados; 1 novos") {
		t.Fatalf("primeira saída inesperada:\n%s", output)
	}
	if output := execute(); !strings.Contains(output, "1 commits coletados; 0 novos") || !strings.Contains(output, "1 deployments coletados; 0 novos") {
		t.Fatalf("saída do retry inesperada:\n%s", output)
	}
	for _, fixtureName := range []string{"checkout-baseline-sample.json", "checkout-incident-sample.json"} {
		fixturePath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "otel", fixtureName))
		if err != nil {
			t.Fatalf("resolver fixture: %v", err)
		}
		ingest := newRootCommand()
		ingest.SetArgs([]string{"ingest", "file", "--config", configPath, "--input", fixturePath})
		ingest.SetOut(io.Discard)
		ingest.SetErr(io.Discard)
		if err := ingest.Execute(); err != nil {
			t.Fatalf("ingerir %s: %v", fixtureName, err)
		}
	}
	var diagnosisOutput bytes.Buffer
	diagnose := newRootCommand()
	diagnose.SetArgs([]string{
		"diagnose", "incident", "--config", configPath, "--service", "checkout-service",
		"--environment", "staging", "--since", "1m", "--baseline", "1m",
		"--until", "2025-12-01T10:02:00Z", "--limit", "100",
	})
	diagnose.SetOut(&diagnosisOutput)
	diagnose.SetErr(io.Discard)
	if err := diagnose.Execute(); err != nil {
		t.Fatalf("diagnosticar com deployment: %v", err)
	}
	for _, expected := range []string{
		"deployment_proximity",
		"6 minuto(s) antes do incidente",
		"service.version observada no incidente",
		"Score agregado: 0.58",
	} {
		if !strings.Contains(diagnosisOutput.String(), expected) {
			t.Errorf("diagnóstico não contém %q:\n%s", expected, diagnosisOutput.String())
		}
	}

	database, err := sql.Open("sqlite", filepath.Join(projectDir, "faultmap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar banco: %v", closeErr)
		}
	}()
	for table, expected := range map[string]int{"commits": 1, "deployments": 1} {
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatalf("contar %s: %v", table, err)
		}
		if count != expected {
			t.Fatalf("%s = %d, esperado %d", table, count, expected)
		}
	}
	var incidentID string
	if err := database.QueryRow(`SELECT id FROM incidents WHERE environment = ? LIMIT 1`, "staging").Scan(&incidentID); err != nil {
		t.Fatalf("ler incidente por ambiente: %v", err)
	}
	var persistedOutput bytes.Buffer
	show := newRootCommand()
	show.SetArgs([]string{"incident", "show", "--config", configPath, "--id", incidentID})
	show.SetOut(&persistedOutput)
	show.SetErr(io.Discard)
	if err := show.Execute(); err != nil {
		t.Fatalf("consultar incidente com deployment: %v", err)
	}
	if !strings.Contains(persistedOutput.String(), "Ambiente: staging") || !strings.Contains(persistedOutput.String(), "deployment_proximity") {
		t.Fatalf("snapshot não preservou ambiente e finding:\n%s", persistedOutput.String())
	}
}

// TestIngestGitHubCommandValidaFlagsAntesDoBanco evita I/O para seleção ou limite inválidos.
func TestIngestGitHubCommandValidaFlagsAntesDoBanco(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"ingest", "github", "--config", "inexistente.yaml", "--repo", "acme/checkout"}, want: "--commits"},
		{args: []string{"ingest", "github", "--config", "inexistente.yaml", "--repo", "acme/checkout", "--commits", "--limit", "0"}, want: "--limit"},
	} {
		command := newRootCommand()
		command.SetArgs(test.args)
		command.SetOut(io.Discard)
		command.SetErr(io.Discard)
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("ingest github erro = %v, esperado %q", err, test.want)
		}
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
	if !strings.Contains(diagnosisOutput, "Diagnóstico salvo: inc_") {
		t.Errorf("saída não confirma persistência do diagnóstico:\n%s", diagnosisOutput)
	}

	database, err := sql.Open("sqlite", filepath.Join(projectDir, "faultmap.db"))
	if err != nil {
		t.Fatalf("abrir banco para verificar diagnóstico: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar banco de verificação: %v", closeErr)
		}
	})
	var incidentID string
	if err := database.QueryRow(`SELECT id FROM incidents`).Scan(&incidentID); err != nil {
		t.Fatalf("ler incidente persistido: %v", err)
	}
	assertTableCount(t, database, "incidents", 1)
	assertTableCount(t, database, "findings", 4)
	assertTableCount(t, database, "ranking_results", 1)

	var retryOutput bytes.Buffer
	retry := newRootCommand()
	retry.SetArgs([]string{
		"diagnose", "incident",
		"--config", configPath,
		"--service", "checkout-service",
		"--since", "1m",
		"--baseline", "1m",
		"--until", "2025-12-01T10:02:00Z",
		"--limit", "100",
	})
	retry.SetOut(&retryOutput)
	retry.SetErr(io.Discard)
	if err := retry.Execute(); err != nil {
		t.Fatalf("repetir diagnóstico: %v", err)
	}
	if !strings.Contains(retryOutput.String(), "Diagnóstico já existente: "+incidentID) {
		t.Errorf("retry não explica idempotência:\n%s", retryOutput.String())
	}
	assertTableCount(t, database, "incidents", 1)
	assertTableCount(t, database, "findings", 4)
	assertTableCount(t, database, "ranking_results", 1)
}

// TestDiagnoseIncidentCommandDetectaRetryStorm cobre a ingestão, o ranking e a
// persistência de uma repetição anormal da mesma chamada outbound por trace.
func TestDiagnoseIncidentCommandDetectaRetryStorm(t *testing.T) {
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

	for _, fixtureName := range []string{"checkout-retry-baseline.json", "checkout-retry-incident.json"} {
		fixturePath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "otel", fixtureName))
		if err != nil {
			t.Fatalf("resolver fixture %q: %v", fixtureName, err)
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
		"diagnose", "incident", "--config", configPath,
		"--service", "checkout-service", "--since", "1m", "--baseline", "1m",
		"--until", "2025-12-01T10:02:00Z", "--limit", "100",
	})
	diagnose.SetOut(&output)
	diagnose.SetErr(io.Discard)
	if err := diagnose.Execute(); err != nil {
		t.Fatalf("diagnose incident erro = %v", err)
	}

	diagnosisOutput := output.String()
	for _, expected := range []string{
		"retry_storm",
		"POST",
		"payment-service",
		"média de tentativas por trace aumentou de 1.00 para 3.00",
		"5 de 5 traces",
		"Confiança: alta",
		"retry_storm: score",
		"× peso",
		"Diagnóstico salvo: inc_",
	} {
		if !strings.Contains(diagnosisOutput, expected) {
			t.Errorf("diagnóstico de retry não contém %q:\n%s", expected, diagnosisOutput)
		}
	}

	database, err := sql.Open("sqlite", filepath.Join(projectDir, "faultmap.db"))
	if err != nil {
		t.Fatalf("abrir banco para verificar retry_storm: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar banco de verificação: %v", closeErr)
		}
	})
	var incidentID string
	if err := database.QueryRow(`SELECT incident_id FROM findings WHERE rule_id = ?`, "retry_storm").Scan(&incidentID); err != nil {
		t.Fatalf("ler finding retry_storm persistido: %v", err)
	}

	var persistedOutput bytes.Buffer
	show := newRootCommand()
	show.SetArgs([]string{"incident", "show", "--config", configPath, "--id", incidentID})
	show.SetOut(&persistedOutput)
	show.SetErr(io.Discard)
	if err := show.Execute(); err != nil {
		t.Fatalf("consultar snapshot com retry_storm: %v", err)
	}
	for _, expected := range []string{"retry_storm", "POST", "payment-service", "Confiança: alta"} {
		if !strings.Contains(persistedOutput.String(), expected) {
			t.Errorf("snapshot de retry não contém %q:\n%s", expected, persistedOutput.String())
		}
	}
}

// TestDiagnoseIncidentCommandNaoPersisteJanelaDeIncidenteVazia garante que
// telemetria ainda não recebida não produza um snapshot imutável incompleto.
func TestDiagnoseIncidentCommandNaoPersisteJanelaDeIncidenteVazia(t *testing.T) {
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

	fixturePath, err := filepath.Abs(filepath.Join("..", "..", "fixtures", "otel", "checkout-baseline-sample.json"))
	if err != nil {
		t.Fatalf("resolver fixture: %v", err)
	}
	ingest := newRootCommand()
	ingest.SetArgs([]string{"ingest", "file", "--input", fixturePath, "--config", configPath})
	ingest.SetOut(io.Discard)
	ingest.SetErr(io.Discard)
	if err := ingest.Execute(); err != nil {
		t.Fatalf("ingerir baseline: %v", err)
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
		"Baseline: 40 sinais · Incidente: 0 sinais",
		"Nenhuma anomalia determinística",
		"Diagnóstico não salvo: janela do incidente sem sinais.",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("saída não contém %q:\n%s", expected, output.String())
		}
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
	assertTableCount(t, database, "incidents", 0)
	assertTableCount(t, database, "findings", 0)
	assertTableCount(t, database, "ranking_results", 0)
}

// TestIncidentListCommandExibeSnapshotsLimitados garante que a consulta de
// histórico apresente somente o resumo persistido, sem recalcular diagnósticos.
func TestIncidentListCommandExibeSnapshotsLimitados(t *testing.T) {
	t.Parallel()

	configPath, incidentID := preparePersistedIncident(t)
	var output bytes.Buffer
	command := newRootCommand()
	command.SetArgs([]string{"incident", "list", "--config", configPath, "--limit", "10"})
	command.SetOut(&output)
	command.SetErr(io.Discard)
	if err := command.Execute(); err != nil {
		t.Fatalf("incident list erro = %v", err)
	}
	for _, expected := range []string{
		"Incidentes persistidos — 1",
		incidentID,
		"checkout-service",
		"Status: diagnosed",
		"2025-12-01 10:01:00 UTC",
		"2025-12-01 10:02:00 UTC",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("saída não contém %q:\n%s", expected, output.String())
		}
	}
}

// TestIncidentShowCommandRecuperaDiagnosticoCompleto garante que findings e
// ranking sejam lidos do snapshot, inclusive as contagens originais das janelas.
func TestIncidentShowCommandRecuperaDiagnosticoCompleto(t *testing.T) {
	t.Parallel()

	configPath, incidentID := preparePersistedIncident(t)
	var output bytes.Buffer
	command := newRootCommand()
	command.SetArgs([]string{"incident", "show", "--config", configPath, "--id", incidentID})
	command.SetOut(&output)
	command.SetErr(io.Discard)
	if err := command.Execute(); err != nil {
		t.Fatalf("incident show erro = %v", err)
	}
	result := output.String()
	for _, expected := range []string{
		"Incidente persistido — " + incidentID,
		"Serviço: checkout-service",
		"Status: diagnosed",
		"Baseline: 40 sinais",
		"Incidente: 40 sinais",
		"Score agregado: 0.40",
		"database_http_trace_correlation",
		"database_timeout",
		"error_rate_delta",
		"latency_delta",
		"Correlação entre sinais não comprova causalidade.",
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("saída não contém %q:\n%s", expected, result)
		}
	}
}

// TestIncidentCommandsValidamEntradaAntesDoBanco evita abrir o pool para
// comandos que já possuem limite ou identificador inválido.
func TestIncidentCommandsValidamEntradaAntesDoBanco(t *testing.T) {
	t.Parallel()

	list := newRootCommand()
	list.SetArgs([]string{"incident", "list", "--config", "inexistente.yaml", "--limit", "0"})
	list.SetOut(io.Discard)
	list.SetErr(io.Discard)
	if err := list.Execute(); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("incident list erro = %v, esperado limite inválido", err)
	}

	show := newRootCommand()
	show.SetArgs([]string{"incident", "show", "--config", "inexistente.yaml", "--id", " "})
	show.SetOut(io.Discard)
	show.SetErr(io.Discard)
	if err := show.Execute(); err == nil || !strings.Contains(err.Error(), "--id") {
		t.Fatalf("incident show erro = %v, esperado ID obrigatório", err)
	}
}

// TestExportReportCommandGeraJSONEMarkdownDoMesmoSnapshot garante formatos
// estruturados e humanos sem executar novamente detectores ou ranking.
func TestExportReportCommandGeraJSONEMarkdownDoMesmoSnapshot(t *testing.T) {
	t.Parallel()

	configPath, incidentID := preparePersistedIncident(t)

	var jsonOutput bytes.Buffer
	jsonCommand := newRootCommand()
	jsonCommand.SetArgs([]string{
		"export", "report", "--config", configPath,
		"--incident", incidentID, "--format", "json",
	})
	jsonCommand.SetOut(&jsonOutput)
	jsonCommand.SetErr(io.Discard)
	if err := jsonCommand.Execute(); err != nil {
		t.Fatalf("export report JSON erro = %v", err)
	}
	var document struct {
		SchemaVersion string `json:"schema_version"`
		Incident      struct {
			ID          string `json:"id"`
			ServiceName string `json:"service_name"`
		} `json:"incident"`
		Baseline *struct {
			SignalCount int `json:"signal_count"`
		} `json:"baseline"`
		IncidentWindow struct {
			SignalCount int `json:"signal_count"`
		} `json:"incident_window"`
		Findings []struct {
			RuleID string `json:"rule_id"`
		} `json:"findings"`
		Ranking []struct {
			ID    string  `json:"id"`
			Score float64 `json:"score"`
		} `json:"ranking"`
	}
	if err := json.Unmarshal(jsonOutput.Bytes(), &document); err != nil {
		t.Fatalf("decodificar relatório JSON: %v\n%s", err, jsonOutput.String())
	}
	if document.SchemaVersion != "1" || document.Incident.ID != incidentID || document.Incident.ServiceName != "checkout-service" {
		t.Fatalf("cabeçalho JSON inesperado: %#v", document)
	}
	if document.Baseline == nil || document.Baseline.SignalCount != 40 || document.IncidentWindow.SignalCount != 40 {
		t.Fatalf("janelas JSON inesperadas: baseline=%#v incidente=%#v", document.Baseline, document.IncidentWindow)
	}
	if len(document.Findings) != 4 || len(document.Ranking) != 1 || document.Ranking[0].Score < 0.4035 || document.Ranking[0].Score > 0.4037 {
		t.Fatalf("análise JSON inesperada: findings=%d ranking=%#v", len(document.Findings), document.Ranking)
	}

	var markdownOutput bytes.Buffer
	markdownCommand := newRootCommand()
	markdownCommand.SetArgs([]string{
		"export", "report", "--config", configPath,
		"--incident", incidentID, "--format", "markdown",
	})
	markdownCommand.SetOut(&markdownOutput)
	markdownCommand.SetErr(io.Discard)
	if err := markdownCommand.Execute(); err != nil {
		t.Fatalf("export report Markdown erro = %v", err)
	}
	for _, expected := range []string{
		"# Diagnóstico do incidente `" + incidentID + "`",
		"**Serviço:** checkout-service",
		"**Baseline:** 40 sinais",
		"## Ranking de suspeitos",
		"## Evidências",
		"database_http_trace_correlation",
		"Correlação entre sinais não comprova causalidade.",
	} {
		if !strings.Contains(markdownOutput.String(), expected) {
			t.Errorf("Markdown não contém %q:\n%s", expected, markdownOutput.String())
		}
	}
}

// TestExportReportCommandValidaEntradaAntesDoBanco evita I/O quando ID ou
// formato já são inválidos.
func TestExportReportCommandValidaEntradaAntesDoBanco(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "incident required", args: []string{"export", "report", "--config", "inexistente.yaml", "--incident", " "}, want: "--incident"},
		{name: "format allowlist", args: []string{"export", "report", "--config", "inexistente.yaml", "--incident", "inc_1", "--format", "html"}, want: "--format"},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := newRootCommand()
			command.SetArgs(test.args)
			command.SetOut(io.Discard)
			command.SetErr(io.Discard)
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("export report erro = %v, esperado %q", err, test.want)
			}
		})
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

func assertTableCount(t *testing.T, database *sql.DB, table string, want int) {
	t.Helper()
	allowed := map[string]struct{}{
		"incidents":       {},
		"findings":        {},
		"ranking_results": {},
	}
	if _, ok := allowed[table]; !ok {
		t.Fatalf("tabela não permitida no teste: %q", table)
	}
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatalf("contar %s: %v", table, err)
	}
	if count != want {
		t.Fatalf("quantidade em %s = %d, esperado %d", table, count, want)
	}
}

func preparePersistedIncident(t *testing.T) (configPath string, incidentID string) {
	t.Helper()
	projectDir := t.TempDir()
	configPath = filepath.Join(projectDir, "faultmap.yaml")
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
			t.Fatalf("resolver fixture %q: %v", fixtureName, err)
		}
		ingest := newRootCommand()
		ingest.SetArgs([]string{"ingest", "file", "--input", fixturePath, "--config", configPath})
		ingest.SetOut(io.Discard)
		ingest.SetErr(io.Discard)
		if err := ingest.Execute(); err != nil {
			t.Fatalf("ingerir fixture %q: %v", fixtureName, err)
		}
	}
	diagnose := newRootCommand()
	diagnose.SetArgs([]string{
		"diagnose", "incident", "--config", configPath,
		"--service", "checkout-service", "--since", "1m", "--baseline", "1m",
		"--until", "2025-12-01T10:02:00Z", "--limit", "100",
	})
	diagnose.SetOut(io.Discard)
	diagnose.SetErr(io.Discard)
	if err := diagnose.Execute(); err != nil {
		t.Fatalf("diagnosticar fixture: %v", err)
	}
	database, err := sql.Open("sqlite", filepath.Join(projectDir, "faultmap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar banco: %v", closeErr)
		}
	}()
	if err := database.QueryRow(`SELECT id FROM incidents LIMIT 1`).Scan(&incidentID); err != nil {
		t.Fatalf("ler ID persistido: %v", err)
	}
	return configPath, incidentID
}
