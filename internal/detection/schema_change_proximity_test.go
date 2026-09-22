package detection

import (
	"fmt"
	"strings"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

var incidenteEm = time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)

// spansDeBanco produz sinais de banco de um serviço contra uma base nomeada,
// que é o vínculo exigido pelo detector.
func spansDeBanco(prefixo, servico, base string, total int) []domain.Signal {
	signals := make([]domain.Signal, 0, total)
	for index := range total {
		signals = append(signals, domain.Signal{
			ID:          fmt.Sprintf("%s-%d", prefixo, index),
			ServiceName: servico,
			Timestamp:   incidenteEm.Add(time.Duration(index) * time.Second),
			Attributes: map[string]string{
				"db.system.name":    "postgresql",
				"db.namespace":      base,
				"db.operation.name": "SELECT",
			},
			Measurements: map[string]float64{"duration_ms": 12},
		})
	}
	return signals
}

func mudancaDeSchema(base string, idade time.Duration, intervalo time.Duration) changedomain.SchemaChange {
	observedBefore := incidenteEm.Add(-idade)
	return changedomain.SchemaChange{
		ID:           "schema:" + base + ":index:idx_payments_created_at:removed",
		DatabaseName: base, ObjectKind: changedomain.SchemaObjectIndex,
		ObjectName: "idx_payments_created_at", ChangeKind: changedomain.SchemaChangeRemoved,
		ObservedAfter: observedBefore.Add(-intervalo), ObservedBefore: observedBefore,
	}
}

func entradaComBanco(servico, base string, total int) Input {
	return Input{
		ServiceName: servico,
		Baseline:    spansDeBanco("baseline", servico, base, total),
		Incident:    spansDeBanco("incident", servico, base, total),
	}
}

// TestDetectSchemaChangeProximityAcusaServicoQueFalaComABase cobre o caminho
// principal: uma migração recente numa base que o serviço consulta.
func TestDetectSchemaChangeProximityAcusaServicoQueFalaComABase(t *testing.T) {
	t.Parallel()

	change := mudancaDeSchema("payments", 6*time.Hour, 10*time.Minute)
	finding, found := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{change},
		incidenteEm,
	)

	if !found {
		t.Fatal("DetectSchemaChangeProximity() found = false para mudança dentro da janela")
	}
	if finding.Rule != RuleSchemaChangeProximity || finding.ServiceName != "payment-service" {
		t.Fatalf("finding = %#v", finding)
	}
	// 6h de 24h decorridas deixam 0.75 do decaimento.
	if finding.Score < 0.74 || finding.Score > 0.76 {
		t.Fatalf("score = %v, esperado aproximadamente 0.75", finding.Score)
	}
	if len(finding.Evidence) != 1 || len(finding.Evidence[0].ChangeIDs) != 1 ||
		finding.Evidence[0].ChangeIDs[0] != change.ID {
		t.Fatalf("evidência = %#v", finding.Evidence)
	}
	if !strings.Contains(finding.Evidence[0].Summary, "idx_payments_created_at") ||
		!strings.Contains(finding.Evidence[0].Summary, "payments") {
		t.Fatalf("resumo = %q, esperado nomear o objeto e a base", finding.Evidence[0].Summary)
	}
	if !containsLimitation(finding.Limitations, "causalidade") {
		t.Fatalf("limitações = %#v, esperado aviso causal", finding.Limitations)
	}
	if !containsLimitation(finding.Limitations, "coletas") {
		t.Fatalf("limitações = %#v, esperado a limitação do intervalo entre coletas", finding.Limitations)
	}
}

// TestDetectSchemaChangeProximityIgnoraServicoSemSpanDeBanco impede a acusação
// puramente temporal. Sem span de banco não há vínculo observado entre o
// serviço e a base migrada, e qualquer migração acusaria qualquer serviço.
func TestDetectSchemaChangeProximityIgnoraServicoSemSpanDeBanco(t *testing.T) {
	t.Parallel()

	input := Input{
		ServiceName: "frontend",
		Incident: []domain.Signal{{
			ID: "http-1", ServiceName: "frontend", Timestamp: incidenteEm,
			Attributes: map[string]string{"http.response.status_code": "500"},
		}},
	}

	if _, found := DetectSchemaChangeProximity(
		input,
		[]changedomain.SchemaChange{mudancaDeSchema("payments", time.Hour, time.Minute)},
		incidenteEm,
	); found {
		t.Fatal("DetectSchemaChangeProximity() acusou serviço que não fala com a base")
	}
}

// TestDetectSchemaChangeProximityIgnoraBaseDiferente garante que o vínculo seja
// pela base observada, e não pela mera existência de spans de banco.
func TestDetectSchemaChangeProximityIgnoraBaseDiferente(t *testing.T) {
	t.Parallel()

	if _, found := DetectSchemaChangeProximity(
		entradaComBanco("catalog-service", "catalog", 8),
		[]changedomain.SchemaChange{mudancaDeSchema("payments", time.Hour, time.Minute)},
		incidenteEm,
	); found {
		t.Fatal("DetectSchemaChangeProximity() acusou serviço que fala com outra base")
	}
}

// TestDetectSchemaChangeProximityRespeitaAJanelaDeBusca verifica os dois lados
// do intervalo: mudança velha demais e mudança posterior ao início do incidente.
func TestDetectSchemaChangeProximityRespeitaAJanelaDeBusca(t *testing.T) {
	t.Parallel()

	for nome, idade := range map[string]time.Duration{
		"anterior à janela de busca": SchemaChangeLookback + time.Minute,
		"posterior ao incidente":     -time.Minute,
	} {
		if _, found := DetectSchemaChangeProximity(
			entradaComBanco("payment-service", "payments", 8),
			[]changedomain.SchemaChange{mudancaDeSchema("payments", idade, time.Minute)},
			incidenteEm,
		); found {
			t.Fatalf("%s: DetectSchemaChangeProximity() found = true", nome)
		}
	}
}

// TestDetectSchemaChangeProximityDecaiComADistancia confirma que uma migração
// de ontem pese menos que uma de agora há pouco.
func TestDetectSchemaChangeProximityDecaiComADistancia(t *testing.T) {
	t.Parallel()

	recente, foundRecente := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaDeSchema("payments", time.Hour, time.Minute)},
		incidenteEm,
	)
	antiga, foundAntiga := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaDeSchema("payments", 20*time.Hour, time.Minute)},
		incidenteEm,
	)

	if !foundRecente || !foundAntiga {
		t.Fatal("as duas mudanças estão dentro da janela e deveriam produzir finding")
	}
	if recente.Score <= antiga.Score {
		t.Fatalf("score recente %v não é maior que o antigo %v", recente.Score, antiga.Score)
	}
}

// TestDetectSchemaChangeProximityEscolheAMudancaMaisProxima garante que, entre
// várias migrações na janela, a evidência aponte a mais próxima do incidente —
// e que a escolha não dependa da ordem em que as mudanças chegam.
func TestDetectSchemaChangeProximityEscolheAMudancaMaisProxima(t *testing.T) {
	t.Parallel()

	antiga := mudancaDeSchema("payments", 20*time.Hour, time.Minute)
	antiga.ID = "schema:payments:antiga"
	proxima := mudancaDeSchema("payments", 2*time.Hour, time.Minute)
	proxima.ID = "schema:payments:proxima"

	for _, ordem := range [][]changedomain.SchemaChange{
		{antiga, proxima},
		{proxima, antiga},
	} {
		finding, found := DetectSchemaChangeProximity(
			entradaComBanco("payment-service", "payments", 8), ordem, incidenteEm,
		)
		if !found {
			t.Fatal("DetectSchemaChangeProximity() found = false")
		}
		if finding.Evidence[0].ChangeIDs[0] != proxima.ID {
			t.Fatalf("evidência apontou %q, esperado a mudança mais próxima %q",
				finding.Evidence[0].ChangeIDs[0], proxima.ID)
		}
	}
}

// TestDetectSchemaChangeProximityRebaixaConfiancaComIntervaloLargo cobre a
// consequência honesta da ADR 0014: se as coletas estão muito espaçadas, o
// produto não sabe se a mudança aconteceu perto do incidente ou muito antes.
func TestDetectSchemaChangeProximityRebaixaConfiancaComIntervaloLargo(t *testing.T) {
	t.Parallel()

	estreito, _ := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaDeSchema("payments", time.Hour, 5*time.Minute)},
		incidenteEm,
	)
	largo, found := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaDeSchema("payments", time.Hour, SchemaChangeLookback+time.Hour)},
		incidenteEm,
	)

	if !found {
		t.Fatal("intervalo largo deveria produzir finding com ressalva, não silêncio")
	}
	if estreito.Confidence != ConfidenceHigh {
		t.Fatalf("confiança com intervalo estreito = %q, esperado alta", estreito.Confidence)
	}
	if largo.Confidence != ConfidenceLow {
		t.Fatalf("confiança com intervalo largo = %q, esperado baixa", largo.Confidence)
	}
	if !containsLimitation(largo.Limitations, "intervalo") {
		t.Fatalf("limitações = %#v, esperado ressalva sobre o intervalo entre coletas", largo.Limitations)
	}
}

// TestDetectSchemaChangeProximityRebaixaConfiancaComPoucosSinais aplica ao
// detector o mesmo piso amostral do resto do produto: poucos spans de banco não
// sustentam a afirmação de que o serviço depende daquela base.
func TestDetectSchemaChangeProximityRebaixaConfiancaComPoucosSinais(t *testing.T) {
	t.Parallel()

	finding, found := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 2),
		[]changedomain.SchemaChange{mudancaDeSchema("payments", time.Hour, time.Minute)},
		incidenteEm,
	)

	if !found {
		t.Fatal("DetectSchemaChangeProximity() found = false")
	}
	if finding.Confidence != ConfidenceLow {
		t.Fatalf("confiança = %q, esperado baixa com amostra pequena", finding.Confidence)
	}
}

// TestSchemaChangeProximityTemCausasComuns fecha a ADR 0013 para a regra nova:
// uma medida sozinha não evoca a lista que alguém experiente teria de imediato.
func TestSchemaChangeProximityTemCausasComuns(t *testing.T) {
	t.Parallel()

	if CommonCauses(RuleSchemaChangeProximity) == "" {
		t.Fatal("CommonCauses(RuleSchemaChangeProximity) está vazio")
	}
}

// spansDeBancoComoAInstrumentacaoReal reproduz o que a demo-shop de fato emite,
// medido em 199 spans: sistema, operação e tabela — e nenhum nome de base.
func spansDeBancoComoAInstrumentacaoReal(prefixo, servico, tabela string, total int) []domain.Signal {
	signals := make([]domain.Signal, 0, total)
	for index := range total {
		signals = append(signals, domain.Signal{
			ID:          fmt.Sprintf("%s-%d", prefixo, index),
			ServiceName: servico,
			Timestamp:   incidenteEm.Add(time.Duration(index) * time.Second),
			Attributes: map[string]string{
				"db.system.name":     "postgresql",
				"db.operation.name":  "INSERT",
				"db.collection.name": tabela,
				"span.kind":          "SPAN_KIND_CLIENT",
			},
			Measurements: map[string]float64{"duration_ms": 12},
		})
	}
	return signals
}

// TestDetectSchemaChangeProximityUsaATabelaQuandoNaoHaNomeDeBase é a lição de
// rodar contra uma aplicação instrumentada de verdade.
//
// A demo-shop emite `db.collection.name` e `db.system.name`, e nunca
// `db.namespace` nem `db.name`. Um detector que exigisse o nome da base ficaria
// permanentemente calado ali — sem erro e sem aviso, só "nenhuma anomalia
// encontrada", que é exatamente o defeito que as ADRs 0006 e 0011 registram.
//
// A tabela é um vínculo melhor que a base, não um consolo: uma migração em
// `payments` e um serviço que consulta `payments` é uma ligação mais estreita
// do que "os dois usam o mesmo PostgreSQL".
func TestDetectSchemaChangeProximityUsaATabelaQuandoNaoHaNomeDeBase(t *testing.T) {
	t.Parallel()

	mudanca := changedomain.SchemaChange{
		ID:             "schema:demo:index:public.payments_created_at_id_idx:removed",
		DatabaseName:   "demo",
		TableName:      "payments",
		ObjectKind:     changedomain.SchemaObjectIndex,
		ObjectName:     "public.payments_created_at_id_idx",
		ChangeKind:     changedomain.SchemaChangeRemoved,
		ObservedAfter:  incidenteEm.Add(-3 * time.Hour),
		ObservedBefore: incidenteEm.Add(-2 * time.Hour),
	}
	input := Input{
		ServiceName: "payment-service",
		Baseline:    spansDeBancoComoAInstrumentacaoReal("baseline", "payment-service", "payments", 8),
		Incident:    spansDeBancoComoAInstrumentacaoReal("incident", "payment-service", "payments", 8),
	}

	finding, found := DetectSchemaChangeProximity(input, []changedomain.SchemaChange{mudanca}, incidenteEm)
	if !found {
		t.Fatal("o detector ficou calado com a telemetria que a instrumentação real produz")
	}
	if !strings.Contains(finding.Evidence[0].Summary, "payments") {
		t.Fatalf("resumo = %q, esperado nomear a tabela", finding.Evidence[0].Summary)
	}
}

// TestDetectSchemaChangeProximityIgnoraTabelaDeOutroNome mantém o vínculo
// estreito: consultar outra tabela não é ser afetado pela migração.
func TestDetectSchemaChangeProximityIgnoraTabelaDeOutroNome(t *testing.T) {
	t.Parallel()

	mudanca := changedomain.SchemaChange{
		ID: "schema:demo:column:public.pedidos.valor:altered", DatabaseName: "demo", TableName: "pedidos",
		ObjectKind: changedomain.SchemaObjectColumn, ObjectName: "public.pedidos.valor",
		ChangeKind:     changedomain.SchemaChangeAltered,
		ObservedAfter:  incidenteEm.Add(-2 * time.Hour),
		ObservedBefore: incidenteEm.Add(-time.Hour),
	}
	input := Input{
		ServiceName: "payment-service",
		Incident:    spansDeBancoComoAInstrumentacaoReal("incident", "payment-service", "payments", 8),
	}

	if _, found := DetectSchemaChangeProximity(input, []changedomain.SchemaChange{mudanca}, incidenteEm); found {
		t.Fatal("o detector acusou uma migração em tabela que o serviço não consulta")
	}
}

// TestDetectSchemaChangeProximitySemNomeDeBaseNemTabelaSeCala fecha a porta do
// vínculo frouxo: só "os dois falam PostgreSQL" não liga nada a nada.
func TestDetectSchemaChangeProximitySemNomeDeBaseNemTabelaSeCala(t *testing.T) {
	t.Parallel()

	mudanca := mudancaDeSchema("payments", time.Hour, time.Minute)
	input := Input{
		ServiceName: "payment-service",
		Incident: []domain.Signal{{
			ID: "db-1", ServiceName: "payment-service", Timestamp: incidenteEm,
			Attributes: map[string]string{"db.system.name": "postgresql", "span.kind": "SPAN_KIND_CLIENT"},
		}},
	}

	if _, found := DetectSchemaChangeProximity(input, []changedomain.SchemaChange{mudanca}, incidenteEm); found {
		t.Fatal("o detector acusou sem nenhum vínculo observado além do sistema de banco")
	}
}

// TestSchemaChangeSummaryNaoDiz0s vem de ler a saída real contra a demo-shop:
// uma migração vinte e poucos segundos antes do incidente era arredondada para
// minuto e o texto saía "observado entre coletas até 0s antes do incidente".
// "0s antes" não quer dizer nada para quem lê.
func TestSchemaChangeSummaryNaoDiz0s(t *testing.T) {
	t.Parallel()

	finding, found := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaDeSchema("payments", 26*time.Second, time.Minute)},
		incidenteEm,
	)
	if !found {
		t.Fatal("DetectSchemaChangeProximity() found = false")
	}
	// A asserção é sobre duração zero apresentada como informação, não sobre a
	// substring: "1m0s" contém "0s" e é legítimo.
	for _, ruim := range []string{" 0s ", " 0s.", "entre 0s", "e 0s"} {
		if strings.Contains(finding.Evidence[0].Summary, ruim) {
			t.Fatalf("resumo apresenta duração zero como informação: %q", finding.Evidence[0].Summary)
		}
	}
	if !strings.Contains(finding.Evidence[0].Summary, "menos de um minuto") {
		t.Fatalf("resumo = %q, esperado dizer que foi menos de um minuto", finding.Evidence[0].Summary)
	}
}

// TestIntervaloLargoCustaScore é a resposta à pergunta que o produto não
// respondia: de quanto em quanto tempo coletar o catálogo.
//
// O score usava a ponta otimista do intervalo — o instante da coleta que
// revelou a mudança —, então coletar uma vez por dia produzia a mesma pontuação
// que coletar a cada minuto. Uma migração que rodou 22 horas antes do incidente
// aparecia como "até 1 hora antes", porque foi só na coleta seguinte que ela foi
// vista.
//
// Passando a pontuar pela ponta pessimista, a largura do intervalo custa score
// por si. A frequência de coleta deixa de precisar de recomendação em
// documentação: quem coleta mais vezes recebe evidência mais forte, e a
// matemática explica o porquê sozinha.
func TestIntervaloLargoCustaScore(t *testing.T) {
	t.Parallel()

	estreito, achouEstreito := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaDeSchema("payments", time.Hour, time.Minute)},
		incidenteEm,
	)
	largo, achouLargo := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaDeSchema("payments", time.Hour, 20*time.Hour)},
		incidenteEm,
	)

	if !achouEstreito || !achouLargo {
		t.Fatal("as duas mudanças estão na janela e deveriam ser apresentadas")
	}
	if largo.Score >= estreito.Score {
		t.Fatalf("intervalo de 20h pontuou %v, não menos que o de 1 min (%v)", largo.Score, estreito.Score)
	}
	// Coleta a cada minuto: a incerteza é desprezível e o score continua alto.
	if estreito.Score < 0.94 {
		t.Fatalf("score com intervalo estreito = %v, esperado próximo do máximo", estreito.Score)
	}
}

// TestIntervaloMaiorQueAProximidadeRebaixaConfianca troca um limiar que não
// dizia nada por um que diz.
//
// Antes, a confiança só caía quando o intervalo passava de 24 horas — então uma
// coleta diária afirmava, com confiança alta, que a mudança ocorreu "1 hora
// antes" sem saber em qual das 23 horas anteriores ela realmente ocorreu. O que
// importa não é o intervalo contra a janela de busca, é o intervalo contra a
// proximidade que se está afirmando.
func TestIntervaloMaiorQueAProximidadeRebaixaConfianca(t *testing.T) {
	t.Parallel()

	coletaDiaria, found := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaDeSchema("payments", time.Hour, 23*time.Hour)},
		incidenteEm,
	)
	if !found {
		t.Fatal("DetectSchemaChangeProximity() found = false")
	}
	if coletaDiaria.Confidence != ConfidenceLow {
		t.Fatalf("confiança = %q com intervalo de 23h para proximidade de 1h, esperado baixa",
			coletaDiaria.Confidence)
	}
	if !containsLimitation(coletaDiaria.Limitations, "intervalo") {
		t.Fatalf("limitações = %#v, esperado ressalva sobre o intervalo", coletaDiaria.Limitations)
	}
}

// TestResumoDeclaraALarguraDoIntervalo garante que quem lê saiba o quanto a
// proximidade é precisa, em vez de receber só a ponta mais favorável.
func TestResumoDeclaraALarguraDoIntervalo(t *testing.T) {
	t.Parallel()

	finding, found := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaDeSchema("payments", time.Hour, 6*time.Hour)},
		incidenteEm,
	)
	if !found {
		t.Fatal("DetectSchemaChangeProximity() found = false")
	}
	// Com 6h de intervalo, a mudança pode ter ocorrido entre 1h e 7h antes.
	resumo := finding.Evidence[0].Summary
	if !strings.Contains(resumo, "1h") || !strings.Contains(resumo, "7h") {
		t.Fatalf("resumo = %q, esperado declarar as duas pontas do intervalo", resumo)
	}
}

// mudancaQueAtravessaOInicio monta uma mudança cujo intervalo de observação
// começa antes do incidente e termina depois: a coleta que a revelou já pegou o
// incidente em curso.
//
// É a forma mais comum quando a coleta não é frequente. Com coleta diária, a
// migração das 9h de um incidente das 14h só é vista na coleta do dia seguinte,
// e o intervalo inteiro atravessa o começo do incidente.
func mudancaQueAtravessaOInicio(
	base string, antesDoInicio, depoisDoInicio time.Duration,
) changedomain.SchemaChange {
	change := mudancaDeSchema(base, 0, 0)
	change.ObservedAfter = incidenteEm.Add(-antesDoInicio)
	change.ObservedBefore = incidenteEm.Add(depoisDoInicio)
	return change
}

// TestIntervaloQueAtravessaOInicioEApresentadoComRessalva corrige uma omissão
// que contrariava a própria regra escrita no detector: a inclusão usa a ponta
// otimista do intervalo, e o filtro usava a ponta otimista pelo avesso.
//
// Uma mudança cujo intervalo começa antes do incidente PODE ter ocorrido antes
// dele — é exatamente o caso que a inclusão generosa existe para não perder. A
// ponta que pontua continua sendo a anterior ao incidente, então nada é
// afirmado além do que foi medido.
//
// O que se paga por isso é confiança: parte do intervalo cai dentro do
// incidente, e uma migração aplicada para conter o incidente apareceria aqui do
// mesmo jeito. Isso precisa estar escrito no finding, não na cabeça de quem o
// lê.
func TestIntervaloQueAtravessaOInicioEApresentadoComRessalva(t *testing.T) {
	t.Parallel()

	finding, found := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaQueAtravessaOInicio("payments", 6*time.Hour, 20*time.Minute)},
		incidenteEm,
	)

	if !found {
		t.Fatal("DetectSchemaChangeProximity() ficou calado com intervalo que pode preceder o incidente")
	}
	if finding.Confidence != ConfidenceLow {
		t.Fatalf("confiança = %q, esperado baixa: o intervalo não separa causa de resposta ao incidente",
			finding.Confidence)
	}
	if !containsLimitation(finding.Limitations, "durante o incidente") {
		t.Fatalf("limitações = %#v, esperado dizer que a mudança pode ter ocorrido durante o incidente",
			finding.Limitations)
	}
	if !containsLimitation(finding.Limitations, "resposta ao incidente") {
		t.Fatalf("limitações = %#v, esperado admitir a hipótese de resposta ao incidente",
			finding.Limitations)
	}
	// A ressalva de coleta espaçada afirma uma "proximidade" que aqui não é
	// afirmada: a ponta otimista está dentro do incidente, não antes dele.
	// Emitir as duas faria o finding declarar uma proximidade que o resumo nega.
	if containsLimitation(finding.Limitations, "proximidade de") {
		t.Fatalf("limitações = %#v, esperado não afirmar proximidade que o intervalo não sustenta",
			finding.Limitations)
	}
	// Cada ponta do lado a que pertence: a pessimista precede o incidente, a
	// otimista caiu dentro dele. Trocá-las de lado inverteria o sentido da
	// evidência sem que o texto parecesse errado.
	resumo := finding.Evidence[0].Summary
	if !strings.Contains(resumo, "6h antes do início do incidente") {
		t.Fatalf("resumo = %q, esperado situar a ponta pessimista antes do incidente", resumo)
	}
	if !strings.Contains(resumo, "20m depois dele") {
		t.Fatalf("resumo = %q, esperado dizer o quanto a coleta caiu dentro do incidente", resumo)
	}
}

// TestIntervaloQueAtravessaOInicioPontuaPelaPontaAnteriorAoIncidente mantém a
// ponta pessimista da ADR 0014 como única fonte de score.
//
// A ponta otimista de uma mudança que atravessa o início está dentro do
// incidente, e pontuar por ela daria score máximo justamente ao caso mais
// ambíguo — quanto mais tarde a coleta, mais forte ficaria a acusação. O score
// sai da ponta que precede o incidente, que é a única distância medida.
func TestIntervaloQueAtravessaOInicioPontuaPelaPontaAnteriorAoIncidente(t *testing.T) {
	t.Parallel()

	proxima, achouProxima := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaQueAtravessaOInicio("payments", 6*time.Hour, 20*time.Minute)},
		incidenteEm,
	)
	distante, achouDistante := DetectSchemaChangeProximity(
		entradaComBanco("payment-service", "payments", 8),
		[]changedomain.SchemaChange{mudancaQueAtravessaOInicio("payments", 20*time.Hour, 20*time.Minute)},
		incidenteEm,
	)

	if !achouProxima || !achouDistante {
		t.Fatal("as duas mudanças podem preceder o incidente e deveriam ser apresentadas")
	}
	// 6h de 24h decorridas deixam 0.75 do decaimento, medidos da ponta anterior
	// ao incidente. Pela ponta otimista o score seria 1 nos dois casos.
	if proxima.Score < 0.74 || proxima.Score > 0.76 {
		t.Fatalf("score = %v, esperado aproximadamente 0.75 medido da ponta anterior ao incidente",
			proxima.Score)
	}
	if distante.Score >= proxima.Score {
		t.Fatalf("a mudança que pode ser 20h mais velha pontuou %v, não menos que %v",
			distante.Score, proxima.Score)
	}
}

// TestMudancaInteiramenteDentroDoIncidenteSeCala é uma recusa deliberada, e não
// uma limitação por implementar.
//
// Quando o intervalo inteiro começa depois do início do incidente, todo instante
// possível da mudança é posterior ao começo dele. Não há proximidade anterior a
// afirmar, e as duas leituras — a migração piorou o incidente, ou a migração foi
// a tentativa de contê-lo — são igualmente compatíveis com o que foi observado.
//
// A corroboração da ADR 0014, que protege contra a migração inofensiva, não
// protege aqui: durante o incidente o serviço está sintomático por construção,
// então a exigência de sintoma passa sempre. Acusar seria apontar, com o score
// mais alto possível, quem correu para consertar. Ver ADR 0019.
func TestMudancaInteiramenteDentroDoIncidenteSeCala(t *testing.T) {
	t.Parallel()

	for nome, change := range map[string]changedomain.SchemaChange{
		"começa no instante do incidente": mudancaQueAtravessaOInicio("payments", 0, 20*time.Minute),
		"começa depois do incidente":      mudancaQueAtravessaOInicio("payments", -10*time.Minute, 20*time.Minute),
	} {
		if finding, found := DetectSchemaChangeProximity(
			entradaComBanco("payment-service", "payments", 8),
			[]changedomain.SchemaChange{change},
			incidenteEm,
		); found {
			t.Fatalf("%s: o detector acusou uma mudança que só pode ter ocorrido durante o incidente: %#v",
				nome, finding)
		}
	}
}

// TestMudancaCertamenteAnteriorVenceAQueAtravessaOInicio impede que a inclusão
// nova piore o que já funcionava.
//
// A ordenação escolhe uma mudança só, e a que atravessa o início tem a ponta
// otimista mais próxima de todas — o começo do incidente. Se ela vencesse,
// bastaria existir para rebaixar a confiança de um finding que hoje sai alto
// com uma migração comprovadamente anterior. Evidência sem ambiguidade vem
// primeiro; a ambígua só aparece quando é tudo o que há.
func TestMudancaCertamenteAnteriorVenceAQueAtravessaOInicio(t *testing.T) {
	t.Parallel()

	atravessa := mudancaQueAtravessaOInicio("payments", 2*time.Hour, 20*time.Minute)
	atravessa.ID = "schema:payments:atravessa"
	anterior := mudancaDeSchema("payments", 3*time.Hour, 5*time.Minute)
	anterior.ID = "schema:payments:anterior"

	for _, ordem := range [][]changedomain.SchemaChange{
		{atravessa, anterior},
		{anterior, atravessa},
	} {
		finding, found := DetectSchemaChangeProximity(
			entradaComBanco("payment-service", "payments", 8), ordem, incidenteEm,
		)
		if !found {
			t.Fatal("DetectSchemaChangeProximity() found = false")
		}
		if finding.Evidence[0].ChangeIDs[0] != anterior.ID {
			t.Fatalf("evidência apontou %q, esperado a mudança certamente anterior %q",
				finding.Evidence[0].ChangeIDs[0], anterior.ID)
		}
		if finding.Confidence != ConfidenceHigh {
			t.Fatalf("confiança = %q, esperado alta: a mudança escolhida precede o incidente",
				finding.Confidence)
		}
	}
}

// TestRoundedDurationNaoEngoleAUnidade apareceu ao escrever o resumo de uma
// mudança observada 20 minutos depois do início do incidente: o texto saía
// "2 depois dele".
//
// A supressão dos zeros à direita removia "0m" de qualquer texto terminado em
// "0m", e não só de "h0m": em "20m" o "0m" é o fim do número e a unidade, não um
// zero à direita. O relatório passava a informar "2" onde havia 20 minutos, sem
// unidade nenhuma e uma ordem de grandeza abaixo.
func TestRoundedDurationNaoEngoleAUnidade(t *testing.T) {
	t.Parallel()

	for duracao, esperado := range map[time.Duration]string{
		20 * time.Minute:              "20m",
		10 * time.Minute:              "10m",
		time.Minute:                   "1m",
		2 * time.Hour:                 "2h",
		time.Hour + 20*time.Minute:    "1h20m",
		29 * time.Second:              "menos de um minuto",
		24*time.Hour + 30*time.Minute: "24h30m",
		3*time.Hour + 4*time.Minute:   "3h4m",
	} {
		if texto := roundedDuration(duracao); texto != esperado {
			t.Errorf("roundedDuration(%s) = %q, esperado %q", duracao, texto, esperado)
		}
	}
}

// TestEntreAmbiguasVenceAQueInvadiuMenosOIncidente decide o desempate que sobra
// quando todas as mudanças atravessam o começo do incidente.
//
// Quanto mais a coleta avançou para dentro do incidente, mais do intervalo é
// compatível com uma migração aplicada em resposta a ele. Escolher a que invadiu
// menos é escolher a que menos depende dessa hipótese. Sem este critério a
// escolha cairia no ID, que ordena por acaso.
func TestEntreAmbiguasVenceAQueInvadiuMenosOIncidente(t *testing.T) {
	t.Parallel()

	// O ID da que invadiu mais vem antes no alfabeto: se o desempate caísse no
	// ID, seria ela a escolhida.
	invadiuMuito := mudancaQueAtravessaOInicio("payments", 5*time.Hour, 3*time.Hour)
	invadiuMuito.ID = "schema:payments:a-invadiu-muito"
	invadiuPouco := mudancaQueAtravessaOInicio("payments", 2*time.Hour, 5*time.Minute)
	invadiuPouco.ID = "schema:payments:b-invadiu-pouco"

	for _, ordem := range [][]changedomain.SchemaChange{
		{invadiuMuito, invadiuPouco},
		{invadiuPouco, invadiuMuito},
	} {
		finding, found := DetectSchemaChangeProximity(
			entradaComBanco("payment-service", "payments", 8), ordem, incidenteEm,
		)
		if !found {
			t.Fatal("DetectSchemaChangeProximity() found = false")
		}
		if finding.Evidence[0].ChangeIDs[0] != invadiuPouco.ID {
			t.Fatalf("evidência apontou %q, esperado a que invadiu menos o incidente %q",
				finding.Evidence[0].ChangeIDs[0], invadiuPouco.ID)
		}
	}
}
