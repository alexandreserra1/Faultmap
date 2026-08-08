// Package privacy aplica a política de atributos configurada antes de a
// telemetria ser persistida.
//
// A política existia apenas como configuração validada: nenhum ponto do código
// a consultava. Telemetria de uma aplicação real revelou o efeito — o SQL das
// consultas foi gravado por inteiro no banco, contrariando a regra de não
// armazenar SQL bruto por padrão. A filtragem vive aqui, entre a normalização e
// a persistência, para valer igualmente para arquivos e para o receiver OTLP.
package privacy

import (
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// Policy decide quais atributos podem ser persistidos e com qual tamanho máximo.
type Policy struct {
	blocked            map[string]struct{}
	maxAttributeLength int
}

// NewPolicy constrói a política a partir dos valores já validados da configuração.
// A comparação de nomes ignora caixa e espaços para que uma variação de escrita
// no YAML não deixe passar um atributo sensível.
func NewPolicy(blockedAttributes []string, maxAttributeLength int) Policy {
	blocked := make(map[string]struct{}, len(blockedAttributes))
	for _, attribute := range blockedAttributes {
		normalized := strings.ToLower(strings.TrimSpace(attribute))
		if normalized == "" {
			continue
		}
		blocked[normalized] = struct{}{}
	}
	return Policy{blocked: blocked, maxAttributeLength: maxAttributeLength}
}

// Apply remove atributos bloqueados e trunca valores longos, devolvendo os
// sinais prontos para persistência. Os sinais são alterados no lugar: eles já
// pertencem a este fluxo de ingestão e não são compartilhados com outro dono.
func (policy Policy) Apply(signals []domain.Signal) []domain.Signal {
	for index := range signals {
		attributes := signals[index].Attributes
		if attributes == nil {
			continue
		}
		for key, value := range attributes {
			if _, blocked := policy.blocked[strings.ToLower(strings.TrimSpace(key))]; blocked {
				delete(attributes, key)
				continue
			}
			if policy.maxAttributeLength > 0 && len(value) > policy.maxAttributeLength {
				attributes[key] = value[:policy.maxAttributeLength]
			}
		}
	}
	return signals
}
