// Package terminal formata informações de diagnóstico para leitura humana no terminal.
package terminal

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// RenderSignals escreve uma visão cronológica e segura dos sinais de um serviço.
// Somente campos explicitamente permitidos são exibidos para não expor atributos
// arbitrários que possam conter dados sensíveis de telemetria.
func RenderSignals(writer io.Writer, serviceName string, signals []domain.Signal) error {
	if len(signals) == 0 {
		if _, err := fmt.Fprintf(writer, "Telemetria de %s — nenhum sinal encontrado.\n", serviceName); err != nil {
			return fmt.Errorf("escrever sinais no terminal: %w", err)
		}
		return nil
	}

	orderedSignals := append([]domain.Signal(nil), signals...)
	sort.Slice(orderedSignals, func(firstIndex, secondIndex int) bool {
		first := orderedSignals[firstIndex]
		second := orderedSignals[secondIndex]
		if !first.Timestamp.Equal(second.Timestamp) {
			return first.Timestamp.Before(second.Timestamp)
		}
		if first.ID != second.ID {
			return first.ID < second.ID
		}
		if first.TraceID != second.TraceID {
			return first.TraceID < second.TraceID
		}
		return first.SpanID < second.SpanID
	})

	var output strings.Builder
	fmt.Fprintf(&output, "Telemetria de %s — %d %s\n\n", serviceName, len(orderedSignals), signalNoun(len(orderedSignals)))
	for index, signal := range orderedSignals {
		writeSignal(&output, signal)
		if index < len(orderedSignals)-1 {
			output.WriteByte('\n')
		}
	}

	if _, err := io.WriteString(writer, output.String()); err != nil {
		return fmt.Errorf("escrever sinais no terminal: %w", err)
	}
	return nil
}

func signalNoun(count int) string {
	if count == 1 {
		return "sinal"
	}
	return "sinais"
}

func writeSignal(output *strings.Builder, signal domain.Signal) {
	fmt.Fprintf(output, "%s  %s  %s\n", signal.Timestamp.UTC().Format("2006-01-02 15:04:05 MST"), displaySeverity(signal.Severity), spanName(signal))
	fmt.Fprintf(output, "  %s\n", strings.Join(signalDetails(signal), " · "))
}

func displaySeverity(severity string) string {
	if strings.EqualFold(strings.TrimSpace(severity), "error") {
		return "ERRO"
	}
	if strings.TrimSpace(severity) == "" {
		return "INFO"
	}
	return strings.ToUpper(severity)
}

func spanName(signal domain.Signal) string {
	if name := strings.TrimSpace(signal.Attributes["span.name"]); name != "" {
		return name
	}
	return "span sem nome"
}

func signalDetails(signal domain.Signal) []string {
	details := make([]string, 0, 4)
	if statusCode := firstAttribute(signal.Attributes, "http.response.status_code", "http.status_code"); statusCode != "" {
		details = append(details, "HTTP "+statusCode)
	}
	if databaseDetail := databaseDetail(signal.Attributes); databaseDetail != "" {
		details = append(details, databaseDetail)
	}
	if errorType := firstAttribute(signal.Attributes, "error.type", "exception.type"); errorType != "" {
		details = append(details, "erro "+errorType)
	}
	details = append(details, "duração "+duration(signal))
	details = append(details, "trace "+traceID(signal))
	return details
}

func firstAttribute(attributes map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(attributes[key]); value != "" {
			return value
		}
	}
	return ""
}

func databaseDetail(attributes map[string]string) string {
	system := displayDatabaseSystem(firstAttribute(attributes, "db.system", "db.system.name"))
	operation := firstAttribute(attributes, "db.operation.name")
	switch {
	case system != "" && operation != "":
		return "Banco " + system + " · operação " + operation
	case system != "":
		return "Banco " + system
	case operation != "":
		return "Operação " + operation
	default:
		return ""
	}
}

func displayDatabaseSystem(system string) string {
	if strings.EqualFold(system, "postgresql") {
		return "PostgreSQL"
	}
	return system
}

func duration(signal domain.Signal) string {
	value, ok := signal.Measurements["duration_ms"]
	if !ok {
		return "indisponível"
	}
	return strconv.FormatFloat(value, 'f', -1, 64) + " ms"
}

func traceID(signal domain.Signal) string {
	if value := strings.TrimSpace(signal.TraceID); value != "" {
		return value
	}
	return "indisponível"
}
