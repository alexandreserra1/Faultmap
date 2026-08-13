package normalizer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// O corpo do log nunca é persistido.
//
// A mensagem é ao mesmo tempo a parte mais útil e a mais perigosa da telemetria
// de logs: é onde aparecem endereço de e-mail, documento, token e corpo de
// requisição. O Faultmap guarda o que sustenta um diagnóstico — severidade,
// instante, correlação de trace e atributos permitidos — e deixa a leitura da
// mensagem para o sistema de logs de origem, que já existe e é feito para isso.
//
// O corpo é usado apenas para compor o identificador do registro, que precisa
// distinguir dois logs emitidos no mesmo instante. Ele entra em um resumo
// criptográfico e é descartado em seguida, sem nunca alcançar a persistência.

type exportLogsServiceRequest struct {
	ResourceLogs []resourceLog `json:"resourceLogs"`
}

type resourceLog struct {
	Resource  resource   `json:"resource"`
	ScopeLogs []scopeLog `json:"scopeLogs"`
}

type scopeLog struct {
	LogRecords []logRecord `json:"logRecords"`
}

type logRecord struct {
	TimeUnixNano         json.RawMessage `json:"timeUnixNano"`
	ObservedTimeUnixNano json.RawMessage `json:"observedTimeUnixNano"`
	SeverityNumber       json.RawMessage `json:"severityNumber"`
	SeverityText         string          `json:"severityText"`
	Body                 anyValue        `json:"body"`
	Attributes           []keyValue      `json:"attributes"`
	TraceID              string          `json:"traceId"`
	SpanID               string          `json:"spanId"`
}

// ParseOTLPLogsJSON normaliza um lote de logs OTLP em JSON.
func ParseOTLPLogsJSON(ctx context.Context, reader io.Reader) ([]domain.Signal, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, fmt.Errorf("interpretar OTLP JSON de logs: leitor é obrigatório")
	}

	var request exportLogsServiceRequest
	decoder := json.NewDecoder(contextReader{ctx: ctx, reader: reader})
	if err := decoder.Decode(&request); err != nil {
		return nil, fmt.Errorf("interpretar OTLP JSON de logs: %w", err)
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		return nil, fmt.Errorf("interpretar OTLP JSON de logs: %w", err)
	}

	signals := make([]domain.Signal, 0)
	for _, resourceLogs := range request.ResourceLogs {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		resourceAttributes := resourceSignalAttributes(resourceLogs.Resource.Attributes)
		service := serviceName(resourceLogs.Resource.Attributes)
		for _, scope := range resourceLogs.ScopeLogs {
			for _, record := range scope.LogRecords {
				signal, err := normalizeLogRecord(service, resourceAttributes, record)
				if err != nil {
					return nil, fmt.Errorf("interpretar OTLP JSON de logs: %w", err)
				}
				signals = append(signals, signal)
			}
		}
	}
	return signals, nil
}

func normalizeLogRecord(
	service string,
	resourceAttributes map[string]string,
	record logRecord,
) (domain.Signal, error) {
	timestamp, err := parseUnixNano(record.TimeUnixNano)
	if err != nil {
		// Quando o produtor não informa o instante do evento, o instante em que
		// a coleta o observou é a melhor aproximação disponível.
		timestamp, err = parseUnixNano(record.ObservedTimeUnixNano)
		if err != nil {
			return domain.Signal{}, fmt.Errorf("registro de log sem instante utilizável")
		}
	}

	attributes := make(map[string]string, len(resourceAttributes)+len(record.Attributes))
	for key, value := range resourceAttributes {
		attributes[key] = value
	}
	for key, value := range normalizeAttributes(record.Attributes) {
		attributes[key] = value
	}

	return domain.Signal{
		ID:          logRecordID(service, record),
		Type:        domain.SignalTypeLog,
		ServiceName: service,
		Timestamp:   timestamp,
		TraceID:     strings.TrimSpace(record.TraceID),
		SpanID:      strings.TrimSpace(record.SpanID),
		Severity:    logSeverity(record),
		Attributes:  attributes,
		// Logs não trazem duração; o mapa existe para manter o contrato do sinal.
		Measurements: map[string]float64{},
	}, nil
}

// logSeverity reduz a severidade ao vocabulário que os detectores já usam.
//
// O texto é preferido por ser o que a aplicação declarou. Na ausência dele, o
// número segue a escala do OpenTelemetry, em que 17 inicia a faixa de erro.
func logSeverity(record logRecord) string {
	if text := strings.TrimSpace(record.SeverityText); text != "" {
		lowered := strings.ToLower(text)
		if strings.HasPrefix(lowered, "err") || strings.HasPrefix(lowered, "fatal") ||
			strings.HasPrefix(lowered, "crit") {
			return "error"
		}
		return lowered
	}
	if value, ok := scalarJSONValue(record.SeverityNumber); ok {
		if number, err := strconv.Atoi(value); err == nil && number >= 17 {
			return "error"
		}
	}
	return "info"
}

// logRecordID deriva um identificador estável do conteúdo do registro.
//
// Reenviar o mesmo lote não pode duplicar sinais, e dois logs emitidos no mesmo
// instante precisam continuar distintos. O corpo participa do resumo por ser o
// que melhor os diferencia — e é descartado logo em seguida, sem ser guardado.
func logRecordID(service string, record logRecord) string {
	body, _ := record.Body.stringValue()
	parts := []string{
		service,
		strings.TrimSpace(record.TraceID),
		strings.TrimSpace(record.SpanID),
		string(record.TimeUnixNano),
		strings.TrimSpace(record.SeverityText),
		body,
	}
	for _, attribute := range record.Attributes {
		value, _ := attribute.Value.stringValue()
		parts = append(parts, attribute.Key+"="+value)
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "log:" + hex.EncodeToString(digest[:16])
}
