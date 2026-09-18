package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// workspacePostgres monta um workspace apontando para o PostgreSQL de teste.
//
// Pula quando FAULTMAP_TEST_PG_DSN não está definida, como o restante da suíte
// de integração: a bateria precisa continuar passando numa máquina sem Docker.
func workspacePostgres(t *testing.T) string {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("FAULTMAP_TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("FAULTMAP_TEST_PG_DSN não definida; backend PostgreSQL na CLI ignorado")
	}
	configPath := workspaceInicializado(t)
	conteudo, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ler configuração: %v", err)
	}
	// O driver vem do YAML; a DSN vem do ambiente e nunca é gravada em arquivo.
	trocado := strings.Replace(string(conteudo), "driver: sqlite", "driver: postgres", 1)
	if trocado == string(conteudo) {
		t.Fatalf("não encontrei 'driver: sqlite' na configuração padrão:\n%s", conteudo)
	}
	if err := os.WriteFile(configPath, []byte(trocado), 0o600); err != nil {
		t.Fatalf("gravar configuração: %v", err)
	}
	t.Setenv("FAULTMAP_STORAGE_DSN", dsn)
	return configPath
}

// TestComandosDeLeituraFuncionamComPostgres é o que prova que o backend
// alternativo serve à CLI, e não só à bateria de conformidade.
//
// Os comandos de leitura são o alvo certo: eles abrem o banco, aplicam
// migrations e consultam. Se o backend não estivesse religado, eles abririam um
// SQLite inexistente e falhariam, ou pior, responderiam de um banco vazio.
func TestComandosDeLeituraFuncionamComPostgres(t *testing.T) {
	configPath := workspacePostgres(t)

	for _, args := range [][]string{
		{"incident", "list", "--config", configPath},
		{"telemetry", "list", "--config", configPath, "--service", "checkout"},
	} {
		if _, err := executar(t, args...); err != nil {
			t.Fatalf("%v com backend PostgreSQL erro = %v", args, err)
		}
	}
}

// TestRetencaoFuncionaComPostgres cobre o comando que mexe nas duas frentes da
// retenção — telemetria e catálogo — pelo backend alternativo.
func TestRetencaoFuncionaComPostgres(t *testing.T) {
	configPath := workspacePostgres(t)

	saida, err := executar(t, "retention", "apply", "--config", configPath)
	if err != nil {
		t.Fatalf("retention apply com PostgreSQL erro = %v", err)
	}
	if !strings.Contains(saida, "catálogos de schema liberados") {
		t.Fatalf("saída não relatou a liberação de catálogos:\n%s", saida)
	}
}

// TestBackendPostgresNaoCriaArquivoLocal é a metade do pedido de sessão sem
// persistência local que o PostgreSQL resolve: com ele o dado vive no servidor,
// e o diretório do projeto não recebe banco nenhum.
func TestBackendPostgresNaoCriaArquivoLocal(t *testing.T) {
	configPath := workspacePostgres(t)
	workspace := filepath.Dir(configPath)

	// O init ainda cria o arquivo; o que este caso prova é que os comandos
	// seguintes não voltam a escrever nele quando o driver é PostgreSQL.
	if err := os.Remove(filepath.Join(workspace, "faultmap.db")); err != nil {
		t.Fatalf("remover banco local: %v", err)
	}
	if _, err := executar(t, "incident", "list", "--config", configPath); err != nil {
		t.Fatalf("incident list com PostgreSQL erro = %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "faultmap.db")); !os.IsNotExist(err) {
		t.Fatalf("o comando recriou o banco local apesar do driver PostgreSQL: %v", err)
	}
}
