package normalizer

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestParseOTLPJSONNormalizesResourceSpans verifica a transformação dos spans OTLP no contrato interno.
func TestParseOTLPJSONNormalizesResourceSpans(t *testing.T) {
	t.Parallel()

	const payload = `{
  "resourceSpans": [{
    "resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "checkout-service"}}]},
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
			"http.request.method":       "POST",
			"http.response.status_code": "500",
			"retry":                     "true",
			"span.kind":                 "SPAN_KIND_SERVER",
			"span.name":                 "POST /checkout",
			"span.parent_id":            "00f067aa0ba90000",
			"status.message":            "payment unavailable",
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
