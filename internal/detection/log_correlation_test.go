package detection

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestLogCorrelationLigaErrosDeLogAoTraceQueFalhou cobre o que torna o log útil
// ao diagnóstico: sozinho ele é uma contagem, mas ligado ao trace da requisição
// que falhou ele mostra que os dois sinais descrevem o mesmo evento.
func TestLogCorrelationLigaErrosDeLogAoTraceQueFalhou(t *testing.T) {
	t.Parallel()

	baseline := append(
		logsDeErro("base", 8, 0),
		requisicoes("base", 8, 0)...,
	)
	incidente := append(
		logsDeErro("inc", 8, 8),
		requisicoes("inc", 8, 8)...,
	)

	finding, found := DetectLogCorrelation(Input{
		ServiceName: "checkout-service", Baseline: baseline, Incident: incidente,
	})
	if !found {
		t.Fatal("detector silenciou logs de erro correlacionados a requisições que falharam")
	}
	if finding.Rule != RuleLogCorrelation {
		t.Fatalf("regra = %q", finding.Rule)
	}
	if len(finding.Evidence) == 0 || len(finding.Evidence[0].SignalIDs) == 0 {
		t.Fatal("finding sem proveniência")
	}
	// A evidência descreve contagem e correlação; jamais o texto da mensagem,
	// que sequer chega até aqui.
	if !strings.Contains(finding.Evidence[0].Summary, "trace") {
		t.Fatalf("a evidência não declara a correlação: %s", finding.Evidence[0].Summary)
	}
}

// TestLogCorrelationIgnoraRuídoConstante mantém a regra comparativa: um serviço
// que sempre registra erros descreve o próprio sistema, não o incidente.
func TestLogCorrelationIgnoraRuídoConstante(t *testing.T) {
	t.Parallel()

	baseline := append(logsDeErro("base", 8, 8), requisicoes("base", 8, 8)...)
	incidente := append(logsDeErro("inc", 8, 8), requisicoes("inc", 8, 8)...)

	if finding, found := DetectLogCorrelation(Input{
		ServiceName: "checkout-service", Baseline: baseline, Incident: incidente,
	}); found {
		t.Fatalf("detector acusou ruído constante: %s", finding.Evidence[0].Summary)
	}
}

// TestLogCorrelationExigeCorrelaçãoComTrace impede que o detector vire um mero
// contador de logs: sem ligação com uma requisição que falhou, o aumento pode
// vir de rotina de fundo, migração ou tarefa agendada.
func TestLogCorrelationExigeCorrelaçãoComTrace(t *testing.T) {
	t.Parallel()

	// Logs de erro crescem, mas nenhuma requisição falhou nos mesmos traces.
	baseline := append(logsDeErro("base", 8, 0), requisicoes("base", 8, 0)...)
	incidente := append(logsDeErro("inc", 8, 8), requisicoes("inc", 8, 0)...)

	if finding, found := DetectLogCorrelation(Input{
		ServiceName: "checkout-service", Baseline: baseline, Incident: incidente,
	}); found {
		t.Fatalf("detector concluiu sem correlação com requisição que falhou: %s", finding.Evidence[0].Summary)
	}
}

// TestLogCorrelationSemLogsNãoConclui garante silêncio quando não há logs, que é
// o estado de qualquer instalação que ainda não os exporta.
func TestLogCorrelationSemLogsNãoConclui(t *testing.T) {
	t.Parallel()

	if _, found := DetectLogCorrelation(Input{
		ServiceName: "checkout-service",
		Baseline:    requisicoes("base", 8, 0),
		Incident:    requisicoes("inc", 8, 8),
	}); found {
		t.Fatal("detector concluiu sem nenhum log")
	}
}

// logsDeErro monta registros de log, dos quais os primeiros são de erro.
func logsDeErro(prefixo string, total, comErro int) []domain.Signal {
	signals := make([]domain.Signal, 0, total)
	for indice := 0; indice < total; indice++ {
		severidade := "info"
		if indice < comErro {
			severidade = "error"
		}
		signals = append(signals, domain.Signal{
			ID:          fmt.Sprintf("log-%s-%02d", prefixo, indice),
			Type:        domain.SignalTypeLog,
			ServiceName: "checkout-service",
			Timestamp:   time.Date(2026, time.August, 13, 10, 0, indice, 0, time.UTC),
			TraceID:     fmt.Sprintf("%s-trace-%02d", prefixo, indice),
			Severity:    severidade,
			Attributes:  map[string]string{"http.route": "/checkout"},
		})
	}
	return signals
}

// requisicoes monta spans HTTP, dos quais os primeiros falharam.
func requisicoes(prefixo string, total, comFalha int) []domain.Signal {
	signals := make([]domain.Signal, 0, total)
	for indice := 0; indice < total; indice++ {
		status := "200"
		if indice < comFalha {
			status = "500"
		}
		signals = append(signals, domain.Signal{
			ID:          fmt.Sprintf("span-%s-%02d", prefixo, indice),
			Type:        domain.SignalTypeSpan,
			ServiceName: "checkout-service",
			Timestamp:   time.Date(2026, time.August, 13, 10, 0, indice, 0, time.UTC),
			TraceID:     fmt.Sprintf("%s-trace-%02d", prefixo, indice),
			Attributes: map[string]string{
				"http.response.status_code": status,
				"span.kind":                 "SPAN_KIND_SERVER",
			},
			Measurements: map[string]float64{"duration_ms": 20},
		})
	}
	return signals
}
