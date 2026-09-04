package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/faultmap/faultmap/internal/application"
	jsonreport "github.com/faultmap/faultmap/internal/reporting/json"
)

// serverVersion identifica esta implementação para o cliente.
const serverVersion = "0.5.0"

// defaultIncidentLimit vale quando o cliente não informa quantos incidentes quer.
const defaultIncidentLimit = 20

type toolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// callTool executa uma ferramenta e devolve o resultado no formato do MCP.
//
// Erro de uso — argumento faltando, incidente inexistente, suspeito não
// encontrado — volta como resultado com isError, e não como erro de protocolo.
// A distinção importa: o modelo consegue corrigir sozinho um argumento errado,
// e não consegue fazer nada com uma falha de transporte.
func callTool(
	ctx context.Context,
	params json.RawMessage,
	options Options,
	auditor *auditor,
) map[string]any {
	var call toolCall
	if err := json.Unmarshal(params, &call); err != nil {
		auditor.record("<invalid-params>", err)
		return toolError(fmt.Errorf("parâmetros de tools/call inválidos: %w", err))
	}

	payload, err := runTool(ctx, call, options.History)
	auditor.record(call.Name, err)
	if err != nil {
		return toolError(err)
	}
	return map[string]any{
		"content": []any{map[string]any{"type": "text", "text": payload}},
	}
}

func runTool(
	ctx context.Context,
	call toolCall,
	history application.IncidentHistoryReader,
) (string, error) {
	switch call.Name {
	case toolListIncidents:
		return listIncidents(ctx, call.Arguments, history)
	case toolGetIncident:
		return getIncident(ctx, call.Arguments, history)
	case toolExplainSuspect:
		return explainSuspect(ctx, call.Arguments, history)
	default:
		return "", fmt.Errorf("tool %q não existe", call.Name)
	}
}

func listIncidents(
	ctx context.Context,
	arguments json.RawMessage,
	history application.IncidentHistoryReader,
) (string, error) {
	var input struct {
		Limit int `json:"limit"`
	}
	if err := decodeArguments(arguments, &input); err != nil {
		return "", err
	}
	if input.Limit <= 0 {
		input.Limit = defaultIncidentLimit
	}

	incidents, err := application.ListIncidents(ctx, input.Limit, history)
	if err != nil {
		return "", err
	}

	// A forma da listagem é definida aqui porque não há renderizador JSON de
	// lista no produto: os relatórios existentes descrevem um diagnóstico, não
	// um índice. As duas leituras seguintes reusam os renderizadores.
	summaries := make([]map[string]any, 0, len(incidents))
	for _, incident := range incidents {
		summaries = append(summaries, map[string]any{
			"incident_id":    incident.ID,
			"service_name":   incident.ServiceName,
			"environment":    incident.Environment,
			"status":         incident.Status,
			"incident_start": incident.IncidentStart.UTC().Format(timeLayout),
			"incident_end":   incident.IncidentEnd.UTC().Format(timeLayout),
		})
	}
	return marshalIndented(map[string]any{"incidents": summaries})
}

func getIncident(
	ctx context.Context,
	arguments json.RawMessage,
	history application.IncidentHistoryReader,
) (string, error) {
	var input struct {
		IncidentID string `json:"incident_id"`
	}
	if err := decodeArguments(arguments, &input); err != nil {
		return "", err
	}

	diagnosis, err := application.GetIncident(ctx, input.IncidentID, history)
	if err != nil {
		return "", err
	}

	// O relatório reusa o renderizador do `export report`. Definir aqui uma
	// segunda forma JSON do mesmo diagnóstico faria as duas divergirem com o
	// tempo — foi assim que a tela e os detectores passaram a discordar sobre o
	// mesmo span, duas vezes, antes de a lista de convenções virar um pacote só.
	var buffer bytes.Buffer
	if err := jsonreport.Render(&buffer, diagnosis); err != nil {
		return "", fmt.Errorf("renderizar diagnóstico: %w", err)
	}
	return buffer.String(), nil
}

func explainSuspect(
	ctx context.Context,
	arguments json.RawMessage,
	history application.IncidentHistoryReader,
) (string, error) {
	var input struct {
		IncidentID string `json:"incident_id"`
		Suspect    string `json:"suspect"`
	}
	if err := decodeArguments(arguments, &input); err != nil {
		return "", err
	}

	diagnosis, err := application.GetIncident(ctx, input.IncidentID, history)
	if err != nil {
		return "", err
	}
	explanation, err := application.ExplainSuspect(diagnosis, input.Suspect)
	if err != nil {
		return "", err
	}

	contributions := make([]map[string]any, 0, len(explanation.Contributions))
	for _, contribution := range explanation.Contributions {
		contributions = append(contributions, map[string]any{
			"rule_id":   contribution.RuleID,
			"value":     contribution.Value,
			"reason":    contribution.Reason,
			"supported": contribution.Supported,
			"findings":  describeFindings(contribution.Findings),
		})
	}
	return marshalIndented(map[string]any{
		"incident_id":       explanation.IncidentID,
		"suspect_id":        explanation.SuspectID,
		"suspect_label":     explanation.SuspectLabel,
		"score":             explanation.Score,
		"confidence":        string(explanation.Confidence),
		"contributions":     contributions,
		"unscored_findings": describeFindings(explanation.UnscoredFindings),
		"limitations":       explanation.Limitations,
	})
}
