// Package config carrega e valida a configuração local do Faultmap.
package config

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config reúne as opções que definem o comportamento local do processo Faultmap.
type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Storage       StorageConfig       `yaml:"storage"`
	Investigation InvestigationConfig `yaml:"investigation"`
	Ranking       RankingConfig       `yaml:"ranking"`
	GitHub        GitHubConfig        `yaml:"github"`
	Privacy       PrivacyConfig       `yaml:"privacy"`
}

// ServerConfig define os endereços dos listeners locais do processo.
type ServerConfig struct {
	OTLPHTTPAddress string `yaml:"otlp_http_address"`
	HealthAddress   string `yaml:"health_address"`
}

// StorageConfig define a persistência local e sua política de retenção.
type StorageConfig struct {
	Driver    string `yaml:"driver"`
	Path      string `yaml:"path"`
	Retention string `yaml:"retention"`
}

// InvestigationConfig define os valores padrão das janelas e do ranking exibido.
type InvestigationConfig struct {
	DefaultIncidentWindow string `yaml:"default_incident_window"`
	DefaultBaselineWindow string `yaml:"default_baseline_window"`
	TopSuspects           int    `yaml:"top_suspects"`
}

// RankingConfig agrupa os pesos configuráveis dos detectores determinísticos.
type RankingConfig struct {
	Weights RankingWeights `yaml:"weights"`
}

// RankingWeights define a contribuição máxima de cada classe de evidência.
type RankingWeights struct {
	ErrorRateDelta      float64 `yaml:"error_rate_delta"`
	DeploymentProximity float64 `yaml:"deployment_proximity"`
	DatabaseEvidence    float64 `yaml:"database_evidence"`
	GraphProximity      float64 `yaml:"graph_proximity"`
	LatencyDelta        float64 `yaml:"latency_delta"`
	LogCorrelation      float64 `yaml:"log_correlation"`
}

// GitHubConfig habilita a correlação opcional com commits e deployments.
type GitHubConfig struct {
	Enabled     bool   `yaml:"enabled"`
	APIURL      string `yaml:"api_url"`
	Repository  string `yaml:"repository"`
	Environment string `yaml:"environment"`
}

// PrivacyConfig limita a retenção de atributos potencialmente sensíveis.
type PrivacyConfig struct {
	StoreRawLogs       bool     `yaml:"store_raw_logs"`
	MaxAttributeLength int      `yaml:"max_attribute_length"`
	BlockedAttributes  []string `yaml:"blocked_attributes"`
}

// Default retorna uma configuração local segura para o primeiro uso do Faultmap.
func Default() Config {
	return Config{
		Server: ServerConfig{
			OTLPHTTPAddress: "0.0.0.0:4318",
			HealthAddress:   "0.0.0.0:8081",
		},
		Storage: StorageConfig{
			Driver:    "sqlite",
			Path:      "./faultmap.db",
			Retention: "7d",
		},
		Investigation: InvestigationConfig{
			DefaultIncidentWindow: "30m",
			DefaultBaselineWindow: "60m",
			TopSuspects:           3,
		},
		Ranking: RankingConfig{Weights: RankingWeights{
			ErrorRateDelta:      0.25,
			DeploymentProximity: 0.20,
			DatabaseEvidence:    0.20,
			GraphProximity:      0.15,
			LatencyDelta:        0.10,
			LogCorrelation:      0.10,
		}},
		GitHub: GitHubConfig{APIURL: "https://api.github.com", Environment: "staging"},
		Privacy: PrivacyConfig{
			MaxAttributeLength: 512,
			BlockedAttributes:  []string{"http.request.body", "db.statement"},
		},
	}
}

// DefaultYAML serializa os defaults completos para o arquivo criado pelo comando init.
func DefaultYAML() ([]byte, error) {
	content, err := yaml.Marshal(Default())
	if err != nil {
		return nil, fmt.Errorf("serializar configuração padrão: %w", err)
	}
	return content, nil
}

// Load lê, desserializa e valida a configuração indicada sem iniciar I/O após cancelamento.
func Load(ctx context.Context, path string) (Config, error) {
	if err := contextError(ctx); err != nil {
		return Config{}, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("ler configuração %q: %w", path, err)
	}
	if err := contextError(ctx); err != nil {
		return Config{}, err
	}

	loaded := Default()
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&loaded); err != nil {
		return Config{}, fmt.Errorf("interpretar configuração %q: %w", path, err)
	}
	if err := loaded.Validate(); err != nil {
		return Config{}, err
	}
	return loaded, nil
}

// Validate rejeita opções incompatíveis antes que sejam usadas pelo bootstrap.
func (config Config) Validate() error {
	if config.Storage.Driver != "sqlite" {
		return fmt.Errorf("configuração inválida: storage.driver deve ser sqlite")
	}
	if strings.TrimSpace(config.Storage.Path) == "" {
		return fmt.Errorf("configuração inválida: storage.path é obrigatório")
	}
	if _, err := parseDuration(config.Storage.Retention); err != nil {
		return fmt.Errorf("configuração inválida: storage.retention: %w", err)
	}
	if _, err := parseDuration(config.Investigation.DefaultIncidentWindow); err != nil {
		return fmt.Errorf("configuração inválida: investigation.default_incident_window: %w", err)
	}
	if _, err := parseDuration(config.Investigation.DefaultBaselineWindow); err != nil {
		return fmt.Errorf("configuração inválida: investigation.default_baseline_window: %w", err)
	}
	if config.Investigation.TopSuspects < 1 {
		return fmt.Errorf("configuração inválida: investigation.top_suspects deve ser maior que zero")
	}
	if err := validateRankingWeights(config.Ranking.Weights); err != nil {
		return err
	}
	if config.Privacy.MaxAttributeLength < 1 {
		return fmt.Errorf("configuração inválida: privacy.max_attribute_length deve ser maior que zero")
	}
	if config.GitHub.Enabled && strings.TrimSpace(config.GitHub.Repository) == "" {
		return fmt.Errorf("configuração inválida: github.repository é obrigatório quando github.enabled é true")
	}
	githubURL, err := url.Parse(strings.TrimSpace(config.GitHub.APIURL))
	if err != nil || (githubURL.Scheme != "http" && githubURL.Scheme != "https") || githubURL.Host == "" {
		return fmt.Errorf("configuração inválida: github.api_url deve ser uma URL HTTP(S)")
	}
	return nil
}

// validateRankingWeights garante que cada contribuição e sua soma preservem o
// contrato normalizado do score, sem corrigir silenciosamente o YAML do usuário.
func validateRankingWeights(weights RankingWeights) error {
	values := []struct {
		name  string
		value float64
	}{
		{name: "error_rate_delta", value: weights.ErrorRateDelta},
		{name: "deployment_proximity", value: weights.DeploymentProximity},
		{name: "database_evidence", value: weights.DatabaseEvidence},
		{name: "graph_proximity", value: weights.GraphProximity},
		{name: "latency_delta", value: weights.LatencyDelta},
		{name: "log_correlation", value: weights.LogCorrelation},
	}

	total := 0.0
	for _, weight := range values {
		if math.IsNaN(weight.value) || math.IsInf(weight.value, 0) || weight.value < 0 || weight.value > 1 {
			return fmt.Errorf("configuração inválida: ranking.weights.%s deve estar entre 0 e 1", weight.name)
		}
		total += weight.value
	}
	const floatingPointTolerance = 1e-9
	if total <= floatingPointTolerance || total > 1+floatingPointTolerance {
		return fmt.Errorf("configuração inválida: soma de ranking.weights deve ser maior que 0 e no máximo 1")
	}
	return nil
}

func contextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("carregar configuração: contexto cancelado: %w", err)
	}
	return nil
}

func parseDuration(value string) (time.Duration, error) {
	if strings.HasSuffix(value, "d") {
		value = strings.TrimSuffix(value, "d") + "24h"
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, fmt.Errorf("a duração deve ser maior que zero")
	}
	return duration, nil
}
