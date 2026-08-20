package detection

import (
	"strings"
	"testing"
)

// todasAsRegras reúne as regras que o produto sabe produzir. Manter a lista
// aqui, e não derivá-la do código, é o que faz o teste falhar quando alguém
// acrescenta um detector sem dizer o que aquele padrão costuma significar.
var todasAsRegras = []string{
	RuleErrorRateDelta,
	RuleLatencyDelta,
	RuleDatabaseTimeout,
	RuleTraceCorrelation,
	RuleDeploymentProximity,
	RuleRetryStorm,
	RuleDependencyFailure,
	RuleTraceBreak,
	RuleLogCorrelation,
	RuleDatabaseLatencyDelta,
	RuleDatabaseError,
	RuleVersionRegression,
}

// TestTodaRegraDizOQueAquelePadrãoCostumaSignificar nasce de um resultado do
// piloto cego: diante de "o p95 do banco aumentou de 3,54 ms para 150,73 ms", a
// pessoa que investigava formulou a hipótese de SQL injection. A medida estava
// certa e a natureza da causa não foi comunicada.
//
// Uma medida sozinha não evoca a lista que alguém experiente teria de imediato —
// lock, saturação de pool, consulta sem índice. Sem ela o relatório é um
// termômetro: mostra a febre e não sugere o que investigar.
func TestTodaRegraDizOQueAquelePadrãoCostumaSignificar(t *testing.T) {
	t.Parallel()

	for _, regra := range todasAsRegras {
		causas := CommonCauses(regra)
		if strings.TrimSpace(causas) == "" {
			t.Errorf("regra %q não diz o que aquele padrão costuma significar", regra)
		}
	}
}

// TestCausasComunsOferecemAlternativas protege a promessa central do produto:
// ele não afirma causalidade. Uma frase com uma única causa leria como
// veredito; duas ou mais deixam explícito que são hipóteses a investigar.
func TestCausasComunsOferecemAlternativas(t *testing.T) {
	t.Parallel()

	for _, regra := range todasAsRegras {
		causas := CommonCauses(regra)
		if !strings.Contains(causas, " ou ") {
			t.Errorf("regra %q apresenta causa única, o que leria como veredito: %q", regra, causas)
		}
		for _, proibido := range []string{"foi causado", "a causa é", "porque o"} {
			if strings.Contains(strings.ToLower(causas), proibido) {
				t.Errorf("regra %q afirma causalidade com %q: %q", regra, proibido, causas)
			}
		}
	}
}

// TestCausasComunsIgnoraRegraDesconhecida garante que uma regra vinda de um
// snapshot antigo ou de outra versão não derrube a renderização.
func TestCausasComunsIgnoraRegraDesconhecida(t *testing.T) {
	t.Parallel()

	if causas := CommonCauses("regra_que_nunca_existiu"); causas != "" {
		t.Fatalf("regra desconhecida devolveu %q", causas)
	}
}
