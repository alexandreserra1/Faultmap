package normalizer

import (
	"context"
	"strings"
	"testing"
)

const logsPayload = `{
  "resourceLogs": [{
    "resource": {"attributes": [
      {"key": "service.name", "value": {"stringValue": "checkout-service"}},
      {"key": "service.version", "value": {"stringValue": "1.2.0"}}
    ]},
    "scopeLogs": [{
      "logRecords": [
        {
          "timeUnixNano": "1700000000000000000",
          "severityNumber": 17,
          "severityText": "ERROR",
          "body": {"stringValue": "falha ao cobrar cartao de maria@example.com token=abc123"},
          "traceId": "3f0a1b2c3d4e5f60718293a4b5c6d7e8",
          "spanId": "1b2c3d4e5f607182",
          "attributes": [{"key": "http.route", "value": {"stringValue": "/checkout"}}]
        },
        {
          "timeUnixNano": "1700000001000000000",
          "severityNumber": 9,
          "severityText": "INFO",
          "body": {"stringValue": "pedido recebido"},
          "traceId": "3f0a1b2c3d4e5f60718293a4b5c6d7e8",
          "spanId": "1b2c3d4e5f607183"
        }
      ]
    }]
  }]
}`

// TestParseOTLPLogsNuncaArmazenaOTextoDaMensagem fixa a decisão de privacidade
// que precede este código: o corpo do log é a parte mais útil e a mais
// perigosa, porque é onde aparecem e-mail, documento e token. O Faultmap guarda
// severidade, instante, correlação de trace e atributos permitidos, e nunca o
// texto — quem precisa lê-lo vai ao sistema de logs de origem.
func TestParseOTLPLogsNuncaArmazenaOTextoDaMensagem(t *testing.T) {
	t.Parallel()

	signals, err := ParseOTLPLogsJSON(context.Background(), strings.NewReader(logsPayload))
	if err != nil {
		t.Fatalf("ParseOTLPLogsJSON() erro = %v", err)
	}
	if len(signals) != 2 {
		t.Fatalf("sinais = %d, esperado 2", len(signals))
	}

	for _, signal := range signals {
		for chave, valor := range signal.Attributes {
			if strings.Contains(valor, "maria@example.com") || strings.Contains(valor, "abc123") ||
				strings.Contains(valor, "pedido recebido") {
				t.Fatalf("o texto da mensagem foi preservado em %q: %q", chave, valor)
			}
		}
	}
}

// TestParseOTLPLogsPreservaSeveridadeECorrelação garante que o que sustenta o
// diagnóstico continue disponível: sem o trace, um log de erro é apenas uma
// contagem; com ele, liga-se à requisição que falhou.
func TestParseOTLPLogsPreservaSeveridadeECorrelação(t *testing.T) {
	t.Parallel()

	signals, err := ParseOTLPLogsJSON(context.Background(), strings.NewReader(logsPayload))
	if err != nil {
		t.Fatalf("ParseOTLPLogsJSON() erro = %v", err)
	}

	primeiro := signals[0]
	if primeiro.Type != "log" {
		t.Fatalf("tipo = %q, esperado log", primeiro.Type)
	}
	if primeiro.ServiceName != "checkout-service" {
		t.Fatalf("serviço = %q", primeiro.ServiceName)
	}
	if primeiro.Severity != "error" {
		t.Fatalf("severidade = %q, esperado error", primeiro.Severity)
	}
	if primeiro.TraceID != "3f0a1b2c3d4e5f60718293a4b5c6d7e8" || primeiro.SpanID != "1b2c3d4e5f607182" {
		t.Fatalf("correlação perdida: trace=%q span=%q", primeiro.TraceID, primeiro.SpanID)
	}
	if primeiro.Attributes["http.route"] != "/checkout" {
		t.Fatalf("atributo legítimo do log foi descartado: %#v", primeiro.Attributes)
	}
	if primeiro.Attributes["service.version"] != "1.2.0" {
		t.Fatalf("atributo de recurso não foi propagado: %#v", primeiro.Attributes)
	}
	if signals[1].Severity != "info" {
		t.Fatalf("severidade do segundo = %q, esperado info", signals[1].Severity)
	}
}

// TestParseOTLPLogsÉIdempotentePorRegistro garante que reenviar o mesmo lote não
// duplique sinais: o identificador precisa ser derivado do conteúdo do registro,
// e não de um contador de leitura.
func TestParseOTLPLogsÉIdempotentePorRegistro(t *testing.T) {
	t.Parallel()

	primeira, err := ParseOTLPLogsJSON(context.Background(), strings.NewReader(logsPayload))
	if err != nil {
		t.Fatalf("primeira leitura: %v", err)
	}
	segunda, err := ParseOTLPLogsJSON(context.Background(), strings.NewReader(logsPayload))
	if err != nil {
		t.Fatalf("segunda leitura: %v", err)
	}
	for indice := range primeira {
		if primeira[indice].ID != segunda[indice].ID {
			t.Fatalf("identificador instável: %q vs %q", primeira[indice].ID, segunda[indice].ID)
		}
	}
	// Registros diferentes precisam de identificadores diferentes, senão a
	// deduplicação apagaria logs legítimos.
	if primeira[0].ID == primeira[1].ID {
		t.Fatal("registros distintos receberam o mesmo identificador")
	}
}

// TestParseOTLPLogsRejeitaEnvelopeInválido mantém o receiver honesto sobre
// entrada malformada, em vez de aceitar silenciosamente.
func TestParseOTLPLogsRejeitaEnvelopeInválido(t *testing.T) {
	t.Parallel()

	if _, err := ParseOTLPLogsJSON(context.Background(), strings.NewReader("{isso não é json")); err == nil {
		t.Fatal("erro = nil para envelope inválido")
	}
}
