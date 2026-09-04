// Package mcp expõe diagnósticos já persistidos a clientes MCP.
//
// O servidor é somente leitura, e isso é uma posição do produto, não uma etapa
// de implementação. O motor de diagnóstico do Faultmap é determinístico; o
// papel de um LLM é consumir e explicar o resultado estruturado, não participar
// da análise. Uma tool que disparasse `diagnose` colocaria o modelo dentro do
// caminho que o produto promete ser reproduzível.
//
// Ser somente leitura também zera a superfície de privacidade: tudo que sai
// daqui já passou pela política aplicada na ingestão (ADRs 0008, 0011 e 0012).
// O servidor não tem como revelar o que a ingestão barrou, porque nunca chegou
// a guardar.
//
// O protocolo é escrito à mão em vez de trazer um SDK. MCP sobre stdio é
// pequeno — enquadramento por linha, `initialize`, `tools/list` e `tools/call` —
// e o produto se distribui como binário estático único com um go.mod enxuto.
// O custo é acompanhar a evolução da especificação manualmente.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/application"
)

// protocolVersion é a revisão da especificação que este servidor implementa.
const protocolVersion = "2025-06-18"

// maxRequestBytes limita uma única linha de entrada. Sem teto, um cliente
// defeituoso enviando uma linha infinita consumiria toda a memória do processo.
//
// Estourar o teto responde erro e segue: a primeira versão usava bufio.Scanner,
// que trata linha longa demais como falha de leitura e encerrava a sessão
// inteira. Uma requisição malformada derrubava a investigação de quem estava do
// outro lado.
const maxRequestBytes = 1 << 20

// defaultReadBuffer é o buffer de leitura; mensagens maiores são montadas em
// pedaços, sem que o buffer precise comportar a maior delas.
const defaultReadBuffer = 64 << 10

// Códigos de erro definidos pelo JSON-RPC 2.0.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInternalError  = -32603
)

// Options reúne as dependências do servidor.
//
// Entrada e saída são interfaces, e não os descritores do processo, para que a
// sessão inteira possa ser exercitada em memória: os testes de protocolo rodam
// sem subir processo nem depender de um cliente externo.
type Options struct {
	Input   io.Reader
	Output  io.Writer
	Audit   io.Writer
	History application.IncidentHistoryReader
	// Now existe para tornar a auditoria reproduzível nos testes.
	Now func() time.Time
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// isNotification distingue requisição de notificação. A ausência de `id` é o
// único critério do JSON-RPC, e responder a uma notificação quebra clientes que
// não esperam nada de volta.
func (message request) isNotification() bool {
	return len(message.ID) == 0 || string(message.ID) == "null"
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

// Serve atende uma sessão MCP até o fim da entrada ou o cancelamento do contexto.
//
// Nenhum erro de uma mensagem encerra a sessão. JSON inválido, método
// desconhecido e tool inexistente viram resposta de erro e o laço continua: um
// cliente mal comportado não pode derrubar o servidor de quem está no meio de
// uma investigação.
func Serve(ctx context.Context, options Options) error {
	if options.Input == nil || options.Output == nil {
		return fmt.Errorf("servidor MCP: entrada e saída são obrigatórias")
	}
	if options.History == nil {
		return fmt.Errorf("servidor MCP: leitor de histórico é obrigatório")
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}

	auditor := newAuditor(options.Audit, options.Now)
	reader := bufio.NewReaderSize(options.Input, defaultReadBuffer)
	encoder := json.NewEncoder(options.Output)

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		raw, tooLong, readErr := readLine(reader)
		if tooLong {
			if err := encoder.Encode(protocolError(
				json.RawMessage("null"), codeInvalidRequest,
				fmt.Sprintf("mensagem excede o limite de %d bytes", maxRequestBytes),
			)); err != nil {
				return fmt.Errorf("servidor MCP: escrever resposta: %w", err)
			}
			auditor.record("<request-too-large>", errors.New("linha excede o limite"))
		} else if line := strings.TrimSpace(string(raw)); line != "" {
			if err := dispatch(ctx, line, options, auditor, encoder); err != nil {
				return err
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return fmt.Errorf("servidor MCP: ler entrada: %w", readErr)
		}
	}
}

// dispatch trata uma mensagem já delimitada.
func dispatch(
	ctx context.Context,
	line string,
	options Options,
	auditor *auditor,
	encoder *json.Encoder,
) error {
	var message request
	if err := json.Unmarshal([]byte(line), &message); err != nil {
		// Sem `id` legível não há a quem endereçar a resposta; o JSON-RPC
		// manda usar id nulo nesse caso.
		if writeErr := encoder.Encode(protocolError(
			json.RawMessage("null"), codeParseError, "mensagem não é JSON válido",
		)); writeErr != nil {
			return fmt.Errorf("servidor MCP: escrever resposta: %w", writeErr)
		}
		auditor.record("<parse-error>", errors.New("json inválido"))
		return nil
	}

	reply, respond := handle(ctx, message, options, auditor)
	if !respond {
		return nil
	}
	if err := encoder.Encode(reply); err != nil {
		return fmt.Errorf("servidor MCP: escrever resposta: %w", err)
	}
	return nil
}

// readLine lê uma mensagem inteira, sinalizando quando ela estourou o teto.
//
// O excedente é descartado até a próxima quebra de linha, e não abandonado no
// meio: parar de ler ali faria o resto da mensagem grande ser interpretado como
// as mensagens seguintes, transformando um erro em uma enxurrada deles.
func readLine(reader *bufio.Reader) (line []byte, tooLong bool, err error) {
	for {
		chunk, readErr := reader.ReadSlice('\n')
		if !tooLong && len(line)+len(chunk) > maxRequestBytes {
			tooLong, line = true, nil
		}
		if !tooLong {
			// ReadSlice devolve uma fatia do buffer interno, válida só até a
			// próxima leitura; append copia.
			line = append(line, chunk...)
		}
		if errors.Is(readErr, bufio.ErrBufferFull) {
			continue
		}
		return line, tooLong, readErr
	}
}

func protocolError(id json.RawMessage, code int, message string) response {
	return response{JSONRPC: "2.0", ID: id, Error: &responseError{Code: code, Message: message}}
}

// handle devolve a resposta e se ela deve ser enviada. Notificações não recebem
// resposta alguma.
func handle(
	ctx context.Context,
	message request,
	options Options,
	auditor *auditor,
) (response, bool) {
	if message.isNotification() {
		return response{}, false
	}

	reply := response{JSONRPC: "2.0", ID: message.ID}
	switch message.Method {
	case "initialize":
		reply.Result = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "faultmap", "version": serverVersion},
		}
	case "ping":
		reply.Result = map[string]any{}
	case "tools/list":
		reply.Result = map[string]any{"tools": toolDefinitions()}
	case "tools/call":
		reply.Result = callTool(ctx, message.Params, options, auditor)
	default:
		reply.Error = &responseError{
			Code:    codeMethodNotFound,
			Message: fmt.Sprintf("método %q não é suportado", message.Method),
		}
	}
	return reply, true
}
