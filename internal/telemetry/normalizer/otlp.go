package normalizer

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// ErrInvalidOTLP classifica formatos não suportados ou payloads que não podem
// ser interpretados, permitindo distingui-los de falhas de persistência.
var ErrInvalidOTLP = errors.New("payload OTLP inválido")

// OTLPEncoding identifica como uma requisição OTLP de traces foi serializada.
type OTLPEncoding string

const (
	// OTLPEncodingJSON representa o mapeamento JSON definido pelo protocolo OTLP.
	OTLPEncodingJSON OTLPEncoding = "json"
	// OTLPEncodingProtobuf representa a serialização binária protobuf do OTLP.
	OTLPEncodingProtobuf OTLPEncoding = "protobuf"
)

// ParseOTLPTraces interpreta traces OTLP no formato informado e produz o mesmo
// contrato interno, independentemente do transporte que recebeu o payload.
func ParseOTLPTraces(ctx context.Context, reader io.Reader, encoding OTLPEncoding) ([]domain.Signal, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, fmt.Errorf("interpretar traces OTLP: leitor é obrigatório")
	}

	switch encoding {
	case OTLPEncodingJSON:
		signals, err := ParseOTLPJSON(ctx, reader)
		return signals, classifyOTLPError(err)
	case OTLPEncodingProtobuf:
		signals, err := parseOTLPProtobuf(ctx, reader)
		return signals, classifyOTLPError(err)
	default:
		return nil, fmt.Errorf("%w: codificação %q não suportada", ErrInvalidOTLP, encoding)
	}
}

func classifyOTLPError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrInvalidOTLP, err)
}
