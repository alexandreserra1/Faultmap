package detection

import (
	"fmt"
	"sort"
	"strings"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	telemetrydomain "github.com/faultmap/faultmap/internal/telemetry/domain"
	"github.com/faultmap/faultmap/internal/telemetry/semconv"
)

// SchemaChangeLookback limita quanto tempo antes do incidente uma mudança de
// catálogo ainda sustenta uma hipótese.
//
// São 24 horas, contra uma hora do deployment, e a diferença é deliberada. Um
// deploy que quebra costuma quebrar de imediato, porque o código novo passa a
// atender no mesmo minuto. Uma migração raramente quebra quando roda: ela
// quebra quando o código que a pressupõe encontra o schema, ou quando o volume
// cresce o bastante para que a falta do índice apareça. Uma janela de uma hora
// perderia justamente o caso comum.
const SchemaChangeLookback = 24 * time.Hour

// detectSchemaChangeProximity é a forma interna, sem o vínculo com o serviço.
type schemaChangeCandidate struct {
	change changedomain.SchemaChange
	age    time.Duration
}

// DetectSchemaChangeProximity acusa o serviço quando uma mudança recente de
// catálogo atingiu uma base que ele consulta.
//
// O vínculo com a base é o que separa esta regra de uma coincidência: sem
// exigir que o serviço tenha spans contra aquela base na janela, qualquer
// migração em qualquer banco acusaria qualquer serviço que estivesse com
// problema no mesmo horário. A telemetria já carrega o nome da base, e é ela
// quem responde de quem é a dependência — não uma configuração declarada.
//
// O sujeito acusado é o serviço, e não a mudança de schema. O commit ganhou
// tipo próprio porque já tinha identidade estável e legível; uma mudança de
// catálogo ainda não provou que merece o mesmo custo em ranking, renderização e
// leitura de snapshots antigos. O objeto migrado vai no texto da evidência, que
// é o que o `explain suspect` imprime.
func DetectSchemaChangeProximity(
	input Input,
	changes []changedomain.SchemaChange,
	incidentStart time.Time,
) (Finding, bool) {
	databases := databasesQueriedBy(input.ServiceName, input.Incident)
	if len(databases) == 0 {
		return Finding{}, false
	}

	candidates := make([]schemaChangeCandidate, 0, len(changes))
	for _, change := range changes {
		if _, queried := databases[strings.TrimSpace(change.DatabaseName)]; !queried {
			continue
		}
		age := incidentStart.Sub(change.ObservedBefore)
		if age < 0 || age > SchemaChangeLookback {
			continue
		}
		candidates = append(candidates, schemaChangeCandidate{change: change, age: age})
	}
	if len(candidates) == 0 {
		return Finding{}, false
	}

	// A mais próxima do incidente vence; o ID desempata para que a mesma
	// investigação produza sempre a mesma evidência, independentemente da ordem
	// em que a consulta devolveu as mudanças.
	sort.Slice(candidates, func(first, second int) bool {
		if candidates[first].age != candidates[second].age {
			return candidates[first].age < candidates[second].age
		}
		return candidates[first].change.ID < candidates[second].change.ID
	})
	selected := candidates[0]

	databaseSignals := signalsForDatabase(input.Incident, selected.change.DatabaseName)
	captureGap := selected.change.ObservedBefore.Sub(selected.change.ObservedAfter)

	confidence := ConfidenceHigh
	limitations := []string{
		"Proximidade temporal não prova causalidade.",
		"A mudança foi observada entre duas coletas do catálogo; o instante exato não é conhecido.",
	}
	// Um intervalo entre coletas maior que a própria janela de busca torna a
	// proximidade quase sem informação: a mudança pode ter acontecido logo antes
	// do incidente ou muito antes dele, e o produto não tem como distinguir.
	// Ainda assim o finding é emitido, com ressalva — silenciar esconderia uma
	// migração real de quem investiga.
	if captureGap > SchemaChangeLookback {
		confidence = ConfidenceLow
		limitations = append(limitations, fmt.Sprintf(
			"O intervalo entre as duas coletas foi de %s, maior que a janela de busca de %s: a proximidade com o incidente é fraca.",
			roundedDuration(captureGap), roundedDuration(SchemaChangeLookback),
		))
	}
	if len(databaseSignals) < minimumSampleSize {
		confidence = ConfidenceLow
		limitations = append(limitations, fmt.Sprintf(
			"Amostra pequena: %d operações observadas contra a base %s; mínimo recomendado de %d.",
			len(databaseSignals), selected.change.DatabaseName, minimumSampleSize,
		))
	}

	return Finding{
		Rule:        RuleSchemaChangeProximity,
		ServiceName: input.ServiceName,
		Score:       clamp(1 - selected.age.Seconds()/SchemaChangeLookback.Seconds()),
		Confidence:  confidence,
		Evidence: []Evidence{{
			Summary:       schemaChangeSummary(selected),
			ChangeIDs:     []string{selected.change.ID},
			SignalIDs:     signalIDs(databaseSignals),
			IncidentValue: clamp(1 - selected.age.Seconds()/SchemaChangeLookback.Seconds()),
		}},
		Limitations: limitations,
	}, true
}

// databasesQueriedBy devolve as bases que o serviço consultou na janela,
// segundo a própria telemetria.
func databasesQueriedBy(serviceName string, signals []telemetrydomain.Signal) map[string]struct{} {
	databases := make(map[string]struct{})
	for _, signal := range signals {
		if serviceName != "" && signal.ServiceName != serviceName {
			continue
		}
		if databaseSystem(signal.Attributes) == "" {
			continue
		}
		if name := strings.TrimSpace(semconv.DatabaseName(signal.Attributes)); name != "" {
			databases[name] = struct{}{}
		}
	}
	return databases
}

func signalsForDatabase(signals []telemetrydomain.Signal, databaseName string) []telemetrydomain.Signal {
	filtered := make([]telemetrydomain.Signal, 0, len(signals))
	for _, signal := range signals {
		if strings.TrimSpace(semconv.DatabaseName(signal.Attributes)) == strings.TrimSpace(databaseName) {
			filtered = append(filtered, signal)
		}
	}
	return filtered
}

// schemaChangeSummary descreve o que foi observado, sem supor o comando que
// produziu aquilo. Duas fotos do catálogo mostram efeito, não DDL.
func schemaChangeSummary(candidate schemaChangeCandidate) string {
	change := candidate.change
	summary := fmt.Sprintf(
		"O %s %s da base %s foi %s, observado entre coletas até %s antes do incidente.",
		schemaObjectLabel(change.ObjectKind),
		change.ObjectName,
		change.DatabaseName,
		schemaChangeLabel(change.ChangeKind),
		roundedDuration(candidate.age),
	)
	if detail := strings.TrimSpace(change.Detail); detail != "" {
		summary += " " + strings.ToUpper(detail[:1]) + detail[1:] + "."
	}
	return summary
}

func schemaObjectLabel(kind changedomain.SchemaObjectKind) string {
	switch kind {
	case changedomain.SchemaObjectTable:
		return "objeto tabela"
	case changedomain.SchemaObjectColumn:
		return "objeto coluna"
	case changedomain.SchemaObjectIndex:
		return "objeto índice"
	case changedomain.SchemaObjectConstraint:
		return "objeto restrição"
	default:
		return "objeto"
	}
}

func schemaChangeLabel(kind changedomain.SchemaChangeKind) string {
	switch kind {
	case changedomain.SchemaChangeAdded:
		return "adicionado"
	case changedomain.SchemaChangeRemoved:
		return "removido"
	case changedomain.SchemaChangeAltered:
		return "alterado"
	default:
		return "modificado"
	}
}

// roundedDuration arredonda para minuto porque a precisão de segundos sugeriria
// um conhecimento do instante que esta regra explicitamente não tem.
func roundedDuration(duration time.Duration) string {
	return duration.Round(time.Minute).String()
}
