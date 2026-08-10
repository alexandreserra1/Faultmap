package detection

import (
	"fmt"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// versionedHTTPSignals representa a fatia de tráfego atendida por uma versão
// durante a janela de incidente, como acontece em rollout parcial ou canário.
func versionedHTTPSignals(version string, count, errorCount int, durationMS float64) []domain.Signal {
	signals := make([]domain.Signal, 0, count)
	for index := range count {
		statusCode := "200"
		severity := "INFO"
		if index < errorCount {
			statusCode = "500"
			severity = "ERROR"
		}
		signals = append(signals, domain.Signal{
			ID:          fmt.Sprintf("%s-http-%d", version, index),
			ServiceName: "checkout-service",
			Severity:    severity,
			Attributes: map[string]string{
				"http.response.status_code": statusCode,
				"service.version":           version,
				"span.kind":                 "SPAN_KIND_SERVER",
			},
			Measurements: map[string]float64{"duration_ms": durationMS},
		})
	}
	return signals
}

func TestVersionRegressionComparaDuasVersõesNaMesmaJanela(t *testing.T) {
	t.Parallel()

	finding, found := DetectVersionRegression(Input{
		ServiceName: "checkout-service",
		Incident: append(
			versionedHTTPSignals("v1", 40, 0, 100),
			versionedHTTPSignals("v2", 40, 20, 100)...,
		),
	})
	if !found {
		t.Fatal("esperava hipótese comparando as duas versões")
	}
	if finding.Rule != RuleVersionRegression {
		t.Fatalf("regra = %q", finding.Rule)
	}
	if finding.Confidence != ConfidenceHigh {
		t.Fatalf("confiança = %q", finding.Confidence)
	}
	summary := finding.Evidence[0].Summary
	if !contains([]string{summary}, "v2") || !contains([]string{summary}, "v1") {
		t.Fatalf("evidência deveria citar as duas versões: %q", summary)
	}
	if !contains(finding.Limitations, "não comprova causalidade") {
		t.Fatalf("finding deve declarar ausência de causalidade: %#v", finding.Limitations)
	}
}

func TestVersionRegressionDetectaLatênciaPiorEmUmaVersão(t *testing.T) {
	t.Parallel()

	finding, found := DetectVersionRegression(Input{
		ServiceName: "checkout-service",
		Incident: append(
			versionedHTTPSignals("v1", 20, 0, 100),
			versionedHTTPSignals("v2", 20, 0, 900)...,
		),
	})
	if !found {
		t.Fatal("esperava hipótese para latência p95 pior em uma das versões")
	}
	if !contains([]string{finding.Evidence[0].Summary}, "p95") {
		t.Fatalf("evidência deveria descrever a latência comparada: %q", finding.Evidence[0].Summary)
	}
}

// Com uma versão só não existe comparação possível.
func TestVersionRegressionSilenciaComUmaÚnicaVersão(t *testing.T) {
	t.Parallel()

	if _, found := DetectVersionRegression(Input{
		ServiceName: "checkout-service",
		Incident:    versionedHTTPSignals("v1", 40, 20, 900),
	}); found {
		t.Fatal("detector reportou regressão com uma única versão observada")
	}
}

// Um canário com pouquíssimas requisições não sustenta conclusão nenhuma.
func TestVersionRegressionSilenciaComVolumeInsuficienteNoCanário(t *testing.T) {
	t.Parallel()

	if _, found := DetectVersionRegression(Input{
		ServiceName: "checkout-service",
		Incident: append(
			versionedHTTPSignals("v1", 40, 0, 100),
			versionedHTTPSignals("v2", 3, 3, 900)...,
		),
	}); found {
		t.Fatal("detector concluiu a partir de um canário com três requisições")
	}
}

func TestVersionRegressionIgnoraDiferençaDentroDoRuído(t *testing.T) {
	t.Parallel()

	if _, found := DetectVersionRegression(Input{
		ServiceName: "checkout-service",
		Incident: append(
			versionedHTTPSignals("v1", 20, 5, 100),
			versionedHTTPSignals("v2", 20, 6, 104)...,
		),
	}); found {
		t.Fatal("detector acusou ruído de amostragem como regressão de versão")
	}
}

// A ordem em que os sinais chegam não pode mudar o resultado: mapas de versão
// iteram em ordem aleatória e isso não pode vazar para a saída.
func TestVersionRegressionÉDeterminísticoIndependenteDaOrdem(t *testing.T) {
	t.Parallel()

	direct, _ := DetectVersionRegression(Input{
		ServiceName: "checkout-service",
		Incident: append(
			versionedHTTPSignals("v1", 30, 0, 100),
			append(versionedHTTPSignals("v2", 30, 15, 100), versionedHTTPSignals("v3", 30, 1, 100)...)...,
		),
	})
	for range 20 {
		reversed, _ := DetectVersionRegression(Input{
			ServiceName: "checkout-service",
			Incident: append(
				versionedHTTPSignals("v3", 30, 1, 100),
				append(versionedHTTPSignals("v2", 30, 15, 100), versionedHTTPSignals("v1", 30, 0, 100)...)...,
			),
		})
		if reversed.Evidence[0].Summary != direct.Evidence[0].Summary || reversed.Score != direct.Score {
			t.Fatalf("saída dependeu da ordem dos sinais: %q vs %q", direct.Evidence[0].Summary, reversed.Evidence[0].Summary)
		}
	}
}
