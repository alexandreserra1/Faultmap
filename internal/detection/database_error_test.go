package detection

import (
	"fmt"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// databaseErrorSignals produz operações de banco cujas falhas não são timeout,
// para separar o território deste detector do de database_timeout.
func databaseErrorSignals(prefix string, count, failureCount int, failureType, failureMessage string) []domain.Signal {
	signals := make([]domain.Signal, 0, count)
	for index := range count {
		attributes := map[string]string{
			"db.system.name":    "postgresql",
			"db.operation.name": "SELECT",
		}
		severity := "INFO"
		if index < failureCount {
			attributes["error.type"] = failureType
			attributes["status.message"] = failureMessage
			severity = "ERROR"
		}
		signals = append(signals, domain.Signal{
			ID:           fmt.Sprintf("%s-db-%d", prefix, index),
			ServiceName:  "checkout-service",
			Severity:     severity,
			Attributes:   attributes,
			Measurements: map[string]float64{"duration_ms": 20},
		})
	}
	return signals
}

func TestDatabaseErrorDetectaCrescimentoDeFalhasDeConexão(t *testing.T) {
	t.Parallel()

	finding, found := DetectDatabaseError(Input{
		ServiceName: "checkout-service",
		Baseline:    databaseErrorSignals("baseline", 40, 0, "", ""),
		Incident:    databaseErrorSignals("incident", 40, 20, "ConnectionRefused", "connection refused by server"),
	})
	if !found {
		t.Fatal("esperava hipótese para crescimento de falhas de banco")
	}
	if finding.Rule != RuleDatabaseError {
		t.Fatalf("regra = %q", finding.Rule)
	}
	if finding.Confidence != ConfidenceHigh {
		t.Fatalf("confiança = %q", finding.Confidence)
	}
	if len(finding.Evidence) != 1 {
		t.Fatalf("esperava uma evidência, recebeu %d", len(finding.Evidence))
	}
	if !contains([]string{finding.Evidence[0].Summary}, "PostgreSQL") {
		t.Fatalf("evidência deveria citar o sistema de banco: %q", finding.Evidence[0].Summary)
	}
	if !contains([]string{finding.Evidence[0].Summary}, "20 de 40") {
		t.Fatalf("evidência deveria citar os números comparados: %q", finding.Evidence[0].Summary)
	}
	if len(finding.Evidence[0].SignalIDs) != 20 {
		t.Fatalf("esperava 20 sinais referenciados, recebeu %d", len(finding.Evidence[0].SignalIDs))
	}
	if !contains(finding.Limitations, "não comprova causalidade") {
		t.Fatalf("finding deve declarar ausência de causalidade: %#v", finding.Limitations)
	}
}

// A falha crônica e constante é o estado normal daquele sistema: sem mudança
// entre as janelas, não há o que reportar.
func TestDatabaseErrorSilenciaComFalhaCrônicaConstante(t *testing.T) {
	t.Parallel()

	if _, found := DetectDatabaseError(Input{
		ServiceName: "checkout-service",
		Baseline:    databaseErrorSignals("baseline", 40, 10, "QueryFailed", "syntax error"),
		Incident:    databaseErrorSignals("incident", 40, 10, "QueryFailed", "syntax error"),
	}); found {
		t.Fatal("detector acusou falha crônica constante como regressão")
	}
}

// Uma falha a mais em vinte é variação de amostragem, não regressão.
func TestDatabaseErrorIgnoraVariaçãoDeAmostragem(t *testing.T) {
	t.Parallel()

	if _, found := DetectDatabaseError(Input{
		ServiceName: "checkout-service",
		Baseline:    databaseErrorSignals("baseline", 20, 5, "QueryFailed", "invalid response"),
		Incident:    databaseErrorSignals("incident", 20, 6, "QueryFailed", "invalid response"),
	}); found {
		t.Fatal("detector acusou ruído de amostragem como regressão")
	}
}

// Timeouts já pertencem a database_timeout; contá-los aqui reportaria o mesmo
// sinal duas vezes com regras diferentes.
func TestDatabaseErrorNãoContabilizaTimeouts(t *testing.T) {
	t.Parallel()

	if _, found := DetectDatabaseError(Input{
		ServiceName: "checkout-service",
		Baseline:    databaseTimeoutSignals("baseline", 40, 0, 50),
		Incident:    databaseTimeoutSignals("incident", 40, 20, 1_500),
	}); found {
		t.Fatal("detector de erro de banco reportou timeouts, que já têm regra própria")
	}
}

func TestDatabaseErrorSilenciaSemOperaçõesDeBanco(t *testing.T) {
	t.Parallel()

	if _, found := DetectDatabaseError(Input{
		ServiceName: "checkout-service",
		Baseline:    httpSignals("baseline", 20, 0, 100),
		Incident:    httpSignals("incident", 20, 10, 100),
	}); found {
		t.Fatal("detector reportou sem nenhuma operação de banco observada")
	}
}

// TestDatabaseErrorIgnoraCancelamentoDoCliente cobre um caso observado na demo:
// quando o serviço de cima desiste da chamada, as operações de banco de baixo
// são canceladas e apareciam como falha do banco. O banco não falhou — quem
// abandonou foi o chamador —, e contar isso contra o serviço de baixo culpa a
// vítima da desistência, que é o padrão que este produto existe para evitar.
//
// Cancelamento causado por esgotamento de tempo continua sendo tratado pelo
// database_timeout, que o reconhece pelo texto.
func TestDatabaseErrorIgnoraCancelamentoDoCliente(t *testing.T) {
	t.Parallel()

	baseline := bancoComFalhas(20, 0, "")
	incidente := bancoComFalhas(20, 20, "canceled")

	if finding, found := DetectDatabaseError(Input{
		ServiceName: "payment-service", Baseline: baseline, Incident: incidente,
	}); found {
		t.Fatalf("cancelamento do cliente foi contado como falha de banco: %s", finding.Evidence[0].Summary)
	}
}

// TestDatabaseErrorIgnoraCancelamentoDeInstrumentaçãoReal usa o tipo de exceção
// que a biblioteca oficial do PostgreSQL emite ao ter a consulta cancelada.
func TestDatabaseErrorIgnoraCancelamentoDeInstrumentaçãoReal(t *testing.T) {
	t.Parallel()

	if finding, found := DetectDatabaseError(Input{
		ServiceName: "payment-service",
		Baseline:    bancoComFalhas(20, 0, ""),
		Incident:    bancoComFalhas(20, 20, "psycopg2.errors.QueryCanceled"),
	}); found {
		t.Fatalf("cancelamento de instrumentação real foi contado como falha: %s", finding.Evidence[0].Summary)
	}
}

// TestDatabaseErrorAcusaFalhaVerdadeira garante que a exclusão do cancelamento
// não silenciou o que o detector existe para encontrar.
func TestDatabaseErrorAcusaFalhaVerdadeira(t *testing.T) {
	t.Parallel()

	if _, found := DetectDatabaseError(Input{
		ServiceName: "payment-service",
		Baseline:    bancoComFalhas(20, 0, ""),
		Incident:    bancoComFalhas(20, 20, "database_error"),
	}); !found {
		t.Fatal("detector silenciou falha real de banco")
	}
}

// bancoComFalhas monta spans de banco em que as primeiras operações falharam
// com o tipo informado.
func bancoComFalhas(total, falhas int, tipoDeFalha string) []domain.Signal {
	signals := make([]domain.Signal, 0, total)
	for index := 0; index < total; index++ {
		attributes := map[string]string{
			"db.system.name":    "postgresql",
			"db.operation.name": "INSERT",
		}
		severity := "info"
		if index < falhas {
			attributes["error.type"] = tipoDeFalha
			severity = "error"
		}
		signals = append(signals, domain.Signal{
			ID:           fmt.Sprintf("db-%03d", index),
			ServiceName:  "payment-service",
			Severity:     severity,
			Attributes:   attributes,
			Measurements: map[string]float64{"duration_ms": 5},
		})
	}
	return signals
}
