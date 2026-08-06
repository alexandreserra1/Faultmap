// Package demoruntime reúne fronteiras operacionais pequenas compartilhadas
// pelos processos da demonstração, sem misturar regras dos serviços.
package demoruntime

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Lookup representa uma fonte de configuração compatível com os.LookupEnv.
type Lookup func(string) (string, bool)

// Environment converte variáveis de ambiente em valores já validados.
type Environment struct {
	lookup Lookup
}

// NewEnvironment cria um leitor injetável para permitir testes sem alterar o
// ambiente global do processo.
func NewEnvironment(lookup Lookup) Environment {
	return Environment{lookup: lookup}
}

// Value retorna o valor não vazio ou o default explícito.
func (environment Environment) Value(key, fallback string) string {
	value, found := environment.lookup(key)
	if !found || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

// Required exige um valor não vazio para dependências sem default seguro.
func (environment Environment) Required(key string) (string, error) {
	value, found := environment.lookup(key)
	if !found || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s é obrigatório", key)
	}
	return strings.TrimSpace(value), nil
}

// Int converte um inteiro e aplica limites inclusivos antes do bootstrap.
func (environment Environment) Int(key string, fallback, minimum, maximum int) (int, error) {
	raw := environment.Value(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s deve ser inteiro: %w", key, err)
	}
	if value < minimum || value > maximum {
		return 0, fmt.Errorf("%s deve estar entre %d e %d", key, minimum, maximum)
	}
	return value, nil
}

// Port valida o intervalo reservado a portas TCP.
func (environment Environment) Port(key string, fallback int) (int, error) {
	return environment.Int(key, fallback, 1, 65535)
}

// Duration converte uma duração e controla se zero é aceito pelo caso de uso.
func (environment Environment) Duration(key string, fallback time.Duration, allowZero bool) (time.Duration, error) {
	raw := environment.Value(key, fallback.String())
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s deve ser uma duração: %w", key, err)
	}
	if value < 0 || (!allowZero && value == 0) {
		return 0, fmt.Errorf("%s deve ser %s", key, map[bool]string{true: "não negativo", false: "positivo"}[allowZero])
	}
	return value, nil
}
