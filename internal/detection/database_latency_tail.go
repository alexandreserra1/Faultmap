package detection

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

const (
	// minimumDatabaseTailRatio é quantas vezes acima do pior caso normal uma
	// operação precisa estar para ser considerada cauda.
	//
	// A regra irmã aceita o dobro como aumento relevante, mas ali o dobro vale
	// para a janela inteira; aqui a afirmação se apoia num punhado de
	// observações, e o critério precisa ser mais severo na mesma proporção em
	// que a amostra é menor. Vinte vezes o p95 da baseline está uma ordem de
	// grandeza além do que escalonamento e coleta de lixo produzem.
	minimumDatabaseTailRatio = 20
	// minimumDatabaseTailMilliseconds é o piso absoluto, que existe porque a
	// razão sozinha não protege em durações pequenas.
	//
	// Um banco local trabalha na casa de frações de milissegundo, onde vinte
	// vezes ainda são dez milissegundos — e este projeto já registrou quatro
	// falsos positivos do detector de p95 exatamente nessa escala, todos com
	// valor de incidente abaixo de vinte milissegundos. Abaixo de cem, uma
	// operação lenta é mais provavelmente o processo do que o banco.
	minimumDatabaseTailMilliseconds = 100
	// minimumDatabaseTailOperations impede que uma anedota vire hipótese. Uma
	// operação lenta acontece; três seguem o mesmo motivo.
	minimumDatabaseTailOperations = 3
	// minimumDatabaseTailGrowth exige que a cauda do incidente seja
	// desproporcional à da baseline, e não apenas maior. Uma cauda que já
	// existia descreve o sistema, como em todas as regras deste pacote.
	minimumDatabaseTailGrowth = 2
)

// DetectDatabaseLatencyTail acusa a minoria de operações de banco que ficou
// drasticamente mais lenta enquanto a maioria seguiu normal.
//
// Existe porque o percentil 95 não enxerga essa forma de falha, e a omissão foi
// medida, não suposta. Num piloto cego contra um `ACCESS EXCLUSIVE` real, cinco
// de cento e vinte operações esperaram dois segundos, nenhuma falhou — logo
// todas entraram na conta — e o detector de p95 ficou calado, porque a cauda
// inteira vivia acima do percentil, por uma observação.
//
// Não é calibragem de piso: um lock bloqueia quem colide com ele enquanto é
// mantido, o que numa janela curta é sempre uma minoria. Essa é a forma que a
// falha tem por natureza, e qualquer percentil alto o bastante para ser estável
// vai deixá-la passar. A pergunta precisa mudar, não o limiar — de "a janela
// inteira ficou mais lenta?" para "alguém esperou muito além do normal?".
//
// A regra divide a classe de peso `database_evidence` com as outras três de
// banco. Uma degradação uniforme dispara esta e a de p95 ao mesmo tempo, o que é
// correto — as duas afirmações são verdadeiras —, e o teto por classe da ADR
// 0010 impede que o mesmo fato seja pago duas vezes.
func DetectDatabaseLatencyTail(input Input) (Finding, bool) {
	// A mesma fronteira da regra irmã: o que falhou é explicado por
	// database_timeout e database_error, e contá-lo aqui daria duas hipóteses
	// para o mesmo fato.
	baseline := succeededDatabaseSignals(input.Baseline)
	incident := succeededDatabaseSignals(input.Incident)
	if len(baseline) == 0 || len(incident) == 0 {
		return Finding{}, false
	}

	baselineP95, baselineOK := percentile95(baseline)
	if !baselineOK {
		return Finding{}, false
	}
	threshold := math.Max(baselineP95*minimumDatabaseTailRatio, minimumDatabaseTailMilliseconds)

	incidentTail := signalsSlowerThan(incident, threshold)
	if len(incidentTail) < minimumDatabaseTailOperations {
		return Finding{}, false
	}
	// A comparação é por proporção, e não por contagem: as duas janelas podem
	// ter volumes diferentes, e contar cru faria a janela maior parecer pior só
	// por ser maior.
	baselineShare := float64(len(signalsSlowerThan(baseline, threshold))) / float64(len(baseline))
	incidentShare := float64(len(incidentTail)) / float64(len(incident))
	if incidentShare <= baselineShare*minimumDatabaseTailGrowth {
		return Finding{}, false
	}

	// A mediana da cauda, e não o máximo: um único span extremo não deve
	// definir a magnitude que o relatório afirma.
	tailMedian := medianDuration(incidentTail)
	systems := strings.Join(databaseSystems(incidentTail), ", ")
	if systems == "" {
		systems = "banco de dados"
	}

	// A cláusula sobre a maioria só é dita quando existe uma maioria. Numa
	// degradação uniforme não sobra nada de normal, e afirmar que sobrou seria o
	// produto descrevendo o que não observou.
	resto := ""
	if len(incidentTail) < len(incident) {
		resto = "; as demais seguiram normais"
	}

	return newFinding(
		RuleDatabaseLatencyTail,
		input.ServiceName,
		// A mesma forma de score da regra irmã, para que as duas sejam
		// comparáveis dentro da classe que compartilham.
		(tailMedian-baselineP95)/tailMedian,
		sampleConfidence(len(baseline), len(incident)),
		[]Evidence{{
			Summary: fmt.Sprintf(
				"%d de %d operações %s levaram %.0f ms ou mais, contra um pior caso normal de %.2f ms%s.",
				len(incidentTail), len(incident), systems, tailMedian, baselineP95, resto,
			),
			// A proveniência cita as operações da cauda, não a janela inteira:
			// quem investiga quer os spans que esperaram.
			SignalIDs:     signalIDs(incidentTail),
			BaselineValue: baselineP95,
			IncidentValue: tailMedian,
		}},
		len(baseline),
		len(incident),
	), true
}

// signalsSlowerThan devolve as operações acima do limiar, preservando a ordem
// de entrada para que a proveniência seja estável.
func signalsSlowerThan(signals []domain.Signal, threshold float64) []domain.Signal {
	slower := make([]domain.Signal, 0, len(signals))
	for _, signal := range signals {
		if duration, exists := signal.Measurements["duration_ms"]; exists && duration > threshold {
			slower = append(slower, signal)
		}
	}
	return slower
}

// medianDuration devolve a duração mediana das operações informadas.
func medianDuration(signals []domain.Signal) float64 {
	durations := make([]float64, 0, len(signals))
	for _, signal := range signals {
		if duration, exists := signal.Measurements["duration_ms"]; exists {
			durations = append(durations, duration)
		}
	}
	if len(durations) == 0 {
		return 0
	}
	sort.Float64s(durations)
	return durations[len(durations)/2]
}
