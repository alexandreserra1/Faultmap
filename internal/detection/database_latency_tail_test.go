package detection

import (
	"fmt"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestDatabaseLatencyTailAcusaOLockQueOP95NãoEnxerga é a regressão do piloto
// cego: o produto ficou calado diante de um `ACCESS EXCLUSIVE` real.
//
// Os números vêm da medição, não de invenção. Na rodada 6 do sorteio, das 120
// operações de banco da janela, cinco esperaram o lock por dois segundos e
// nenhuma falhou — logo todas entraram na conta do detector de p95. Mesmo
// assim ele se calou, porque 5 de 120 é 4,2% e o p95 caiu em 21,11 ms: a cauda
// inteira vive acima do percentil escolhido, por uma observação.
//
// Um lock bloqueia quem colide com ele enquanto é mantido, o que numa janela
// curta é sempre uma minoria. Essa é a forma que a falha tem por natureza, e
// não uma coincidência da rodada: detectar virava função de quanto da carga
// esbarrou no lock, não de o lock existir.
func TestDatabaseLatencyTailAcusaOLockQueOP95NãoEnxerga(t *testing.T) {
	t.Parallel()

	baseline := bancoComLatência(120, 0.5)
	incidente := bancoComCauda(115, 0.5, 5, 1999.6)

	// Primeiro o fato que motiva a regra nova: o detector de p95 não vê isto.
	if _, found := DetectDatabaseLatencyDelta(Input{
		ServiceName: "payment-service", Baseline: baseline, Incident: incidente,
	}); found {
		t.Fatal("premissa mudou: o p95 agora enxerga a cauda, e esta regra perdeu o motivo")
	}

	finding, found := DetectDatabaseLatencyTail(Input{
		ServiceName: "payment-service", Baseline: baseline, Incident: incidente,
	})
	if !found {
		t.Fatal("detector silenciou cinco operações de dois segundos contra uma baseline de 0,5 ms")
	}
	if finding.Rule != RuleDatabaseLatencyTail {
		t.Fatalf("regra = %q", finding.Rule)
	}
	if len(finding.Evidence) == 0 || len(finding.Evidence[0].SignalIDs) == 0 {
		t.Fatal("finding sem proveniência: a evidência precisa citar os sinais de origem")
	}
	// A proveniência precisa apontar as operações da cauda, não a janela inteira:
	// quem investiga quer os cinco spans que esperaram, não os 120.
	if quantidade := len(finding.Evidence[0].SignalIDs); quantidade != 5 {
		t.Fatalf("proveniência cita %d sinais, esperado exatamente as 5 operações da cauda", quantidade)
	}
}

// TestDatabaseLatencyTailIgnoraCaudaQueJáExistiaNaBaseline mantém a regra
// comparativa, como todas as outras deste pacote: um banco que sempre tem
// algumas operações lentas descreve o sistema, não o incidente.
func TestDatabaseLatencyTailIgnoraCaudaQueJáExistiaNaBaseline(t *testing.T) {
	t.Parallel()

	mesmaForma := func() []domain.Signal { return bancoComCauda(115, 0.5, 5, 1999.6) }
	if finding, found := DetectDatabaseLatencyTail(Input{
		ServiceName: "payment-service", Baseline: mesmaForma(), Incident: mesmaForma(),
	}); found {
		t.Fatalf("detector acusou uma cauda que já existia na baseline: %s", finding.Evidence[0].Summary)
	}
}

// TestDatabaseLatencyTailExigeCaudaDesproporcional cobre o meio-termo: a cauda
// da baseline não precisa ser zero para a do incidente contar, mas precisa ser
// claramente maior. Só "existir" não basta.
func TestDatabaseLatencyTailExigeCaudaDesproporcional(t *testing.T) {
	t.Parallel()

	casos := []struct {
		nome                            string
		baselineLentas, incidenteLentas int
		esperaAcusação                  bool
	}{
		{nome: "cauda triplicou", baselineLentas: 4, incidenteLentas: 16, esperaAcusação: true},
		{nome: "cauda apenas cresceu um pouco", baselineLentas: 4, incidenteLentas: 6, esperaAcusação: false},
		{nome: "cauda encolheu", baselineLentas: 10, incidenteLentas: 4, esperaAcusação: false},
	}

	for _, caso := range casos {
		caso := caso
		t.Run(caso.nome, func(t *testing.T) {
			t.Parallel()

			_, found := DetectDatabaseLatencyTail(Input{
				ServiceName: "payment-service",
				Baseline:    bancoComCauda(120-caso.baselineLentas, 0.5, caso.baselineLentas, 1999.6),
				Incident:    bancoComCauda(120-caso.incidenteLentas, 0.5, caso.incidenteLentas, 1999.6),
			})
			if found != caso.esperaAcusação {
				t.Fatalf("acusou = %v, esperado %v", found, caso.esperaAcusação)
			}
		})
	}
}

// TestDatabaseLatencyTailExigeMaisDeUmaOperação impede que um único span lento
// sustente uma hipótese. Uma operação é anedota; o pacote já trata amostra
// pequena com o mesmo cuidado em minimumSampleSize.
func TestDatabaseLatencyTailExigeMaisDeUmaOperação(t *testing.T) {
	t.Parallel()

	for _, lentas := range []int{1, 2} {
		lentas := lentas
		t.Run(fmt.Sprintf("%d operação(ões)", lentas), func(t *testing.T) {
			t.Parallel()

			if _, found := DetectDatabaseLatencyTail(Input{
				ServiceName: "payment-service",
				Baseline:    bancoComLatência(120, 0.5),
				Incident:    bancoComCauda(120-lentas, 0.5, lentas, 1999.6),
			}); found {
				t.Fatalf("detector sustentou uma hipótese com %d operação(ões) lenta(s)", lentas)
			}
		})
	}
}

// TestDatabaseLatencyTailIgnoraOscilaçãoAbaixoDoPiso protege contra o defeito
// irmão deste detector, já registrado neste projeto: em durações pequenas, o
// p95 dispara com o sistema saudável porque aquecimento de processo e
// escalonamento produzem múltiplos com facilidade. A razão sozinha repetiria
// isso — 30 vezes 0,5 ms ainda são 15 ms, que não sustentam hipótese nenhuma.
func TestDatabaseLatencyTailIgnoraOscilaçãoAbaixoDoPiso(t *testing.T) {
	t.Parallel()

	if finding, found := DetectDatabaseLatencyTail(Input{
		ServiceName: "payment-service",
		Baseline:    bancoComLatência(120, 0.5),
		Incident:    bancoComCauda(110, 0.5, 10, 15),
	}); found {
		t.Fatalf("detector acusou oscilação de 15 ms: %s", finding.Evidence[0].Summary)
	}
}

// TestDatabaseLatencyTailJulgaACaudaContraONormalDaquelaBase é o teste que
// faltava, descoberto por mutação: sem ele, a razão de vinte vezes podia ser
// afrouxada para duas sem que nenhum teste reclamasse, porque em todos os
// outros casos o piso absoluto de cem milissegundos decidia sozinho.
//
// O que ele protege: num banco cujo normal já é de dezenas de milissegundos, o
// piso absoluto é baixo demais para significar qualquer coisa, e um pico de
// quatro vezes é a operação cara de sempre. Sem a razão, toda base normalmente
// lenta acusaria a si mesma a cada janela.
func TestDatabaseLatencyTailJulgaACaudaContraONormalDaquelaBase(t *testing.T) {
	t.Parallel()

	baseLenta := bancoComLatência(120, 50)

	if finding, found := DetectDatabaseLatencyTail(Input{
		ServiceName: "payment-service",
		Baseline:    baseLenta,
		Incident:    bancoComCauda(115, 50, 5, 200),
	}); found {
		t.Fatalf("detector acusou 200 ms num banco cujo normal é 50 ms: %s", finding.Evidence[0].Summary)
	}

	// A mesma base, com uma cauda que é de fato desproporcional ao normal dela.
	if _, found := DetectDatabaseLatencyTail(Input{
		ServiceName: "payment-service",
		Baseline:    baseLenta,
		Incident:    bancoComCauda(115, 50, 5, 3000),
	}); !found {
		t.Fatal("detector silenciou uma cauda sessenta vezes acima do normal da base")
	}
}

// TestDatabaseLatencyTailIgnoraOperaçõesQueFalharam mantém a fronteira que o
// detector irmão já respeita: o que falhou é explicado por database_timeout e
// database_error, e contá-lo aqui daria duas hipóteses para o mesmo fato.
func TestDatabaseLatencyTailIgnoraOperaçõesQueFalharam(t *testing.T) {
	t.Parallel()

	incidente := bancoComCauda(115, 0.5, 5, 1999.6)
	for indice := 115; indice < len(incidente); indice++ {
		incidente[indice].Severity = "error"
		incidente[indice].Attributes["error.type"] = "timeout"
	}

	if finding, found := DetectDatabaseLatencyTail(Input{
		ServiceName: "payment-service", Baseline: bancoComLatência(120, 0.5), Incident: incidente,
	}); found {
		t.Fatalf("detector duplicou o que o timeout já explica: %s", finding.Evidence[0].Summary)
	}
}

// bancoComCauda monta uma janela em que a maioria das operações é rápida e uma
// minoria é drasticamente mais lenta — a forma que um lock produz.
func bancoComCauda(rápidas int, msRápida float64, lentas int, msLenta float64) []domain.Signal {
	signals := bancoComLatência(rápidas, msRápida)
	for index := 0; index < lentas; index++ {
		signals = append(signals, domain.Signal{
			ID:          fmt.Sprintf("db-cauda-%03d", index),
			ServiceName: "strideredge-api",
			Attributes: map[string]string{
				"db.system":    "duckdb",
				"db.operation": "SELECT",
			},
			Measurements: map[string]float64{"duration_ms": msLenta},
		})
	}
	return signals
}

// TestDatabaseLatencyTailChegaAoRun fecha o caminho que faltava: um detector
// que existe e não é chamado por Run() é código morto que passa nos próprios
// testes. Esta regra nasceu justamente de uma omissão silenciosa, e repeti-la
// aqui seria irônico.
func TestDatabaseLatencyTailChegaAoRun(t *testing.T) {
	t.Parallel()

	// O nome precisa bater com o dos sinais: Run() filtra por serviço antes de
	// chamar os detectores, e um nome divergente esvazia as duas janelas.
	findings := Run(Input{
		ServiceName: "strideredge-api",
		Baseline:    bancoComLatência(120, 0.5),
		Incident:    bancoComCauda(115, 0.5, 5, 1999.6),
	})
	for _, finding := range findings {
		if finding.Rule == RuleDatabaseLatencyTail {
			return
		}
	}
	t.Fatalf("Run() não produziu %q; regras vistas: %v", RuleDatabaseLatencyTail, regrasDe(findings))
}

func regrasDe(findings []Finding) []string {
	regras := make([]string, 0, len(findings))
	for _, finding := range findings {
		regras = append(regras, finding.Rule)
	}
	return regras
}

// TestDatabaseLatencyTailNãoAfirmaQueAsDemaisSeguiramNormais protege contra uma
// frase falsa que a matriz E2E revelou: quando a degradação é uniforme, o
// relatório dizia "8 de 8 operações levaram 758 ms ou mais ... as demais
// seguiram normais". Não havia demais.
//
// O produto não pode afirmar o que não observou. A cláusula só cabe quando de
// fato sobrou uma maioria normal.
func TestDatabaseLatencyTailNãoAfirmaQueAsDemaisSeguiramNormais(t *testing.T) {
	t.Parallel()

	uniforme, found := DetectDatabaseLatencyTail(Input{
		ServiceName: "payment-service",
		Baseline:    bancoComLatência(20, 10),
		Incident:    bancoComCauda(0, 0, 20, 758),
	})
	if !found {
		t.Fatal("detector silenciou uma degradação uniforme")
	}
	if strings.Contains(uniforme.Evidence[0].Summary, "as demais") {
		t.Fatalf("relatório afirma algo sobre operações que não existem: %s", uniforme.Evidence[0].Summary)
	}

	minoria, found := DetectDatabaseLatencyTail(Input{
		ServiceName: "payment-service",
		Baseline:    bancoComLatência(120, 0.5),
		Incident:    bancoComCauda(115, 0.5, 5, 1999.6),
	})
	if !found {
		t.Fatal("detector silenciou a cauda minoritária")
	}
	if !strings.Contains(minoria.Evidence[0].Summary, "as demais") {
		t.Fatalf("relatório omitiu que a maioria seguiu normal, que é a informação que separa cauda de degradação: %s",
			minoria.Evidence[0].Summary)
	}
}
