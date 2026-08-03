// Package normalizer converte formatos externos de telemetria no contrato interno do Faultmap.
package normalizer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

const unknownServiceName = "unknown-service"

// ParseOTLPJSON transforma resourceSpans do formato OTLP JSON em sinais internos de span.
// A função não preserva o payload bruto e interrompe a leitura quando o contexto é cancelado.
func ParseOTLPJSON(ctx context.Context, reader io.Reader) ([]domain.Signal, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	var request exportTraceServiceRequest
	decoder := json.NewDecoder(contextReader{ctx: ctx, reader: reader})
	if err := decoder.Decode(&request); err != nil {
		return nil, fmt.Errorf("interpretar OTLP JSON: %w", err)
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	signals := make([]domain.Signal, 0)
	for resourceIndex, resourceSpan := range request.ResourceSpans {
		serviceName := serviceName(resourceSpan.Resource.Attributes)
		for _, scopeSpan := range resourceSpan.ScopeSpans {
			for spanIndex, span := range scopeSpan.Spans {
				if err := contextError(ctx); err != nil {
					return nil, err
				}
				signal, err := normalizeSpan(serviceName, span)
				if err != nil {
					return nil, fmt.Errorf("normalizar resourceSpans[%d].scopeSpans[].spans[%d]: %w", resourceIndex, spanIndex, err)
				}
				signals = append(signals, signal)
			}
		}
	}
	return signals, nil
}

type exportTraceServiceRequest struct {
	ResourceSpans []resourceSpan `json:"resourceSpans"`
}

type resourceSpan struct {
	Resource   resource    `json:"resource"`
	ScopeSpans []scopeSpan `json:"scopeSpans"`
}

type resource struct {
	Attributes []keyValue `json:"attributes"`
}

type scopeSpan struct {
	Spans []span `json:"spans"`
}

type span struct {
	TraceID           string          `json:"traceId"`
	SpanID            string          `json:"spanId"`
	ParentSpanID      string          `json:"parentSpanId"`
	Name              string          `json:"name"`
	Kind              string          `json:"kind"`
	StartTimeUnixNano json.RawMessage `json:"startTimeUnixNano"`
	EndTimeUnixNano   json.RawMessage `json:"endTimeUnixNano"`
	Attributes        []keyValue      `json:"attributes"`
	Status            status          `json:"status"`
}

type status struct {
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
}

type keyValue struct {
	Key   string   `json:"key"`
	Value anyValue `json:"value"`
}

type anyValue struct {
	StringValue *string         `json:"stringValue"`
	BoolValue   *bool           `json:"boolValue"`
	IntValue    json.RawMessage `json:"intValue"`
	DoubleValue *float64        `json:"doubleValue"`
	BytesValue  *string         `json:"bytesValue"`
	ArrayValue  json.RawMessage `json:"arrayValue"`
	KVListValue json.RawMessage `json:"kvlistValue"`
}

func normalizeSpan(serviceName string, source span) (domain.Signal, error) {
	if strings.TrimSpace(source.TraceID) == "" {
		return domain.Signal{}, fmt.Errorf("traceId é obrigatório")
	}
	if strings.TrimSpace(source.SpanID) == "" {
		return domain.Signal{}, fmt.Errorf("spanId é obrigatório")
	}

	startedAt, err := parseUnixNano(source.StartTimeUnixNano)
	if err != nil {
		return domain.Signal{}, fmt.Errorf("startTimeUnixNano: %w", err)
	}
	endedAt, err := parseUnixNano(source.EndTimeUnixNano)
	if err != nil {
		return domain.Signal{}, fmt.Errorf("endTimeUnixNano: %w", err)
	}
	if endedAt.Before(startedAt) {
		return domain.Signal{}, fmt.Errorf("endTimeUnixNano não pode ser anterior a startTimeUnixNano")
	}

	attributes := normalizeAttributes(source.Attributes)
	if source.Name != "" {
		attributes["span.name"] = source.Name
	}
	if source.Kind != "" {
		attributes["span.kind"] = source.Kind
	}
	if source.ParentSpanID != "" {
		attributes["span.parent_id"] = source.ParentSpanID
	}
	if source.Status.Message != "" {
		attributes["status.message"] = source.Status.Message
	}

	return domain.Signal{
		ID:          source.TraceID + ":" + source.SpanID,
		Type:        domain.SignalTypeSpan,
		ServiceName: serviceName,
		Timestamp:   startedAt,
		TraceID:     source.TraceID,
		SpanID:      source.SpanID,
		Severity:    severity(source.Status.Code),
		Attributes:  attributes,
		Measurements: map[string]float64{
			"duration_ms": float64(endedAt.Sub(startedAt)) / float64(time.Millisecond),
		},
	}, nil
}

func serviceName(attributes []keyValue) string {
	for _, attribute := range attributes {
		if attribute.Key != "service.name" {
			continue
		}
		if value, ok := attribute.Value.stringValue(); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return unknownServiceName
}

func normalizeAttributes(source []keyValue) map[string]string {
	attributes := make(map[string]string, len(source)+3)
	for _, attribute := range source {
		if strings.TrimSpace(attribute.Key) == "" {
			continue
		}
		if value, ok := attribute.Value.stringValue(); ok {
			attributes[attribute.Key] = value
		}
	}
	return attributes
}

func (value anyValue) stringValue() (string, bool) {
	switch {
	case value.StringValue != nil:
		return *value.StringValue, true
	case value.BoolValue != nil:
		return strconv.FormatBool(*value.BoolValue), true
	case len(value.IntValue) > 0:
		return scalarJSONValue(value.IntValue)
	case value.DoubleValue != nil:
		return strconv.FormatFloat(*value.DoubleValue, 'g', -1, 64), true
	case value.BytesValue != nil:
		return *value.BytesValue, true
	case len(value.ArrayValue) > 0:
		return string(value.ArrayValue), true
	case len(value.KVListValue) > 0:
		return string(value.KVListValue), true
	default:
		return "", false
	}
}

func parseUnixNano(raw json.RawMessage) (time.Time, error) {
	value, ok := scalarJSONValue(raw)
	if !ok {
		return time.Time{}, fmt.Errorf("valor ausente")
	}
	nanoseconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("valor inválido %q: %w", value, err)
	}
	return time.Unix(0, nanoseconds).UTC(), nil
}

func scalarJSONValue(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var stringValue string
	if err := json.Unmarshal(raw, &stringValue); err == nil {
		return stringValue, true
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		return number.String(), true
	}
	return "", false
}

func severity(code json.RawMessage) string {
	value, ok := scalarJSONValue(code)
	if !ok {
		return "info"
	}
	if value == "STATUS_CODE_ERROR" || value == "2" {
		return "error"
	}
	return "info"
}

func rejectTrailingJSON(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("interpretar OTLP JSON: %w", err)
	}
	return fmt.Errorf("interpretar OTLP JSON: dados adicionais após o payload")
}

func contextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("normalizar OTLP JSON: contexto cancelado: %w", err)
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(content []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(content)
}
