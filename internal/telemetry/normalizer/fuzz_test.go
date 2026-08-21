package normalizer

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// O normalizador é a primeira coisa que toca bytes vindos da rede: qualquer
// Collector mal configurado, exportador de outra versão ou cliente hostil chega
// aqui antes de qualquer validação de negócio. Testes escritos à mão só cobrem
// os payloads que imaginamos; estes cobrem os que não imaginamos.
//
// Todos exigem a mesma coisa do código: falhar como erro classificado, nunca
// entrar em pânico e nunca produzir um sinal que viole os invariantes que o
// resto do produto assume.

// FuzzParseOTLPTracesJSON exercita o mapeamento JSON com entrada arbitrária.
func FuzzParseOTLPTracesJSON(f *testing.F) {
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"resourceSpans":[]}`))
	f.Add([]byte(`{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"s"}}]},"scopeSpans":[{"spans":[{"traceId":"aa","spanId":"bb","startTimeUnixNano":"1","endTimeUnixNano":"2","name":"GET /x"}]}]}]}`))
	f.Add([]byte(`{"resourceSpans":[{"scopeSpans":[{"spans":[{"attributes":[{"key":"http.response.status_code","value":{"intValue":"500"}}]}]}]}]}`))

	f.Fuzz(func(t *testing.T, payload []byte) {
		signals, err := ParseOTLPTraces(context.Background(), bytes.NewReader(payload), OTLPEncodingJSON)
		exigirErroClassificado(t, err)
		exigirSinaisCoerentes(t, signals, err)
	})
}

// FuzzParseOTLPTracesProtobuf cobre o caminho binário, onde um byte trocado
// muda o significado da mensagem inteira em vez de produzir um erro de sintaxe.
func FuzzParseOTLPTracesProtobuf(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x0a, 0x00})
	f.Add([]byte{0x0a, 0x02, 0x0a, 0x00})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})

	f.Fuzz(func(t *testing.T, payload []byte) {
		signals, err := ParseOTLPTraces(context.Background(), bytes.NewReader(payload), OTLPEncodingProtobuf)
		exigirErroClassificado(t, err)
		exigirSinaisCoerentes(t, signals, err)
	})
}

// FuzzParseOTLPLogsJSONNãoDevolveOCorpo protege a promessa central do ADR 0011
// com entrada arbitrária, e não apenas com os payloads que escrevemos à mão.
//
// O corpo da mensagem é usado em um único ponto — compor o identificador do
// registro — e descartado em seguida. Um sinal que carregue o texto de volta,
// em qualquer campo, é vazamento de dado pessoal.
func FuzzParseOTLPLogsJSONNãoDevolveOCorpo(f *testing.F) {
	f.Add([]byte(`{"resourceLogs":[]}`), "segredo")
	f.Add([]byte(`{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"s"}}]},"scopeLogs":[{"logRecords":[{"timeUnixNano":"1","severityText":"ERROR","body":{"stringValue":"CORPO"},"traceId":"aa","spanId":"bb"}]}]}]}`), "CORPO")

	f.Fuzz(func(t *testing.T, payload []byte, corpo string) {
		// Um corpo curto apareceria por coincidência dentro de um identificador
		// hexadecimal, e a verificação acusaria vazamento onde não há.
		if len(corpo) < 8 {
			return
		}
		signals, err := ParseOTLPLogs(context.Background(), bytes.NewReader(payload), OTLPEncodingJSON)
		exigirErroClassificado(t, err)
		exigirSinaisCoerentes(t, signals, err)

		if !bytes.Contains(payload, []byte(corpo)) {
			return
		}
		for _, signal := range signals {
			for chave, valor := range signal.Attributes {
				if strings.Contains(valor, corpo) {
					t.Fatalf("o corpo do log voltou no atributo %q: %q", chave, valor)
				}
			}
			if strings.Contains(signal.Severity, corpo) || strings.Contains(signal.ID, corpo) {
				t.Fatalf("o corpo do log voltou em campo do sinal: %+v", signal)
			}
		}
	})
}

// exigirErroClassificado garante que toda recusa seja identificável como
// entrada inválida. Um erro fora dessa classe faria o receiver responder 500 em
// vez de 400, e um Collector trataria payload malformado como falha nossa,
// tentando de novo indefinidamente.
func exigirErroClassificado(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if errors.Is(err, ErrInvalidOTLP) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	t.Fatalf("erro não classificado como payload inválido: %v", err)
}

// exigirSinaisCoerentes verifica o que o resto do produto assume de todo sinal
// aceito: nada é devolvido junto com erro, e um sinal sem identificador ou sem
// serviço quebraria a chave primária e o agrupamento do ranking.
func exigirSinaisCoerentes(t *testing.T, signals []domain.Signal, err error) {
	t.Helper()
	if err != nil {
		if len(signals) != 0 {
			t.Fatalf("%d sinais devolvidos junto com erro %v", len(signals), err)
		}
		return
	}
	for _, signal := range signals {
		if strings.TrimSpace(signal.ID) == "" {
			t.Fatalf("sinal sem identificador: %+v", signal)
		}
		if signal.Type != domain.SignalTypeSpan && signal.Type != domain.SignalTypeLog {
			t.Fatalf("sinal com tipo inesperado %q", signal.Type)
		}
		for chave := range signal.Attributes {
			if strings.TrimSpace(chave) == "" {
				t.Fatalf("atributo com chave vazia em %+v", signal)
			}
		}
	}
}
