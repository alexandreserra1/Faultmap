package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/detection"
)

// Nomes das ferramentas. Todos descrevem leitura: não há tool que dispare
// investigação, ingira dados ou apague nada.
const (
	toolListIncidents  = "list_incidents"
	toolGetIncident    = "get_incident"
	toolExplainSuspect = "explain_suspect"
)

const timeLayout = time.RFC3339Nano

// toolDefinitions descreve as ferramentas para o cliente.
//
// A descrição é o que um modelo lê para decidir quando chamar cada uma, então
// ela diz também o que a ferramenta não faz. Sem isso, "get_incident" convida à
// suposição de que o Faultmap investigaria sob demanda.
func toolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name": toolListIncidents,
			"description": "Lista os diagnósticos já registrados, do mais recente para o mais antigo. " +
				"Não executa investigação: devolve apenas o que o Faultmap já analisou.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{
						"type": "integer", "minimum": 1, "maximum": 100,
						"description": "quantidade máxima de incidentes devolvidos",
					},
				},
			},
		},
		{
			"name": toolGetIncident,
			"description": "Devolve o diagnóstico completo de um incidente: janelas, hipóteses com " +
				"evidência e proveniência, suspeitos ranqueados e as limitações declaradas. " +
				"É uma leitura do snapshot; nenhum detector roda de novo.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"incident_id": map[string]any{
						"type": "string", "description": "identificador do incidente, como inc_abc123",
					},
				},
				"required": []string{"incident_id"},
			},
		},
		{
			"name": toolExplainSuspect,
			"description": "Explica por que um suspeito foi apontado em um incidente: cada parcela do " +
				"score, as evidências que a sustentam e as hipóteses registradas que não pontuaram.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"incident_id": map[string]any{"type": "string", "description": "identificador do incidente"},
					"suspect": map[string]any{
						"type": "string", "description": "nome do serviço ou identificador do commit suspeito",
					},
				},
				"required": []string{"incident_id", "suspect"},
			},
		},
	}
}

// decodeArguments aceita argumentos ausentes como objeto vazio: um cliente que
// chama uma tool sem parâmetros opcionais não está errado.
func decodeArguments(arguments json.RawMessage, destination any) error {
	if len(arguments) == 0 || string(arguments) == "null" {
		return nil
	}
	if err := json.Unmarshal(arguments, destination); err != nil {
		return fmt.Errorf("argumentos inválidos: %w", err)
	}
	return nil
}

// describeFindings expõe a evidência e o que aquele padrão costuma significar.
//
// A frase de causas comuns viaja junto por decisão da ADR 0013: uma medida
// sozinha não evoca a lista que alguém experiente teria de imediato, e um
// modelo lendo apenas "p95 subiu de 3 ms para 150 ms" chuta tão mal quanto a
// pessoa que, no piloto cego, formulou hipótese de SQL injection diante desse
// número.
func describeFindings(findings []detection.Finding) []map[string]any {
	described := make([]map[string]any, 0, len(findings))
	for _, finding := range findings {
		evidence := make([]map[string]any, 0, len(finding.Evidence))
		for _, item := range finding.Evidence {
			evidence = append(evidence, map[string]any{
				"summary":        item.Summary,
				"signal_ids":     item.SignalIDs,
				"change_ids":     item.ChangeIDs,
				"baseline_value": item.BaselineValue,
				"incident_value": item.IncidentValue,
			})
		}
		entry := map[string]any{
			"rule_id":     finding.Rule,
			"score":       finding.Score,
			"confidence":  string(finding.Confidence),
			"evidence":    evidence,
			"limitations": finding.Limitations,
		}
		if causes := strings.TrimSpace(detection.CommonCauses(finding.Rule)); causes != "" {
			entry["common_causes"] = causes
		}
		described = append(described, entry)
	}
	return described
}

// marshalIndented produz saída estável e legível. As chaves de um mapa são
// ordenadas pelo encoder do Go, então a mesma entrada devolve os mesmos bytes —
// a promessa que o resto do produto faz.
func marshalIndented(payload any) (string, error) {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serializar resposta: %w", err)
	}
	return string(encoded), nil
}

// toolError devolve a falha como resultado da tool, e não como erro de
// protocolo: o modelo consegue corrigir um argumento errado sozinho.
func toolError(cause error) map[string]any {
	return map[string]any{
		"isError": true,
		"content": []any{map[string]any{"type": "text", "text": cause.Error()}},
	}
}
