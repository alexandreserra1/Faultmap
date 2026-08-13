// Package domain contém os tipos centrais de telemetria sem dependências de transporte ou persistência.
package domain

import "time"

// SignalType identifica a categoria de telemetria representada por um Signal.
type SignalType string

const (
	// SignalTypeSpan representa um span distribuído normalizado a partir de um exportador OTLP.
	SignalTypeSpan SignalType = "span"
	// SignalTypeLog representa um registro de log correlacionado a um trace.
	// Apenas seus metadados são preservados; o texto da mensagem nunca é
	// armazenado, por ser onde dados sensíveis costumam aparecer.
	SignalTypeLog SignalType = "log"
)

// Signal representa uma unidade de telemetria pronta para ser validada, protegida e persistida.
type Signal struct {
	ID           string
	Type         SignalType
	ServiceName  string
	Timestamp    time.Time
	TraceID      string
	SpanID       string
	Severity     string
	Attributes   map[string]string
	Measurements map[string]float64
}
