package detection

import (
	"fmt"
	"strings"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
	"github.com/faultmap/faultmap/internal/telemetry/semconv"
)

// DetectDatabaseError compara, entre baseline e incidente, a proporção de
// operações de banco que falharam por algo que não é timeout: conexão recusada,
// erro de query, resposta inválida ou transação abortada.
//
// O detector reporta apenas a mudança entre as janelas. Um sistema que sempre
// falha na mesma proporção não produz hipótese: aquilo é o estado normal dele, e
// nada no incidente o distingue.
func DetectDatabaseError(input Input) (Finding, bool) {
	baseline := filterDatabaseSignals(input.Baseline)
	incident := filterDatabaseSignals(input.Incident)
	if len(baseline) == 0 || len(incident) == 0 {
		return Finding{}, false
	}

	baselineErrors := databaseNonTimeoutFailures(baseline)
	incidentErrors := databaseNonTimeoutFailures(incident)
	if len(incidentErrors) == 0 {
		return Finding{}, false
	}

	baselineRate := fraction(len(baselineErrors), len(baseline))
	incidentRate := fraction(len(incidentErrors), len(incident))
	delta := incidentRate - baselineRate
	if delta <= 0 {
		return Finding{}, false
	}
	// Sem esta barreira, uma falha a mais em vinte operações vira hipótese de
	// regressão — foi assim que uma release publicada acusou ruído de amostragem.
	if !exceedsSamplingNoise(baselineRate, len(baseline), incidentRate, len(incident), delta) {
		return Finding{}, false
	}

	systems := strings.Join(databaseSystems(incidentErrors), ", ")
	finding := newFinding(
		RuleDatabaseError,
		input.ServiceName,
		delta,
		sampleConfidence(len(baseline), len(incident)),
		[]Evidence{{
			Summary: fmt.Sprintf(
				"%d de %d operações %s falharam sem indício de timeout%s; a baseline tinha %d de %d, ou seja, a taxa passou de %.2f%% para %.2f%%.",
				len(incidentErrors),
				len(incident),
				systems,
				databaseFailureTypeSuffix(incidentErrors),
				len(baselineErrors),
				len(baseline),
				baselineRate*100,
				incidentRate*100,
			),
			SignalIDs:     signalIDs(incidentErrors),
			BaselineValue: baselineRate,
			IncidentValue: incidentRate,
		}},
		len(baseline),
		len(incident),
	)
	finding.Limitations = append(finding.Limitations, "A falha foi observada no cliente do banco; ela pode ter origem na aplicação, na rede ou no servidor, e a telemetria coletada não distingue os três.")
	return finding, true
}

// databaseFailureTypeSuffix nomeia os tipos de falha observados quando a
// instrumentação os declara. Spans que só trazem status de erro não têm tipo, e
// nesse caso a evidência prefere ficar sem o trecho a inventar um rótulo.
func databaseFailureTypeSuffix(signals []domain.Signal) string {
	types := make(map[string]struct{})
	for _, signal := range signals {
		if failureType := semconv.FailureType(signal.Attributes); failureType != "" {
			types[failureType] = struct{}{}
		}
	}
	if len(types) == 0 {
		return ""
	}
	return " (" + strings.Join(sortedSetValues(types), ", ") + ")"
}

// databaseNonTimeoutFailures separa o território deste detector do de
// database_timeout: sem a exclusão, o mesmo span seria contado por duas regras e
// apareceria duas vezes no relatório como se fossem achados independentes.
func databaseNonTimeoutFailures(signals []domain.Signal) []domain.Signal {
	timeoutIDs := make(map[string]struct{})
	for _, signal := range databaseTimeouts(signals) {
		timeoutIDs[signal.ID] = struct{}{}
	}
	failures := make([]domain.Signal, 0, len(signals))
	for _, signal := range databaseFailures(signals) {
		if _, isTimeout := timeoutIDs[signal.ID]; isTimeout {
			continue
		}
		if isClientCancellation(signal) {
			continue
		}
		failures = append(failures, signal)
	}
	return failures
}

// isClientCancellation reconhece a operação interrompida porque quem chamou
// desistiu, e não porque o banco falhou.
//
// Quando um serviço acima abandona a chamada — por timeout próprio ou por
// política de retry —, as operações de banco abaixo são canceladas e a
// instrumentação as marca como erro. Contá-las como falha do banco atribui ao
// serviço de baixo um problema que nasceu acima dele, que é justamente o tipo de
// acusação equivocada que este produto existe para evitar.
//
// Cancelamento provocado por esgotamento de tempo continua coberto por
// database_timeout, que o reconhece pelo texto antes desta exclusão.
func isClientCancellation(signal domain.Signal) bool {
	failure := strings.ToLower(attributeValueOrEmpty(signal))
	return strings.Contains(failure, "cancel")
}

// attributeValueOrEmpty reúne o tipo e a mensagem da falha em um único texto,
// porque as instrumentações registram o cancelamento ora em um, ora no outro.
func attributeValueOrEmpty(signal domain.Signal) string {
	return semconv.FailureType(signal.Attributes) + " " + semconv.FailureMessage(signal.Attributes)
}
