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

// catálogoDeFalhasReais lista classes de falha do catálogo do OpenTelemetry
// Demo — 15 falhas injetáveis de um projeto que não é nosso, em um sistema de
// mais de vinte serviços poliglotas.
//
// Confrontar as nossas frases com uma taxonomia alheia responde a pergunta que
// o piloto deixou: as causas que listamos correspondem ao que quebra de
// verdade, ou inventamos um vocabulário que só serve aos nossos cenários?
//
// A verificação é por regra, e não por busca no texto inteiro. A primeira
// tentativa procurou as palavras em todas as frases juntas e reportou cobertura
// falsa: "cache" aparecia em version_regression falando de aquecimento após
// deploy, "fila" aparecia em trace_break falando de propagação de contexto. A
// palavra existia no lugar errado.
var catálogoDeFalhasReais = []struct {
	falha  string
	regra  string
	termos []string
}{
	{"adHighCpu — CPU saturada", RuleLatencyDelta, []string{"contenção"}},
	{"intlShippingSlowdown — downstream lento", RuleLatencyDelta, []string{"dependência lenta"}},
	{"loadGeneratorVUs — carga aumentada", RuleLatencyDelta, []string{"carga"}},
	{"adManualGc — pausa de coleta de lixo", RuleLatencyDelta, []string{"coleta de lixo"}},
	{"recommendationCacheFailure — cache parou de servir", RuleLatencyDelta, []string{"cache"}},
	{"failedReadinessProbe — instância fora de rotação", RuleLatencyDelta, []string{"capacidade"}},
	{"kafkaQueueProblems — acúmulo em fila", RuleLatencyDelta, []string{"fila"}},
	{"paymentUnreachable — serviço fora do ar", RuleErrorRateDelta, []string{"indisponível"}},
	{"productCatalogFailure — erro em entrada específica", RuleErrorRateDelta, []string{"entrada inesperada"}},
	{"emailMemoryLeak — vazamento de memória", RuleErrorRateDelta, []string{"memória"}},
}

// TestCausasComunsCobremCatálogoDeTerceiro exige que cada classe de falha real
// apareça na frase da regra que dispararia para ela.
func TestCausasComunsCobremCatálogoDeTerceiro(t *testing.T) {
	t.Parallel()

	for _, caso := range catálogoDeFalhasReais {
		frase := strings.ToLower(CommonCauses(caso.regra))
		encontrado := false
		for _, termo := range caso.termos {
			if strings.Contains(frase, strings.ToLower(termo)) {
				encontrado = true
				break
			}
		}
		if !encontrado {
			t.Errorf("a frase de %s não orienta sobre %q\n  frase: %s", caso.regra, caso.falha, frase)
		}
	}
}
