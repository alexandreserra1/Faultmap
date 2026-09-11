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
	// age é a distância até a coleta que revelou a mudança: a hipótese mais
	// favorável, em que ela ocorreu no último instante possível.
	age time.Duration
	// oldestAge é a distância até a coleta anterior: a hipótese mais
	// desfavorável, em que ela ocorreu no primeiro instante possível. É esta
	// que pontua.
	oldestAge  time.Duration
	captureGap time.Duration
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
	databases, tables := databaseTargetsQueriedBy(input.ServiceName, input.Incident)
	if len(databases) == 0 && len(tables) == 0 {
		return Finding{}, false
	}

	candidates := make([]schemaChangeCandidate, 0, len(changes))
	for _, change := range changes {
		if !changeReachesService(change, databases, tables) {
			continue
		}
		// A inclusão usa a ponta otimista: basta que a mudança POSSA ter ocorrido
		// dentro da janela. A pontuação, mais abaixo, usa a pessimista. Incluir
		// com generosidade e pontuar com cautela evita perder uma migração real
		// por causa de uma coleta espaçada, sem pagar por isso em confiança.
		age := incidentStart.Sub(change.ObservedBefore)
		if age < 0 || age > SchemaChangeLookback {
			continue
		}
		candidates = append(candidates, schemaChangeCandidate{
			change:     change,
			age:        age,
			oldestAge:  incidentStart.Sub(change.ObservedAfter),
			captureGap: change.ObservedBefore.Sub(change.ObservedAfter),
		})
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

	databaseSignals := signalsForChange(input.Incident, selected.change)
	// A pontuação sai da ponta pessimista do intervalo. Com a otimista, coletar
	// uma vez por dia rendia o mesmo score que coletar a cada minuto: uma
	// migração de 22 horas antes aparecia como "1 hora antes" só porque foi a
	// coleta seguinte que a revelou. Assim a largura do intervalo custa score
	// por si, e a frequência de coleta dispensa recomendação em documentação —
	// quem coleta mais vezes recebe evidência mais forte, e a matemática explica
	// o porquê sozinha.
	score := clamp(1 - selected.oldestAge.Seconds()/SchemaChangeLookback.Seconds())

	confidence := ConfidenceHigh
	limitations := []string{
		"Proximidade temporal não prova causalidade.",
		"A mudança foi observada entre duas coletas do catálogo; o instante exato não é conhecido.",
	}
	// O que importa não é o intervalo contra a janela de busca, é o intervalo
	// contra a proximidade que se está afirmando. Uma coleta diária afirmando
	// "ocorreu 1 hora antes" não sabe em qual das 23 horas anteriores a mudança
	// de fato ocorreu, e chamar isso de confiança alta seria mentir sobre a
	// precisão. O finding continua sendo emitido: silenciar esconderia uma
	// migração real de quem investiga.
	if selected.captureGap > selected.age {
		confidence = ConfidenceLow
		limitations = append(limitations, fmt.Sprintf(
			"O intervalo entre as coletas foi de %s, maior que a proximidade de %s afirmada: colete o catálogo com mais frequência para estreitar a evidência.",
			roundedDuration(selected.captureGap), roundedDuration(selected.age),
		))
	}
	if len(databaseSignals) < minimumSampleSize {
		confidence = ConfidenceLow
		limitations = append(limitations, fmt.Sprintf(
			"Amostra pequena: %d operações observadas contra o alvo migrado; mínimo recomendado de %d.",
			len(databaseSignals), minimumSampleSize,
		))
	}

	return Finding{
		Rule:        RuleSchemaChangeProximity,
		ServiceName: input.ServiceName,
		Score:       score,
		Confidence:  confidence,
		Evidence: []Evidence{{
			Summary:       schemaChangeSummary(selected),
			ChangeIDs:     []string{selected.change.ID},
			SignalIDs:     signalIDs(databaseSignals),
			IncidentValue: score,
		}},
		Limitations: limitations,
	}, true
}

// databaseTargetsQueriedBy devolve as bases e as tabelas que o serviço tocou na
// janela, segundo a própria telemetria.
//
// São dois conjuntos porque a instrumentação real raramente entrega os dois.
// Medindo uma aplicação instrumentada de verdade, 199 spans de banco traziam
// `db.collection.name` e nenhum trazia `db.namespace`: um detector que exigisse
// o nome da base ficaria permanentemente calado ali, sem erro e sem aviso.
func databaseTargetsQueriedBy(
	serviceName string,
	signals []telemetrydomain.Signal,
) (databases, tables map[string]struct{}) {
	databases = make(map[string]struct{})
	tables = make(map[string]struct{})
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
		if table := strings.TrimSpace(semconv.DatabaseCollection(signal.Attributes)); table != "" {
			tables[table] = struct{}{}
		}
	}
	return databases, tables
}

// changeReachesService decide se a mudança atingiu algo que o serviço usa.
//
// A tabela é o vínculo mais estreito e vem primeiro: uma migração em `payments`
// e um serviço que consulta `payments` é uma ligação mais forte do que "os dois
// usam o mesmo PostgreSQL". O nome da base entra quando a telemetria o traz, ou
// quando a mudança não pertence a tabela alguma.
//
// Nenhum dos dois observado significa silêncio. Aceitar apenas "ambos falam
// PostgreSQL" faria qualquer migração acusar qualquer serviço com problema no
// mesmo horário, que é exatamente a correlação vazia que esta regra evita.
func changeReachesService(
	change changedomain.SchemaChange,
	databases, tables map[string]struct{},
) bool {
	if table := strings.TrimSpace(change.TableName); table != "" {
		if _, queried := tables[table]; queried {
			return true
		}
	}
	if database := strings.TrimSpace(change.DatabaseName); database != "" {
		if _, queried := databases[database]; queried {
			return true
		}
	}
	return false
}

// signalsForChange recolhe a proveniência: os spans que tocaram o alvo migrado.
func signalsForChange(
	signals []telemetrydomain.Signal,
	change changedomain.SchemaChange,
) []telemetrydomain.Signal {
	table := strings.TrimSpace(change.TableName)
	database := strings.TrimSpace(change.DatabaseName)
	filtered := make([]telemetrydomain.Signal, 0, len(signals))
	for _, signal := range signals {
		matchesTable := table != "" && strings.TrimSpace(semconv.DatabaseCollection(signal.Attributes)) == table
		matchesDatabase := database != "" && strings.TrimSpace(semconv.DatabaseName(signal.Attributes)) == database
		if matchesTable || matchesDatabase {
			filtered = append(filtered, signal)
		}
	}
	return filtered
}

// schemaChangeSummary descreve o que foi observado, sem supor o comando que
// produziu aquilo. Duas fotos do catálogo mostram efeito, não DDL.
func schemaChangeSummary(candidate schemaChangeCandidate) string {
	change := candidate.change
	alvo := "da base " + change.DatabaseName
	if table := strings.TrimSpace(change.TableName); table != "" {
		alvo = "da tabela " + table + ", na base " + change.DatabaseName
	}
	// As duas pontas do intervalo vão no texto. Mostrar só a mais favorável
	// sugeriria uma precisão que a comparação de duas fotos do catálogo não tem.
	summary := fmt.Sprintf(
		"O %s %s %s foi %s, entre %s e %s antes do incidente.",
		schemaObjectLabel(change.ObjectKind),
		change.ObjectName,
		alvo,
		schemaChangeLabel(change.ChangeKind),
		roundedDuration(candidate.age),
		roundedDuration(candidate.oldestAge),
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
//
// Abaixo de um minuto o arredondamento produzia "0s", que não informa nada a
// quem lê — apareceu na saída real de uma migração aplicada segundos antes da
// janela. A frase substitui o número justamente onde o número perdeu o sentido.
func roundedDuration(duration time.Duration) string {
	rounded := duration.Round(time.Minute)
	if rounded == 0 {
		return "menos de um minuto"
	}
	// O formato do Go escreve "1m0s" e "2h0m0s"; os zeros à direita só existem
	// porque a unidade menor não foi suprimida, e arrastá-los para o relatório
	// sugere uma precisão de segundos que o arredondamento acabou de descartar.
	texto := rounded.String()
	for _, sufixo := range []string{"m0s", "h0m"} {
		texto = strings.TrimSuffix(texto, sufixo[1:])
	}
	return texto
}
