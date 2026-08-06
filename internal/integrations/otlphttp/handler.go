// Package otlphttp expõe a entrada HTTP do protocolo OTLP sem acoplar o
// transporte à normalização, à persistência ou ao ciclo de vida do servidor.
package otlphttp

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

const (
	// TracePath é o endpoint HTTP padronizado pelo OTLP para exportação de traces.
	TracePath = "/v1/traces"
	// HealthPath é o endpoint simples de vivacidade, separado do protocolo OTLP.
	HealthPath = "/health"

	// ContentTypeJSON identifica payloads OTLP codificados como JSON.
	ContentTypeJSON = "application/json"
	// ContentTypeProtobuf identifica payloads OTLP codificados como protobuf.
	ContentTypeProtobuf = "application/x-protobuf"

	// DefaultMaxRequestBodyBytes limita uma requisição a 64 MiB quando nenhuma
	// configuração explícita é fornecida.
	DefaultMaxRequestBodyBytes int64 = 64 << 20
)

const (
	googleCodeInvalidArgument         int32 = 3
	googleCodeResourceExhausted       int32 = 8
	googleCodeUnimplemented           int32 = 12
	googleCodeInternal                int32 = 13
	internalFailureMessage                  = "falha interna ao ingerir traces"
	invalidPayloadMessage                   = "payload OTLP inválido"
	unsupportedMediaTypeMessage             = "Content-Type OTLP não suportado"
	unsupportedContentEncodingMessage       = "Content-Encoding OTLP não suportado"
	bodyTooLargeMessage                     = "payload OTLP excede o limite"
	methodNotAllowedMessage                 = "método HTTP não permitido"
)

// ErrInvalidPayload permite que a integração de normalização classifique um
// erro de entrada sem expor detalhes internos ao cliente OTLP.
var ErrInvalidPayload = errors.New("payload OTLP inválido")

// Encoding representa as codificações aceitas pelo transporte OTLP HTTP.
type Encoding string

const (
	// EncodingJSON representa uma requisição OTLP JSON.
	EncodingJSON Encoding = "json"
	// EncodingProtobuf representa uma requisição OTLP protobuf.
	EncodingProtobuf Encoding = "protobuf"
)

// TraceIngester define a única dependência do transporte. A implementação deve
// normalizar e persistir o payload usando o contexto recebido da requisição.
type TraceIngester interface {
	IngestTraces(ctx context.Context, reader io.Reader, encoding Encoding) error
}

// IngestFunc adapta uma função ao contrato TraceIngester.
type IngestFunc func(ctx context.Context, reader io.Reader, encoding Encoding) error

// IngestTraces encaminha a chamada à função adaptada.
func (function IngestFunc) IngestTraces(ctx context.Context, reader io.Reader, encoding Encoding) error {
	return function(ctx, reader, encoding)
}

// Options reúne limites do handler. Timeouts e encerramento controlado
// pertencem ao servidor HTTP que hospeda este handler.
type Options struct {
	MaxRequestBodyBytes int64
}

type traceHandler struct {
	ingester            TraceIngester
	maxRequestBodyBytes int64
}

// NewHandler cria um handler que atende somente o endpoint OTLP /v1/traces.
// O handler permanece stateless; toda operação durável é delegada ao ingester.
func NewHandler(ingester TraceIngester, options Options) (http.Handler, error) {
	if ingester == nil {
		return nil, fmt.Errorf("criar handler OTLP HTTP: ingester é obrigatório")
	}
	if function, ok := ingester.(IngestFunc); ok && function == nil {
		return nil, fmt.Errorf("criar handler OTLP HTTP: ingester é obrigatório")
	}
	if options.MaxRequestBodyBytes < 0 {
		return nil, fmt.Errorf("criar handler OTLP HTTP: limite do corpo não pode ser negativo")
	}

	maxRequestBodyBytes := options.MaxRequestBodyBytes
	if maxRequestBodyBytes == 0 {
		maxRequestBodyBytes = DefaultMaxRequestBodyBytes
	}
	handler := &traceHandler{
		ingester:            ingester,
		maxRequestBodyBytes: maxRequestBodyBytes,
	}
	mux := http.NewServeMux()
	mux.Handle(TracePath, handler)
	return mux, nil
}

// NewHealthHandler cria um endpoint de vivacidade independente do banco e da
// ingestão. Ele sinaliza apenas que o processo HTTP consegue responder.
func NewHealthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(HealthPath, func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeGoogleStatus(response, ContentTypeJSON, http.StatusMethodNotAllowed, googleCodeUnimplemented, methodNotAllowedMessage)
			return
		}
		response.Header().Set("Content-Type", ContentTypeJSON)
		response.WriteHeader(http.StatusOK)
		if _, err := response.Write([]byte(`{"status":"ok"}`)); err != nil {
			return
		}
	})
	return mux
}

func (handler *traceHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	encoding, contentType, contentTypeOK := parseContentType(request.Header.Get("Content-Type"))
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		writeGoogleStatus(response, fallbackContentType(contentType, contentTypeOK), http.StatusMethodNotAllowed, googleCodeUnimplemented, methodNotAllowedMessage)
		return
	}
	if !contentTypeOK {
		writeGoogleStatus(response, ContentTypeJSON, http.StatusUnsupportedMediaType, googleCodeInvalidArgument, unsupportedMediaTypeMessage)
		return
	}
	contentEncoding, contentEncodingOK := parseContentEncoding(request.Header.Get("Content-Encoding"))
	if !contentEncodingOK {
		writeGoogleStatus(response, contentType, http.StatusUnsupportedMediaType, googleCodeInvalidArgument, unsupportedContentEncodingMessage)
		return
	}
	if request.ContentLength > handler.maxRequestBodyBytes {
		writeGoogleStatus(response, contentType, http.StatusRequestEntityTooLarge, googleCodeResourceExhausted, bodyTooLargeMessage)
		return
	}
	if request.Context().Err() != nil {
		return
	}

	payload, err := io.ReadAll(io.LimitReader(request.Body, handler.maxRequestBodyBytes+1))
	if err != nil {
		if request.Context().Err() != nil {
			return
		}
		writeGoogleStatus(response, contentType, http.StatusBadRequest, googleCodeInvalidArgument, invalidPayloadMessage)
		return
	}
	if int64(len(payload)) > handler.maxRequestBodyBytes {
		writeGoogleStatus(response, contentType, http.StatusRequestEntityTooLarge, googleCodeResourceExhausted, bodyTooLargeMessage)
		return
	}
	payload, tooLarge, err := decodePayload(payload, contentEncoding, handler.maxRequestBodyBytes)
	if tooLarge {
		writeGoogleStatus(response, contentType, http.StatusRequestEntityTooLarge, googleCodeResourceExhausted, bodyTooLargeMessage)
		return
	}
	if err != nil {
		writeGoogleStatus(response, contentType, http.StatusBadRequest, googleCodeInvalidArgument, invalidPayloadMessage)
		return
	}
	if request.Context().Err() != nil {
		return
	}
	// Um ExportTraceServiceRequest sem campos é válido e não possui trabalho de
	// normalização ou persistência a executar.
	if len(payload) == 0 {
		writeSuccess(response, contentType)
		return
	}

	err = handler.ingester.IngestTraces(request.Context(), bytes.NewReader(payload), encoding)
	if err == nil {
		writeSuccess(response, contentType)
		return
	}
	if request.Context().Err() != nil {
		return
	}
	if errors.Is(err, ErrInvalidPayload) {
		writeGoogleStatus(response, contentType, http.StatusBadRequest, googleCodeInvalidArgument, invalidPayloadMessage)
		return
	}
	writeGoogleStatus(response, contentType, http.StatusInternalServerError, googleCodeInternal, internalFailureMessage)
}

func parseContentEncoding(header string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(header)) {
	case "", "identity":
		return "identity", true
	case "gzip":
		return "gzip", true
	default:
		return "", false
	}
}

// decodePayload aplica o limite também aos bytes descompactados para impedir
// que um payload gzip pequeno expanda além da memória aceita pelo servidor.
func decodePayload(payload []byte, contentEncoding string, maxBytes int64) ([]byte, bool, error) {
	if contentEncoding != "gzip" {
		return payload, false, nil
	}
	reader, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, false, fmt.Errorf("abrir payload OTLP gzip: %w", err)
	}
	decoded, readErr := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		return nil, false, errors.Join(readErr, closeErr)
	}
	if int64(len(decoded)) > maxBytes {
		return nil, true, nil
	}
	return decoded, false, nil
}

func parseContentType(header string) (Encoding, string, bool) {
	mediaType, _, err := mime.ParseMediaType(header)
	if err != nil {
		return "", "", false
	}
	switch mediaType {
	case ContentTypeJSON:
		return EncodingJSON, ContentTypeJSON, true
	case ContentTypeProtobuf:
		return EncodingProtobuf, ContentTypeProtobuf, true
	default:
		return "", "", false
	}
}

func fallbackContentType(contentType string, valid bool) string {
	if valid {
		return contentType
	}
	return ContentTypeJSON
}

func writeSuccess(response http.ResponseWriter, contentType string) {
	response.Header().Set("Content-Type", contentType)
	response.WriteHeader(http.StatusOK)
	if contentType == ContentTypeJSON {
		if _, err := response.Write([]byte("{}")); err != nil {
			return
		}
	}
}

func writeGoogleStatus(response http.ResponseWriter, contentType string, httpCode int, googleCode int32, message string) {
	response.Header().Set("Content-Type", contentType)
	response.WriteHeader(httpCode)

	var payload []byte
	if contentType == ContentTypeProtobuf {
		payload = marshalGoogleStatusProtobuf(googleCode, message)
	} else {
		encoded, err := json.Marshal(struct {
			Code    int32  `json:"code"`
			Message string `json:"message"`
		}{Code: googleCode, Message: message})
		if err != nil {
			return
		}
		payload = encoded
	}
	if _, err := response.Write(payload); err != nil {
		return
	}
}

// marshalGoogleStatusProtobuf codifica os dois campos usados de
// google.rpc.Status sem introduzir uma dependência protobuf no transporte.
func marshalGoogleStatusProtobuf(code int32, message string) []byte {
	payload := make([]byte, 0, len(message)+8)
	payload = append(payload, 0x08)
	payload = appendUvarint(payload, uint64(code))
	payload = append(payload, 0x12)
	payload = appendUvarint(payload, uint64(len(message)))
	payload = append(payload, message...)
	return payload
}

func appendUvarint(destination []byte, value uint64) []byte {
	for value >= 0x80 {
		destination = append(destination, byte(value)|0x80)
		value >>= 7
	}
	return append(destination, byte(value))
}
