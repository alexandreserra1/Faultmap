package privacy

import (
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestPolicyRemoveAtributosBloqueados cobre o defeito encontrado com telemetria
// real: a lista de atributos bloqueados existia na configuração mas nunca era
// aplicada, e o SQL das consultas de uma aplicação de terceiros foi gravado
// integralmente no banco.
func TestPolicyRemoveAtributosBloqueados(t *testing.T) {
	t.Parallel()

	policy := NewPolicy([]string{"db.statement", "db.query.text", "http.request.body"}, 512)
	signals := policy.Apply([]domain.Signal{{
		ID: "sinal-1",
		Attributes: map[string]string{
			"db.system":     "duckdb",
			"db.statement":  "SELECT user_id, name, password_hash FROM auth_users WHERE email = ?",
			"db.query.text": "SELECT segredo FROM cofre",
			"http.route":    "/api/v1/injuries",
		},
	}})

	atributos := signals[0].Attributes
	for _, bloqueado := range []string{"db.statement", "db.query.text"} {
		if _, presente := atributos[bloqueado]; presente {
			t.Fatalf("atributo bloqueado %q foi preservado", bloqueado)
		}
	}
	if atributos["db.system"] != "duckdb" || atributos["http.route"] != "/api/v1/injuries" {
		t.Fatalf("atributos legítimos foram removidos: %#v", atributos)
	}
}

// TestPolicyTruncaAtributosLongos protege memória e cardinalidade sem descartar
// o atributo inteiro, que ainda pode sustentar uma evidência.
func TestPolicyTruncaAtributosLongos(t *testing.T) {
	t.Parallel()

	policy := NewPolicy(nil, 16)
	signals := policy.Apply([]domain.Signal{{
		ID:         "sinal-1",
		Attributes: map[string]string{"span.name": strings.Repeat("a", 100)},
	}})

	if tamanho := len(signals[0].Attributes["span.name"]); tamanho != 16 {
		t.Fatalf("tamanho após truncamento = %d, esperado 16", tamanho)
	}
}

// TestPolicyIgnoraDiferençaDeCaixaEEspaços evita que uma variação de escrita na
// configuração deixe passar um atributo sensível.
func TestPolicyIgnoraDiferençaDeCaixaEEspaços(t *testing.T) {
	t.Parallel()

	policy := NewPolicy([]string{"  DB.Statement  "}, 512)
	signals := policy.Apply([]domain.Signal{{
		ID:         "sinal-1",
		Attributes: map[string]string{"db.statement": "SELECT 1"},
	}})

	if _, presente := signals[0].Attributes["db.statement"]; presente {
		t.Fatal("atributo bloqueado escapou por diferença de caixa")
	}
}

// TestPolicySemLimiteNãoTrunca garante que um limite ausente não zere valores.
func TestPolicySemLimiteNãoTrunca(t *testing.T) {
	t.Parallel()

	policy := NewPolicy(nil, 0)
	signals := policy.Apply([]domain.Signal{{
		ID:         "sinal-1",
		Attributes: map[string]string{"span.name": "GET /rota"},
	}})

	if signals[0].Attributes["span.name"] != "GET /rota" {
		t.Fatalf("valor alterado sem limite configurado: %q", signals[0].Attributes["span.name"])
	}
}
