package normalizer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
	collectortracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// TestParseOTLPJSONNormalizesResourceSpans verifica a transformação dos spans OTLP no contrato interno.
func TestParseOTLPJSONNormalizesResourceSpans(t *testing.T) {
	t.Parallel()

	const payload = `{
  "resourceSpans": [{
    "resource": {"attributes": [
      {"key": "service.name", "value": {"stringValue": "checkout-service"}},
      {"key": "service.version", "value": {"stringValue": "1.0.1"}},
      {"key": "deployment.environment.name", "value": {"stringValue": "staging"}}
    ]},
    "scopeSpans": [{"spans": [{
      "traceId": "4bf92f3577b34da6a3ce929d0e0e4736",
      "spanId": "00f067aa0ba902b7",
      "parentSpanId": "00f067aa0ba90000",
      "name": "POST /checkout",
      "kind": "SPAN_KIND_SERVER",
      "startTimeUnixNano": "1722110400000000000",
      "endTimeUnixNano": "1722110400125000000",
      "attributes": [
        {"key": "http.request.method", "value": {"stringValue": "POST"}},
        {"key": "http.response.status_code", "value": {"intValue": "500"}},
        {"key": "retry", "value": {"boolValue": true}}
      ],
      "status": {"code": "STATUS_CODE_ERROR", "message": "payment unavailable"}
    }]}]
  }]
}`

	signals, err := ParseOTLPJSON(context.Background(), strings.NewReader(payload))
	if err != nil {
		t.Fatalf("ParseOTLPJSON() error = %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("len(signals) = %d, want 1", len(signals))
	}

	wantTimestamp := time.Unix(0, 1722110400000000000).UTC()
	want := domain.Signal{
		ID:          "4bf92f3577b34da6a3ce929d0e0e4736:00f067aa0ba902b7",
		Type:        domain.SignalTypeSpan,
		ServiceName: "checkout-service",
		Timestamp:   wantTimestamp,
		TraceID:     "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:      "00f067aa0ba902b7",
		Severity:    "error",
		Attributes: map[string]string{
			"deployment.environment.name": "staging",
			"http.request.method":         "POST",
			"http.response.status_code":   "500",
			"retry":                       "true",
			"service.version":             "1.0.1",
			"span.kind":                   "SPAN_KIND_SERVER",
			"span.name":                   "POST /checkout",
			"span.parent_id":              "00f067aa0ba90000",
			"status.message":              "payment unavailable",
		},
		Measurements: map[string]float64{"duration_ms": 125},
	}

	assertSignalEqual(t, signals[0], want)
}

// TestParseOTLPJSONUsesDefaultsForOptionalFields verifica os defaults seguros de severidade e serviço.
func TestParseOTLPJSONUsesDefaultsForOptionalFields(t *testing.T) {
	t.Parallel()

	const payload = `{"resourceSpans":[{"scopeSpans":[{"spans":[{
  "traceId":"trace", "spanId":"span", "startTimeUnixNano":"1", "endTimeUnixNano":"1"
}]}]}]}`

	signals, err := ParseOTLPJSON(context.Background(), strings.NewReader(payload))
	if err != nil {
		t.Fatalf("ParseOTLPJSON() error = %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("len(signals) = %d, want 1", len(signals))
	}
	if signals[0].ServiceName != "unknown-service" {
		t.Fatalf("ServiceName = %q, want unknown-service", signals[0].ServiceName)
	}
	if signals[0].Severity != "info" {
		t.Fatalf("Severity = %q, want info", signals[0].Severity)
	}
	if signals[0].Measurements["duration_ms"] != 0 {
		t.Fatalf("duration_ms = %v, want 0", signals[0].Measurements["duration_ms"])
	}
}

// TestParseOTLPJSONRejectsMalformedPayload verifica que a entrada que não é JSON OTLP não produz sinais parciais.
func TestParseOTLPJSONRejectsMalformedPayload(t *testing.T) {
	t.Parallel()

	_, err := ParseOTLPJSON(context.Background(), strings.NewReader(`{"resourceSpans":`))
	if err == nil {
		t.Fatal("ParseOTLPJSON() error = nil, want malformed payload error")
	}
}

// TestParseOTLPJSONHonorsCancelledContext evita consumir telemetria após o cancelamento do caso de uso.
func TestParseOTLPJSONHonorsCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ParseOTLPJSON(ctx, strings.NewReader(`{"resourceSpans":[]}`))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ParseOTLPJSON() error = %v, want context.Canceled", err)
	}
}

// TestParseOTLPTracesProtobufUsaAMesmaNormalizacao garante paridade entre o
// transporte protobuf oficial e o contrato interno já usado por arquivos JSON.
func TestParseOTLPTracesProtobufUsaAMesmaNormalizacao(t *testing.T) {
	t.Parallel()

	payload, err := proto.Marshal(&collectortracev1.ExportTraceServiceRequest{
		ResourceSpans: []*tracev1.ResourceSpans{{
			Resource: &resourcev1.Resource{Attributes: []*commonv1.KeyValue{
				{Key: "service.name", Value: stringAnyValue("payment-service")},
				{Key: "service.version", Value: stringAnyValue("2.1.0")},
			}},
			ScopeSpans: []*tracev1.ScopeSpans{{Spans: []*tracev1.Span{{
				TraceId:           []byte{0, 17, 34, 51, 68, 85, 102, 119, 136, 153, 170, 187, 204, 221, 238, 255},
				SpanId:            []byte{0, 17, 34, 51, 68, 85, 102, 119},
				ParentSpanId:      []byte{136, 153, 170, 187, 204, 221, 238, 255},
				Name:              "POST payment-service",
				Kind:              tracev1.Span_SPAN_KIND_CLIENT,
				StartTimeUnixNano: 1_722_110_400_000_000_000,
				EndTimeUnixNano:   1_722_110_400_125_000_000,
				Attributes: []*commonv1.KeyValue{
					{Key: "http.request.method", Value: stringAnyValue("POST")},
					{Key: "http.response.status_code", Value: intAnyValue(503)},
				},
				Status: &tracev1.Status{Code: tracev1.Status_STATUS_CODE_ERROR, Message: "indisponível"},
			}}},
			}}},
	})
	if err != nil {
		t.Fatalf("proto.Marshal() error = %v", err)
	}

	signals, err := ParseOTLPTraces(context.Background(), strings.NewReader(string(payload)), OTLPEncodingProtobuf)
	if err != nil {
		t.Fatalf("ParseOTLPTraces() error = %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("len(signals) = %d, want 1", len(signals))
	}

	want := domain.Signal{
		ID:          "00112233445566778899aabbccddeeff:0011223344556677",
		Type:        domain.SignalTypeSpan,
		ServiceName: "payment-service",
		Timestamp:   time.Unix(0, 1_722_110_400_000_000_000).UTC(),
		TraceID:     "00112233445566778899aabbccddeeff",
		SpanID:      "0011223344556677",
		Severity:    "error",
		Attributes: map[string]string{
			"http.request.method":       "POST",
			"http.response.status_code": "503",
			"service.version":           "2.1.0",
			"span.kind":                 "SPAN_KIND_CLIENT",
			"span.name":                 "POST payment-service",
			"span.parent_id":            "8899aabbccddeeff",
			"status.message":            "indisponível",
		},
		Measurements: map[string]float64{"duration_ms": 125},
	}
	assertSignalEqual(t, signals[0], want)
}

// TestParseOTLPTracesRejeitaCodificacaoDesconhecida impede que o chamador
// interprete um payload com formato implícito ou ambíguo.
func TestParseOTLPTracesRejeitaCodificacaoDesconhecida(t *testing.T) {
	t.Parallel()

	_, err := ParseOTLPTraces(context.Background(), strings.NewReader("payload"), OTLPEncoding("yaml"))
	if err == nil {
		t.Fatal("ParseOTLPTraces() error = nil, want unsupported encoding error")
	}
	if !errors.Is(err, ErrInvalidOTLP) {
		t.Fatalf("ParseOTLPTraces() error = %v, want ErrInvalidOTLP", err)
	}
}

// TestParseOTLPTracesClassificaPayloadMalformado permite ao transporte
// responder erro do cliente sem confundir a falha com indisponibilidade do banco.
func TestParseOTLPTracesClassificaPayloadMalformado(t *testing.T) {
	t.Parallel()

	_, err := ParseOTLPTraces(context.Background(), strings.NewReader(`{"resourceSpans":`), OTLPEncodingJSON)
	if !errors.Is(err, ErrInvalidOTLP) {
		t.Fatalf("ParseOTLPTraces() error = %v, want ErrInvalidOTLP", err)
	}
}

// TestParseOTLPTracesJSONAceitaEnumsNumericosOficiais mantém compatibilidade
// com o mapeamento JSON do OTLP sem quebrar as fixtures legadas com nomes.
func TestParseOTLPTracesJSONAceitaEnumsNumericosOficiais(t *testing.T) {
	t.Parallel()

	const payload = `{"resourceSpans":[{"scopeSpans":[{"spans":[{
  "traceId":"00112233445566778899aabbccddeeff",
  "spanId":"0011223344556677",
  "kind":3,
  "startTimeUnixNano":"1720000000000000000",
  "endTimeUnixNano":"1720000000000000001",
  "status":{"code":2}
}]}]}]}`
	signals, err := ParseOTLPTraces(context.Background(), strings.NewReader(payload), OTLPEncodingJSON)
	if err != nil {
		t.Fatalf("ParseOTLPTraces() error = %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("len(signals) = %d, want 1", len(signals))
	}
	if got := signals[0].Attributes["span.kind"]; got != "SPAN_KIND_CLIENT" {
		t.Fatalf("span.kind = %q, want SPAN_KIND_CLIENT", got)
	}
	if signals[0].Severity != "error" {
		t.Fatalf("Severity = %q, want error", signals[0].Severity)
	}
}

func stringAnyValue(value string) *commonv1.AnyValue {
	return &commonv1.AnyValue{Value: &commonv1.AnyValue_StringValue{StringValue: value}}
}

func intAnyValue(value int64) *commonv1.AnyValue {
	return &commonv1.AnyValue{Value: &commonv1.AnyValue_IntValue{IntValue: value}}
}

func assertSignalEqual(t *testing.T, got, want domain.Signal) {
	t.Helper()
	if got.ID != want.ID || got.Type != want.Type || got.ServiceName != want.ServiceName || !got.Timestamp.Equal(want.Timestamp) || got.TraceID != want.TraceID || got.SpanID != want.SpanID || got.Severity != want.Severity {
		t.Fatalf("signal = %#v, want %#v", got, want)
	}
	if len(got.Attributes) != len(want.Attributes) {
		t.Fatalf("Attributes = %#v, want %#v", got.Attributes, want.Attributes)
	}
	for key, value := range want.Attributes {
		if got.Attributes[key] != value {
			t.Fatalf("Attributes[%q] = %q, want %q", key, got.Attributes[key], value)
		}
	}
	if len(got.Measurements) != len(want.Measurements) {
		t.Fatalf("Measurements = %#v, want %#v", got.Measurements, want.Measurements)
	}
	for key, value := range want.Measurements {
		if got.Measurements[key] != value {
			t.Fatalf("Measurements[%q] = %v, want %v", key, got.Measurements[key], value)
		}
	}
}
