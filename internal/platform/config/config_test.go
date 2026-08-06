package config

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultYAMLContémTodasAsSeções impede que o init gere uma configuração parcial.
func TestDefaultYAMLContémTodasAsSeções(t *testing.T) {
	t.Parallel()

	content, err := DefaultYAML()
	if err != nil {
		t.Fatalf("DefaultYAML() erro = %v", err)
	}
	for _, section := range [][]byte{
		[]byte("server:"),
		[]byte("storage:"),
		[]byte("investigation:"),
		[]byte("ranking:"),
		[]byte("github:"),
		[]byte("privacy:"),
	} {
		if !bytes.Contains(content, section) {
			t.Fatalf("DefaultYAML() não contém a seção %q", section)
		}
	}
	if !bytes.Contains(content, []byte("max_request_body_bytes: 67108864")) {
		t.Fatalf("DefaultYAML() não contém o limite OTLP de 64 MiB: %s", content)
	}
}

// TestLoadLêEValidaConfiguração verifica a desserialização das opções do Faultmap.
func TestLoadLêEValidaConfiguração(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "faultmap.yaml")
	content := `server:
  otlp_http_address: "127.0.0.1:4318"
  health_address: "127.0.0.1:8081"
  max_request_body_bytes: 4194304
  read_header_timeout: "3s"
  read_timeout: "20s"
  write_timeout: "25s"
  idle_timeout: "45s"
  shutdown_timeout: "8s"
storage:
  driver: "sqlite"
  path: "./data/faultmap.db"
  retention: "14d"
investigation:
  default_incident_window: "15m"
  default_baseline_window: "45m"
  top_suspects: 5
ranking:
  weights:
    error_rate_delta: 0.30
    deployment_proximity: 0.15
    database_evidence: 0.20
    graph_proximity: 0.15
    latency_delta: 0.10
    log_correlation: 0.10
github:
  enabled: true
  repository: "acme/checkout"
  environment: "staging"
privacy:
  store_raw_logs: false
  max_attribute_length: 256
  blocked_attributes: [http.request.body]
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("escrever configuração: %v", err)
	}

	loaded, err := Load(context.Background(), configPath)
	if err != nil {
		t.Fatalf("Load() erro = %v", err)
	}
	if loaded.Storage.Path != "./data/faultmap.db" {
		t.Fatalf("Storage.Path = %q, esperado ./data/faultmap.db", loaded.Storage.Path)
	}
	if loaded.Server.MaxRequestBodyBytes != 4*1024*1024 {
		t.Fatalf("Server.MaxRequestBodyBytes = %d, esperado 4194304", loaded.Server.MaxRequestBodyBytes)
	}
	if loaded.Server.ReadHeaderTimeout != "3s" || loaded.Server.ShutdownTimeout != "8s" {
		t.Fatalf("timeouts do servidor = %#v, esperados valores do YAML", loaded.Server)
	}
	if loaded.Investigation.TopSuspects != 5 {
		t.Fatalf("Investigation.TopSuspects = %d, esperado 5", loaded.Investigation.TopSuspects)
	}
	if loaded.Ranking.Weights.ErrorRateDelta != 0.30 {
		t.Fatalf("Ranking.Weights.ErrorRateDelta = %v, esperado 0.30", loaded.Ranking.Weights.ErrorRateDelta)
	}
	if !loaded.GitHub.Enabled {
		t.Fatal("GitHub.Enabled = false, esperado true")
	}
}

// TestLoadPreservaDefaultsDoServidorEmYAMLAntigo garante que workspaces
// existentes recebam limites seguros sem exigir novos campos imediatamente.
func TestLoadPreservaDefaultsDoServidorEmYAMLAntigo(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "faultmap.yaml")
	content := `server:
  otlp_http_address: "127.0.0.1:4318"
  health_address: "127.0.0.1:8081"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("escrever configuração: %v", err)
	}

	loaded, err := Load(context.Background(), configPath)
	if err != nil {
		t.Fatalf("Load() erro = %v", err)
	}
	defaults := Default().Server
	if loaded.Server.MaxRequestBodyBytes != defaults.MaxRequestBodyBytes ||
		loaded.Server.ReadHeaderTimeout != defaults.ReadHeaderTimeout ||
		loaded.Server.ReadTimeout != defaults.ReadTimeout ||
		loaded.Server.WriteTimeout != defaults.WriteTimeout ||
		loaded.Server.IdleTimeout != defaults.IdleTimeout ||
		loaded.Server.ShutdownTimeout != defaults.ShutdownTimeout {
		t.Fatalf("Server = %#v, esperado preservar defaults %#v", loaded.Server, defaults)
	}
}

// TestValidateRejeitaLimitesOperacionaisInválidos impede iniciar um receiver
// sem limite de corpo ou sem proteção temporal contra clientes lentos.
func TestValidateRejeitaLimitesOperacionaisInválidos(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		change func(*Config)
	}{
		{name: "endereço OTLP vazio", change: func(configuration *Config) { configuration.Server.OTLPHTTPAddress = " " }},
		{name: "endereço OTLP sem porta", change: func(configuration *Config) { configuration.Server.OTLPHTTPAddress = "127.0.0.1" }},
		{name: "endereço health vazio", change: func(configuration *Config) { configuration.Server.HealthAddress = " " }},
		{name: "endereço health inválido", change: func(configuration *Config) { configuration.Server.HealthAddress = "http://127.0.0.1:8081" }},
		{name: "listeners iguais", change: func(configuration *Config) { configuration.Server.HealthAddress = configuration.Server.OTLPHTTPAddress }},
		{name: "corpo sem limite", change: func(configuration *Config) { configuration.Server.MaxRequestBodyBytes = 0 }},
		{name: "timeout de cabeçalho inválido", change: func(configuration *Config) { configuration.Server.ReadHeaderTimeout = "nunca" }},
		{name: "timeout de leitura inválido", change: func(configuration *Config) { configuration.Server.ReadTimeout = "0s" }},
		{name: "timeout de escrita inválido", change: func(configuration *Config) { configuration.Server.WriteTimeout = "-1s" }},
		{name: "timeout ocioso inválido", change: func(configuration *Config) { configuration.Server.IdleTimeout = "" }},
		{name: "timeout de encerramento inválido", change: func(configuration *Config) { configuration.Server.ShutdownTimeout = "amanhã" }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			configuration := Default()
			testCase.change(&configuration)
			if err := configuration.Validate(); err == nil {
				t.Fatalf("Validate() erro = nil para server %#v", configuration.Server)
			}
		})
	}
}

// TestLoadRejeitaConfiguraçãoInválida garante que valores inválidos não chegam ao bootstrap.
func TestLoadRejeitaConfiguraçãoInválida(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "faultmap.yaml")
	content := `storage:
  driver: "sqlite"
  path: "./faultmap.db"
investigation:
  default_incident_window: "30m"
  default_baseline_window: "60m"
  top_suspects: 0
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("escrever configuração: %v", err)
	}

	_, err := Load(context.Background(), configPath)
	if err == nil {
		t.Fatal("Load() erro = nil, esperado erro de validação")
	}
}

// TestLoadRejeitaCampoDesconhecido evita erros de digitação silenciosos no YAML.
func TestLoadRejeitaCampoDesconhecido(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "faultmap.yaml")
	content := `storage:
  driver: "sqlite"
  path: "./faultmap.db"
  retention: "7d"
  caminho: "./ignorado.db"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("escrever configuração: %v", err)
	}

	_, err := Load(context.Background(), configPath)
	if err == nil {
		t.Fatal("Load() erro = nil, esperado erro para campo desconhecido")
	}
}

// TestLoadRejeitaDuraçõesInválidas impede janelas e retenções impossíveis de usar.
func TestLoadRejeitaDuraçõesInválidas(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		content string
	}{
		{
			name: "retenção inválida",
			content: `storage:
  retention: "amanhã"
`,
		},
		{
			name: "janela de incidente inválida",
			content: `investigation:
  default_incident_window: "zero"
`,
		},
		{
			name: "janela de baseline inválida",
			content: `investigation:
  default_baseline_window: "0m"
`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			configPath := filepath.Join(t.TempDir(), "faultmap.yaml")
			if err := os.WriteFile(configPath, []byte(testCase.content), 0o600); err != nil {
				t.Fatalf("escrever configuração: %v", err)
			}

			_, err := Load(context.Background(), configPath)
			if err == nil {
				t.Fatal("Load() erro = nil, esperado erro para duração inválida")
			}
		})
	}
}

// TestLoadRejeitaGitHubHabilitadoSemRepositório evita iniciar uma integração incompleta.
func TestLoadRejeitaGitHubHabilitadoSemRepositório(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "faultmap.yaml")
	content := `github:
  enabled: true
  repository: "   "
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("escrever configuração: %v", err)
	}

	_, err := Load(context.Background(), configPath)
	if err == nil {
		t.Fatal("Load() erro = nil, esperado erro para integração GitHub sem repositório")
	}
}

// TestValidateRejeitaPesosDeRankingInválidos impede scores negativos,
// contribuições acima do contrato e configurações sem qualquer contribuição.
func TestValidateRejeitaPesosDeRankingInválidos(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		weights RankingWeights
	}{
		{name: "peso negativo", weights: RankingWeights{ErrorRateDelta: -0.1}},
		{name: "peso acima de um", weights: RankingWeights{ErrorRateDelta: 1.1}},
		{name: "soma zero", weights: RankingWeights{}},
		{name: "soma acima de um", weights: RankingWeights{ErrorRateDelta: 0.6, LatencyDelta: 0.5}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			configuration := Default()
			configuration.Ranking.Weights = testCase.weights
			if err := configuration.Validate(); err == nil {
				t.Fatalf("Validate() erro = nil para %#v", testCase.weights)
			}
		})
	}
}

// TestValidateAceitaPesosDefault confirma que a configuração distribuída pode
// alimentar o ranking sem normalização implícita.
func TestValidateAceitaPesosDefault(t *testing.T) {
	t.Parallel()

	if err := Default().Validate(); err != nil {
		t.Fatalf("Default().Validate() erro = %v", err)
	}
}

// TestLoadRespeitaCancelamento garante que o carregamento não inicia I/O após cancelamento.
func TestLoadRespeitaCancelamento(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Load(ctx, filepath.Join(t.TempDir(), "faultmap.yaml"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Load() erro = %v, esperado context.Canceled", err)
	}
}
