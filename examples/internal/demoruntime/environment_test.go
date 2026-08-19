package demoruntime

import (
	"testing"
	"time"
)

// TestEnvironmentValidaValoresObrigatorios cobre a fronteira entre variáveis
// de ambiente, defaults explícitos e tipos usados pelo bootstrap.
func TestEnvironmentValidaValoresObrigatorios(t *testing.T) {
	values := map[string]string{
		"NAME":     "checkout-service",
		"PORT":     "8080",
		"TIMEOUT":  "2s",
		"ATTEMPTS": "3",
	}
	environment := NewEnvironment(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})

	if got, err := environment.Required("NAME"); err != nil || got != "checkout-service" {
		t.Fatalf("Required() = %q, %v", got, err)
	}
	if got, err := environment.Port("PORT", 0); err != nil || got != 8080 {
		t.Fatalf("Port() = %d, %v", got, err)
	}
	if got, err := environment.Duration("TIMEOUT", 0, false); err != nil || got != 2*time.Second {
		t.Fatalf("Duration() = %s, %v", got, err)
	}
	if got, err := environment.Int("ATTEMPTS", 1, 1, 10); err != nil || got != 3 {
		t.Fatalf("Int() = %d, %v", got, err)
	}
}

// TestEnvironmentRejeitaConfiguracaoInsegura impede que o processo comece
// com portas, durações ou limites fora do contrato operacional.
func TestEnvironmentRejeitaConfiguracaoInsegura(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		check  func(Environment) error
	}{
		{name: "obrigatória vazia", values: map[string]string{"VALUE": "  "}, check: func(env Environment) error { _, err := env.Required("VALUE"); return err }},
		{name: "porta inválida", values: map[string]string{"VALUE": "70000"}, check: func(env Environment) error { _, err := env.Port("VALUE", 8080); return err }},
		{name: "duração negativa", values: map[string]string{"VALUE": "-1s"}, check: func(env Environment) error { _, err := env.Duration("VALUE", time.Second, true); return err }},
		{name: "inteiro acima do limite", values: map[string]string{"VALUE": "11"}, check: func(env Environment) error { _, err := env.Int("VALUE", 1, 1, 10); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			environment := NewEnvironment(func(key string) (string, bool) { value, ok := test.values[key]; return value, ok })
			if err := test.check(environment); err == nil {
				t.Fatal("esperava erro de validação")
			}
		})
	}
}
