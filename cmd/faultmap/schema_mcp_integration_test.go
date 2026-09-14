package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	storage "github.com/faultmap/faultmap/internal/storage/sqlite"
)

// workspaceInicializado devolve o caminho do YAML de um workspace novo.
func workspaceInicializado(t *testing.T) string {
	t.Helper()

	projectDir := t.TempDir()
	initialize := newRootCommand()
	initialize.SetArgs([]string{"init", "--directory", projectDir})
	initialize.SetOut(io.Discard)
	initialize.SetErr(io.Discard)
	if err := initialize.Execute(); err != nil {
		t.Fatalf("init erro = %v", err)
	}
	return filepath.Join(projectDir, "faultmap.yaml")
}

func executar(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var output bytes.Buffer
	command := newRootCommand()
	command.SetArgs(args)
	command.SetOut(&output)
	command.SetErr(io.Discard)
	err := command.Execute()
	return output.String(), err
}

// gravarTelemetriaDeBanco insere spans de banco direto no SQLite.
//
// A telemetria entra por aqui, e não por uma fixture OTLP, porque o que este
// teste exercita é a ligação entre catálogo e diagnóstico: precisamos de spans
// com `db.namespace` em uma janela conhecida, e montar isso como payload OTLP
// afastaria o teste do que ele quer provar.
func gravarTelemetriaDeBanco(
	t *testing.T, configPath, serviceName, databaseName string,
	instante time.Time, total int, prefixo string, duracaoMS float64,
) {
	t.Helper()

	database, err := sql.Open("sqlite", filepath.Join(filepath.Dir(configPath), "faultmap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	for indice := range total {
		attributes, err := json.Marshal(map[string]string{
			"db.system.name":    "postgresql",
			"db.namespace":      databaseName,
			"db.operation.name": "SELECT",
			"span.kind":         "SPAN_KIND_CLIENT",
		})
		if err != nil {
			t.Fatalf("serializar atributos: %v", err)
		}
		if _, err := database.Exec(`
			INSERT INTO signals (id, signal_type, service_name, timestamp, trace_id, span_id, severity, attributes_json, measurements_json)
			VALUES (?, 'span', ?, ?, ?, ?, 'INFO', ?, ?)
		`,
			prefixo+"-"+serviceName+"-db-"+string(rune('a'+indice)),
			serviceName,
			instante.Add(time.Duration(indice)*time.Second).UTC(),
			prefixo+"-trace-"+string(rune('a'+indice)),
			prefixo+"-span-"+string(rune('a'+indice)),
			string(attributes),
			fmt.Sprintf(`{"duration_ms":%.0f}`, duracaoMS),
		); err != nil {
			t.Fatalf("inserir sinal: %v", err)
		}
	}
}

func gravarColetasDeCatalogo(t *testing.T, configPath string, coletas ...changedomain.SchemaSnapshot) {
	t.Helper()

	database, err := storage.Open(context.Background(), filepath.Join(filepath.Dir(configPath), "faultmap.db"))
	if err != nil {
		t.Fatalf("abrir banco: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := storage.Migrate(context.Background(), database); err != nil {
		t.Fatalf("migrar: %v", err)
	}

	repository := storage.NewSchemaRepository(database)
	for _, coleta := range coletas {
		if _, err := repository.SaveSnapshot(context.Background(), coleta); err != nil {
			t.Fatalf("SaveSnapshot(%s) erro = %v", coleta.ID, err)
		}
	}
}

// TestPipelineDeSchemaChegaAoDiagnosticoEAoMCP percorre a funcionalidade
// inteira pelas mesmas portas que uma pessoa usa: duas coletas do catálogo,
// telemetria de banco, `diagnose incident` e, no fim, a leitura por MCP.
//
// É o teste que pega desligamento de fiação entre camadas — cada camada tem
// unidade verde e ainda assim o sinal pode não chegar ao relatório.
func TestPipelineDeSchemaChegaAoDiagnosticoEAoMCP(t *testing.T) {
	t.Parallel()

	configPath := workspaceInicializado(t)
	incidente := time.Now().UTC().Add(-2 * time.Minute)

	// A baseline é rápida e o incidente é lento: a mudança de schema é evidência
	// de apoio, e sem sintoma observado ela não é apresentada. O índice removido
	// deixando as consultas lentas é justamente o sintoma que ela corrobora.
	gravarTelemetriaDeBanco(t, configPath, "payment-service", "payments",
		incidente.Add(-30*time.Minute), 10, "baseline", 10)
	gravarTelemetriaDeBanco(t, configPath, "payment-service", "payments",
		incidente, 10, "incident", 800)
	gravarColetasDeCatalogo(t, configPath,
		changedomain.SchemaSnapshot{
			ID: "catalog:payments:1", DatabaseName: "payments", CapturedAt: incidente.Add(-4 * time.Hour),
			Objects: []changedomain.SchemaObject{
				{Kind: changedomain.SchemaObjectIndex, Name: "idx_payments_created_at"},
				{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"},
			},
		},
		changedomain.SchemaSnapshot{
			ID: "catalog:payments:2", DatabaseName: "payments", CapturedAt: incidente.Add(-3 * time.Hour),
			Objects: []changedomain.SchemaObject{
				{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"},
			},
		},
	)

	saida, err := executar(t, "diagnose", "incident",
		"--config", configPath, "--service", "payment-service",
		"--since", "10m", "--baseline", "60m",
	)
	if err != nil {
		t.Fatalf("diagnose erro = %v", err)
	}
	if !strings.Contains(saida, "schema_change_proximity") {
		t.Fatalf("o diagnóstico não trouxe a regra de schema:\n%s", saida)
	}
	if !strings.Contains(saida, "idx_payments_created_at") {
		t.Fatalf("a evidência não nomeia o objeto migrado:\n%s", saida)
	}
	// As duas limitações da ADR 0014 precisam chegar a quem lê, e não parar na
	// estrutura de dados.
	for _, limitacao := range []string{"causalidade", "coletas do catálogo"} {
		if !strings.Contains(saida, limitacao) {
			t.Fatalf("a limitação %q não chegou ao relatório:\n%s", limitacao, saida)
		}
	}

	// O mesmo diagnóstico, agora pela porta do MCP.
	sessao := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_incidents","arguments":{"limit":5}}}`
	respostaMCP := sessaoMCP(t, configPath, sessao)
	if !strings.Contains(respostaMCP, "payment-service") {
		t.Fatalf("o MCP não devolveu o incidente recém-diagnosticado: %s", respostaMCP)
	}
}

// sessaoMCP roda o comando mcp com as requisições dadas e devolve o stdout.
func sessaoMCP(t *testing.T, configPath string, requisicoes ...string) string {
	t.Helper()

	var saida bytes.Buffer
	command := newRootCommand()
	command.SetArgs([]string{"mcp", "--config", configPath})
	command.SetIn(strings.NewReader(strings.Join(requisicoes, "\n") + "\n"))
	command.SetOut(&saida)
	command.SetErr(io.Discard)
	if err := command.Execute(); err != nil {
		t.Fatalf("mcp erro = %v", err)
	}
	return saida.String()
}

// TestMCPCommandFalaOProtocoloDePontaAPonta é o smoke test do comando: sobe
// pelo Cobra, abre o SQLite real e completa um aperto de mão.
func TestMCPCommandFalaOProtocoloDePontaAPonta(t *testing.T) {
	t.Parallel()

	configPath := workspaceInicializado(t)
	saida := sessaoMCP(t, configPath,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
	)

	linhas := strings.Split(strings.TrimSpace(saida), "\n")
	if len(linhas) != 2 {
		t.Fatalf("linhas = %d, esperado 2: a notificação não pode gerar resposta\n%s", len(linhas), saida)
	}
	for _, linha := range linhas {
		if !json.Valid([]byte(linha)) {
			t.Fatalf("o comando escreveu algo que não é JSON-RPC em stdout: %q", linha)
		}
	}
	if !strings.Contains(linhas[1], "explain_suspect") {
		t.Fatalf("tools/list não trouxe as ferramentas: %s", linhas[1])
	}
}

// TestMCPCommandNaoPoluiStdout protege o transporte: qualquer coisa que o
// comando imprima na saída padrão corrompe o aperto de mão de todo cliente.
func TestMCPCommandNaoPoluiStdout(t *testing.T) {
	t.Parallel()

	configPath := workspaceInicializado(t)
	var saida, erros bytes.Buffer
	command := newRootCommand()
	command.SetArgs([]string{"mcp", "--config", configPath})
	command.SetIn(strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_incidents","arguments":{}}}` + "\n",
	))
	command.SetOut(&saida)
	command.SetErr(&erros)
	if err := command.Execute(); err != nil {
		t.Fatalf("mcp erro = %v", err)
	}

	for _, linha := range strings.Split(strings.TrimSpace(saida.String()), "\n") {
		if !json.Valid([]byte(linha)) {
			t.Fatalf("stdout contém linha que não é JSON-RPC: %q", linha)
		}
	}
	if !strings.Contains(erros.String(), "list_incidents") {
		t.Fatalf("a auditoria não saiu por stderr: %q", erros.String())
	}
}

// TestIngestSchemaCommandValidaEntradaAntesDeAbrirConexao garante que erro de
// uso apareça de imediato, e não depois de uma tentativa de conexão que
// demoraria a estourar timeout.
func TestIngestSchemaCommandValidaEntradaAntesDeAbrirConexao(t *testing.T) {
	// Sem t.Parallel: t.Setenv exige execução serial.

	configPath := workspaceInicializado(t)

	t.Setenv("FAULTMAP_PG_DSN", "postgres://usuario:senha@localhost:1/base")
	if _, err := executar(t, "ingest", "schema", "--config", configPath); err == nil {
		t.Fatal("ingest schema aceitou execução sem --database")
	} else if !strings.Contains(err.Error(), "--database") {
		t.Fatalf("erro = %v, esperado apontar a flag ausente", err)
	}

	if err := os.Unsetenv("FAULTMAP_PG_DSN"); err != nil {
		t.Fatalf("limpar variável: %v", err)
	}
	_, err := executar(t, "ingest", "schema", "--config", configPath, "--database", "payments")
	if err == nil {
		t.Fatal("ingest schema aceitou execução sem DSN")
	}
	if !strings.Contains(err.Error(), "FAULTMAP_PG_DSN") {
		t.Fatalf("erro = %v, esperado nomear a variável de ambiente", err)
	}
}

// TestIngestSchemaCommandNaoGravaCredencialNoWorkspace é a promessa da ADR
// 0014 verificada no disco: a DSN passa pelo processo e não pode ficar em
// arquivo nenhum do workspace.
func TestIngestSchemaCommandNaoGravaCredencialNoWorkspace(t *testing.T) {
	// Sem t.Parallel: t.Setenv exige execução serial.

	configPath := workspaceInicializado(t)
	const senha = "senha-secreta-do-banco"
	t.Setenv("FAULTMAP_PG_DSN", "postgres://usuario:"+senha+"@localhost:1/payments?sslmode=disable")

	// A conexão falha de propósito: o que importa é o que sobrou no disco depois.
	if _, err := executar(t, "ingest", "schema", "--config", configPath, "--database", "payments"); err == nil {
		t.Fatal("a conexão com uma porta inválida deveria falhar")
	}

	workspace := filepath.Dir(configPath)
	entradas, err := os.ReadDir(workspace)
	if err != nil {
		t.Fatalf("listar workspace: %v", err)
	}
	for _, entrada := range entradas {
		if entrada.IsDir() {
			continue
		}
		conteudo, err := os.ReadFile(filepath.Join(workspace, entrada.Name()))
		if err != nil {
			t.Fatalf("ler %s: %v", entrada.Name(), err)
		}
		if bytes.Contains(conteudo, []byte(senha)) {
			t.Fatalf("a credencial foi gravada em %s", entrada.Name())
		}
	}
}
