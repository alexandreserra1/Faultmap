package detection

import (
	"fmt"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

const (
	// minimumDatabaseLatencyDeltaMilliseconds é o menor aumento de p95 de banco
	// que ainda interessa. O piso é mais baixo que o de latência HTTP porque uma
	// requisição costuma disparar várias consultas: um acréscimo pequeno em cada
	// uma se acumula na resposta que a pessoa percebe.
	minimumDatabaseLatencyDeltaMilliseconds = 2
	// minimumDatabaseLatencyRatio exige que a duração pelo menos dobre. O piso
	// baixo sozinho deixaria passar a oscilação normal de um banco local, que
	// trabalha na casa de frações de milissegundo; exigir o dobro torna o
	// aumento inequívoco.
	minimumDatabaseLatencyRatio = 1.0
	// confiableDatabaseLatencyMilliseconds é a partir de quanto o p95 do
	// incidente sustenta uma afirmação de confiança alta.
	//
	// O número vem de medição, não de gosto. Em janelas comprovadamente
	// saudáveis deste projeto — sete do medidor de ruído e três de um
	// experimento de fases — o p95 do incidente chegou a 17 ms sem que nada
	// tivesse sido injetado. Quatro acusações registradas com "confiança alta"
	// caíram todas abaixo disso: 5,20, 12, 12,3 e 17,04 ms.
	//
	// Acima do piso, o detector segue afirmando alta: as degradações reais já
	// medidas ficaram em 630 ms ou mais, duas ordens de grandeza acima. O número
	// é da demo e do piloto, e deve ser revisto quando houver medição de ruído de
	// um banco que não seja o nosso.
	confiableDatabaseLatencyMilliseconds = 20
)

// DetectDatabaseLatencyDelta compara a duração p95 das operações de banco entre
// as janelas.
//
// Os outros dois detectores de banco procuram falha — timeout e erro. Um banco
// que degrada sem falhar não era percebido por nenhum deles, e o diagnóstico
// mostrava apenas a latência HTTP que a degradação arrastava: quem investigava
// via "a API ficou lenta" sem ver "o banco ficou lento", que é a informação que
// aponta onde mexer. O caso apareceu em uma aplicação real na primeira carga
// gerada contra ela.
//
// Como os demais, a regra é comparativa: um banco sempre lento descreve o
// sistema, não o incidente.
func DetectDatabaseLatencyDelta(input Input) (Finding, bool) {
	// Só operações que concluíram entram na conta. Uma que falhou é lenta por
	// definição, e medi-la aqui repetiria o que database_timeout e
	// database_error já explicam, com duas hipóteses para o mesmo fato.
	baseline := succeededDatabaseSignals(input.Baseline)
	incident := succeededDatabaseSignals(input.Incident)
	if len(baseline) == 0 || len(incident) == 0 {
		return Finding{}, false
	}

	baselineP95, baselineOK := percentile95(baseline)
	incidentP95, incidentOK := percentile95(incident)
	if !baselineOK || !incidentOK || incidentP95 <= baselineP95 {
		return Finding{}, false
	}
	if !exceedsDatabaseLatencyNoise(baselineP95, incidentP95) {
		return Finding{}, false
	}

	systems := strings.Join(databaseSystems(incident), ", ")
	if systems == "" {
		systems = "banco de dados"
	}

	// A confiança olhava só o tamanho da amostra, então 120 operações bastavam
	// para "alta" ainda que o efeito medisse três milissegundos. Foi assim que
	// este detector acusou quatro vezes, com confiança alta, um sistema que
	// ninguém havia quebrado. Amostra e magnitude são fraquezas independentes, e
	// a ressalva precisa dizer qual delas é — mandar coletar mais dados quando já
	// há 120 sinais por janela desperdiça o tempo de quem investiga.
	confidence := sampleConfidence(len(baseline), len(incident))
	magnitudeDuvidosa := incidentP95 < confiableDatabaseLatencyMilliseconds
	if magnitudeDuvidosa {
		confidence = ConfidenceLow
	}

	finding := newFinding(
		RuleDatabaseLatencyDelta,
		input.ServiceName,
		(incidentP95-baselineP95)/incidentP95,
		confidence,
		[]Evidence{{
			Summary: fmt.Sprintf(
				"A duração p95 das operações %s aumentou de %.2f ms para %.2f ms, sem timeout nem erro observados.",
				systems, baselineP95, incidentP95,
			),
			SignalIDs:     signalIDs(incident),
			BaselineValue: baselineP95,
			IncidentValue: incidentP95,
		}},
		len(baseline),
		len(incident),
	)
	if magnitudeDuvidosa {
		finding.Limitations = append(finding.Limitations, fmt.Sprintf(
			"Magnitude pequena: o p95 do incidente ficou em %.2f ms, abaixo dos %d ms a partir dos quais um aumento se distingue do que um sistema saudável produz nesta demo; trate como indício, não como medida.",
			incidentP95, confiableDatabaseLatencyMilliseconds,
		))
	}
	return finding, true
}

// exceedsDatabaseLatencyNoise decide se o aumento é relevante nas duas escalas,
// absoluta e proporcional.
//
// A troca é deliberada: um banco que sai de 0,3 ms para 1,5 ms quintuplicou,
// mas o acréscimo é pequeno demais para sustentar uma hipótese sozinho, e
// acusá-lo repetiria o falso positivo que a latência HTTP já produziu neste
// projeto. Uma degradação real e pequena passa despercebida por isso.
func exceedsDatabaseLatencyNoise(baselineP95, incidentP95 float64) bool {
	delta := incidentP95 - baselineP95
	if delta < minimumDatabaseLatencyDeltaMilliseconds {
		return false
	}
	if baselineP95 <= 0 {
		return true
	}
	return delta/baselineP95 >= minimumDatabaseLatencyRatio
}

// succeededDatabaseSignals devolve as operações de banco que não falharam.
func succeededDatabaseSignals(signals []domain.Signal) []domain.Signal {
	database := filterDatabaseSignals(signals)
	failed := make(map[string]struct{})
	for _, signal := range databaseFailures(database) {
		failed[signal.ID] = struct{}{}
	}
	succeeded := make([]domain.Signal, 0, len(database))
	for _, signal := range database {
		if _, isFailure := failed[signal.ID]; isFailure {
			continue
		}
		succeeded = append(succeeded, signal)
	}
	return succeeded
}
