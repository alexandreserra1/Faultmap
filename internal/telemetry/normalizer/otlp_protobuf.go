package normalizer

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
	collectortracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// parseOTLPProtobuf desserializa somente o envelope oficial de traces e o
// converte ao modelo intermediário usado também pelo parser JSON.
func parseOTLPProtobuf(ctx context.Context, reader io.Reader) ([]domain.Signal, error) {
	payload, err := io.ReadAll(contextReader{ctx: ctx, reader: reader})
	if err != nil {
		return nil, fmt.Errorf("ler OTLP protobuf: %w", err)
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	request := &collectortracev1.ExportTraceServiceRequest{}
	if err := proto.Unmarshal(payload, request); err != nil {
		return nil, fmt.Errorf("interpretar OTLP protobuf: %w", err)
	}

	return normalizeExportRequest(ctx, protobufRequest(request))
}

// protobufRequest adapta tipos gerados pelo protocolo sem carregar essas
// dependências para o domínio nem duplicar as regras de normalização.
func protobufRequest(source *collectortracev1.ExportTraceServiceRequest) exportTraceServiceRequest {
	request := exportTraceServiceRequest{ResourceSpans: make([]resourceSpan, 0, len(source.GetResourceSpans()))}
	for _, sourceResourceSpans := range source.GetResourceSpans() {
		converted := resourceSpan{}
		if sourceResourceSpans.GetResource() != nil {
			converted.Resource.Attributes = protobufAttributes(sourceResourceSpans.GetResource().GetAttributes())
		}
		converted.ScopeSpans = make([]scopeSpan, 0, len(sourceResourceSpans.GetScopeSpans()))
		for _, sourceScopeSpans := range sourceResourceSpans.GetScopeSpans() {
			convertedScope := scopeSpan{Spans: make([]span, 0, len(sourceScopeSpans.GetSpans()))}
			for _, sourceSpan := range sourceScopeSpans.GetSpans() {
				convertedScope.Spans = append(convertedScope.Spans, protobufSpan(sourceSpan))
			}
			converted.ScopeSpans = append(converted.ScopeSpans, convertedScope)
		}
		request.ResourceSpans = append(request.ResourceSpans, converted)
	}
	return request
}

func protobufSpan(source *tracev1.Span) span {
	kind := spanKind("")
	if source.GetKind() != tracev1.Span_SPAN_KIND_UNSPECIFIED {
		kind = spanKind(source.GetKind().String())
	}
	converted := span{
		TraceID:           hex.EncodeToString(source.GetTraceId()),
		SpanID:            hex.EncodeToString(source.GetSpanId()),
		ParentSpanID:      hex.EncodeToString(source.GetParentSpanId()),
		Name:              source.GetName(),
		Kind:              kind,
		StartTimeUnixNano: json.RawMessage(strconv.FormatUint(source.GetStartTimeUnixNano(), 10)),
		EndTimeUnixNano:   json.RawMessage(strconv.FormatUint(source.GetEndTimeUnixNano(), 10)),
		Attributes:        protobufAttributes(source.GetAttributes()),
		Events:            protobufEvents(source.GetEvents()),
	}
	if source.GetStatus() != nil {
		converted.Status = status{
			Code:    json.RawMessage(strconv.Itoa(int(source.GetStatus().GetCode()))),
			Message: source.GetStatus().GetMessage(),
		}
	}
	return converted
}

func protobufAttributes(source []*commonv1.KeyValue) []keyValue {
	attributes := make([]keyValue, 0, len(source))
	for _, attribute := range source {
		if attribute == nil {
			continue
		}
		attributes = append(attributes, keyValue{Key: attribute.GetKey(), Value: protobufAnyValue(attribute.GetValue())})
	}
	return attributes
}

// protobufAnyValue preserva os tipos escalares e serializa coleções de modo
// determinístico antes das regras existentes de filtragem de atributos.
func protobufAnyValue(source *commonv1.AnyValue) anyValue {
	if source == nil {
		return anyValue{}
	}
	switch value := source.Value.(type) {
	case *commonv1.AnyValue_StringValue:
		return anyValue{StringValue: &value.StringValue}
	case *commonv1.AnyValue_BoolValue:
		return anyValue{BoolValue: &value.BoolValue}
	case *commonv1.AnyValue_IntValue:
		return anyValue{IntValue: json.RawMessage(strconv.FormatInt(value.IntValue, 10))}
	case *commonv1.AnyValue_DoubleValue:
		return anyValue{DoubleValue: &value.DoubleValue}
	case *commonv1.AnyValue_BytesValue:
		encoded := base64.StdEncoding.EncodeToString(value.BytesValue)
		return anyValue{BytesValue: &encoded}
	case *commonv1.AnyValue_ArrayValue:
		return anyValue{ArrayValue: marshalProtoValue(protobufArrayValue(value.ArrayValue))}
	case *commonv1.AnyValue_KvlistValue:
		return anyValue{KVListValue: marshalProtoValue(protobufKVListValue(value.KvlistValue))}
	default:
		return anyValue{}
	}
}

func protobufArrayValue(source *commonv1.ArrayValue) []any {
	values := make([]any, 0, len(source.GetValues()))
	for _, value := range source.GetValues() {
		values = append(values, protobufNativeValue(value))
	}
	return values
}

func protobufKVListValue(source *commonv1.KeyValueList) map[string]any {
	values := make(map[string]any, len(source.GetValues()))
	for _, value := range source.GetValues() {
		if value != nil {
			values[value.GetKey()] = protobufNativeValue(value.GetValue())
		}
	}
	return values
}

func protobufNativeValue(source *commonv1.AnyValue) any {
	if source == nil {
		return nil
	}
	switch value := source.Value.(type) {
	case *commonv1.AnyValue_StringValue:
		return value.StringValue
	case *commonv1.AnyValue_BoolValue:
		return value.BoolValue
	case *commonv1.AnyValue_IntValue:
		return value.IntValue
	case *commonv1.AnyValue_DoubleValue:
		return value.DoubleValue
	case *commonv1.AnyValue_BytesValue:
		return base64.StdEncoding.EncodeToString(value.BytesValue)
	case *commonv1.AnyValue_ArrayValue:
		return protobufArrayValue(value.ArrayValue)
	case *commonv1.AnyValue_KvlistValue:
		return protobufKVListValue(value.KvlistValue)
	default:
		return nil
	}
}

func marshalProtoValue(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return encoded
}

// protobufEvents converte os eventos do span para a mesma representação usada
// pelo caminho JSON, de modo que a promoção da causa da exceção funcione
// independentemente do formato em que a telemetria chegou.
func protobufEvents(source []*tracev1.Span_Event) []spanEvent {
	if len(source) == 0 {
		return nil
	}
	events := make([]spanEvent, 0, len(source))
	for _, event := range source {
		if event == nil {
			continue
		}
		events = append(events, spanEvent{
			Name:       event.GetName(),
			Attributes: protobufAttributes(event.GetAttributes()),
		})
	}
	return events
}
