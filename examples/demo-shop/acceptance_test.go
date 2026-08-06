package demoshop_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var requiredScenarios = []string{
	"database-slow",
	"small-pool",
	"payment-500",
	"retry-storm",
	"timeout-after-deploy",
	"table-lock",
}

// TestScenariosDocumentamContratoReproduzivel garante que cada falha possui um
// roteiro autônomo, limitado e observável antes de depender da implementação.
func TestScenariosDocumentamContratoReproduzivel(t *testing.T) {
	t.Parallel()

	for _, scenario := range requiredScenarios {
		scenario := scenario
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()

			directory := filepath.Join("scenarios", scenario)
			readme := readRequiredFile(t, filepath.Join(directory, "README.md"))
			compose := readRequiredFile(t, filepath.Join(directory, "compose.yaml"))
			generatorPath := filepath.Join(directory, "generate-traffic.sh")
			generator := readRequiredFile(t, generatorPath)

			requireContains(t, readme, "## Como executar")
			requireContains(t, readme, "## Resultado esperado")
			requireContains(t, readme, "docker compose -f examples/demo-shop/compose.yaml")
			requireContains(t, readme, "generate-traffic.sh")
			requireContains(t, readme, "diagnose incident")

			requireContains(t, compose, "services:")
			requireContains(t, compose, "SERVICE_VERSION")
			var composeDocument map[string]any
			if err := yaml.Unmarshal([]byte(compose), &composeDocument); err != nil {
				t.Fatalf("%s não é YAML válido: %v", filepath.Join(directory, "compose.yaml"), err)
			}

			if !strings.HasPrefix(generator, "#!/bin/sh\nset -eu\n") {
				t.Fatalf("%s deve iniciar com #!/bin/sh e set -eu", generatorPath)
			}
			requireContains(t, generator, "--max-time")
			requireContains(t, generator, "REQUEST_COUNT")
			if strings.Contains(generator, "rm -rf") || strings.Contains(generator, "docker system prune") {
				t.Fatalf("%s contém limpeza destrutiva ampla", generatorPath)
			}
			info, err := os.Stat(generatorPath)
			if err != nil {
				t.Fatalf("inspecionar %s: %v", generatorPath, err)
			}
			if info.Mode().Perm()&0o111 == 0 {
				t.Fatalf("%s deve ser executável", generatorPath)
			}
		})
	}
}

// TestScenariosUsamControlesDistintos evita que dois nomes documentem a mesma
// falha e mantém os contratos de runtime explícitos nos overrides.
func TestScenariosUsamControlesDistintos(t *testing.T) {
	t.Parallel()

	expectedControls := map[string]string{
		"database-slow":        "DB_DELAY",
		"small-pool":           "DB_MAX_OPEN_CONNS",
		"payment-500":          "FORCE_HTTP_STATUS",
		"retry-storm":          "PAYMENT_MAX_ATTEMPTS",
		"timeout-after-deploy": "PAYMENT_TIMEOUT",
		"table-lock":           "DB_LOCK_DURATION",
	}

	for scenario, control := range expectedControls {
		compose := readRequiredFile(t, filepath.Join("scenarios", scenario, "compose.yaml"))
		requireContains(t, compose, control)
	}
}

// TestDemoUsaPortaDeHostDedicada evita conflitar com aplicações locais que
// frequentemente já ocupam 8080, sem alterar a porta interna dos serviços.
func TestDemoUsaPortaDeHostDedicada(t *testing.T) {
	t.Parallel()

	compose := readRequiredFile(t, "compose.yaml")
	requireContains(t, compose, `"127.0.0.1:18080:8080"`)
	for _, scenario := range requiredScenarios {
		script := readRequiredFile(t, filepath.Join("scenarios", scenario, "generate-traffic.sh"))
		requireContains(t, script, "http://localhost:18080/checkout")
	}
}

// TestDemoPublicaPortasSomenteNoLoopback preserva o contrato de uma demo local
// e evita expor ingestão OTLP ou serviços sem autenticação na rede do host.
func TestDemoPublicaPortasSomenteNoLoopback(t *testing.T) {
	t.Parallel()

	compose := readRequiredFile(t, "compose.yaml")
	for _, binding := range []string{
		`"127.0.0.1:4318:4318"`,
		`"127.0.0.1:8081:8081"`,
		`"127.0.0.1:18080:8080"`,
	} {
		requireContains(t, compose, binding)
	}
}

// TestLoadGeneratorRecebeIdentidadeOTel garante que o utilitário opcional
// inicia com a mesma identidade de resource exigida dos demais processos.
func TestLoadGeneratorRecebeIdentidadeOTel(t *testing.T) {
	t.Parallel()

	compose := readRequiredFile(t, "compose.yaml")
	loadGenerator := compose[strings.Index(compose, "  load-generator:"):]
	for _, variable := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT:",
		"SERVICE_VERSION:",
		"DEPLOYMENT_ENVIRONMENT:",
	} {
		requireContains(t, loadGenerator, variable)
	}
}

// TestReadmesDocumentamExecucaoDaDemo impede que o guia principal volte a
// anunciar a demonstração como futura ou use a porta interna no host.
func TestReadmesDocumentamExecucaoDaDemo(t *testing.T) {
	t.Parallel()

	rootREADME := readRequiredFile(t, filepath.Join("..", "..", "README.md"))
	for _, expected := range []string{
		"## Demo Shop",
		"make demo-up",
		"http://127.0.0.1:18080/checkout",
		"faultmap telemetry list",
		"make demo-down",
	} {
		requireContains(t, rootREADME, expected)
	}
	if strings.Contains(rootREADME, "Ainda não há uma demo executável") {
		t.Fatal("README raiz ainda anuncia a demo executável como futura")
	}

	demoREADME := readRequiredFile(t, "README.md")
	requireContains(t, demoREADME, "portas locais `18080`, `4318` e `8081`")
	requireContains(t, demoREADME, "up --build -d --wait")
}

// TestSchemaDemoNaoCriaIndiceSemCasoDeUso protege o custo de escrita da demo;
// a aplicação atual busca pagamentos somente pela constraint de order_id.
func TestSchemaDemoNaoCriaIndiceSemCasoDeUso(t *testing.T) {
	t.Parallel()

	schema := readRequiredFile(t, filepath.Join("postgres", "001-init.sql"))
	if strings.Contains(strings.ToUpper(schema), "CREATE INDEX") {
		t.Fatal("schema criou índice sem consulta real que justifique seu custo")
	}
}

// TestTimeoutDeployAceitaSHAReal evita documentar correspondência impossível:
// deployment_proximity compara commit_sha literalmente com service.version.
func TestTimeoutDeployAceitaSHAReal(t *testing.T) {
	t.Parallel()

	compose := readRequiredFile(t, filepath.Join("scenarios", "timeout-after-deploy", "compose.yaml"))
	requireContains(t, compose, "${TIMEOUT_DEPLOY_VERSION:-2.0.0-timeout-regression}")
	readme := readRequiredFile(t, filepath.Join("scenarios", "timeout-after-deploy", "README.md"))
	requireContains(t, readme, "TIMEOUT_DEPLOY_VERSION=<commit_sha>")
}

// readRequiredFile lê artefatos obrigatórios e encerra o subteste com uma
// mensagem que aponta diretamente para o contrato ausente.
func readRequiredFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler %s: %v", path, err)
	}
	return string(content)
}

// requireContains mantém as asserções textuais pequenas e informa qual parte
// do contrato deixou de estar documentada.
func requireContains(t *testing.T, content, expected string) {
	t.Helper()

	if !strings.Contains(content, expected) {
		t.Fatalf("conteúdo não possui %q", expected)
	}
}
