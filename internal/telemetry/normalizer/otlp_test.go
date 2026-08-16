package normalizer

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// TestParseOTLPLogsProtobufExplicaComoCorrigir cobre uma recusa que falha em
// silêncio na prática: quem configura OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
// vê os traces entrarem e os logs sumirem, sem nada que ligue as duas coisas. A
// mensagem precisa dizer o que fazer, não apenas o que foi rejeitado.
func TestParseOTLPLogsProtobufExplicaComoCorrigir(t *testing.T) {
	t.Parallel()

	_, err := ParseOTLPLogs(context.Background(), bytes.NewReader([]byte{0x0a, 0x00}), OTLPEncodingProtobuf)
	if err == nil {
		t.Fatal("logs em protobuf foram aceitos")
	}
	if !errors.Is(err, ErrInvalidOTLP) {
		t.Fatalf("erro não classificado como payload inválido: %v", err)
	}
	for _, esperado := range []string{"JSON", "encoding: json"} {
		if !strings.Contains(err.Error(), esperado) {
			t.Fatalf("a mensagem não orienta a correção (%q ausente): %v", esperado, err)
		}
	}
}
