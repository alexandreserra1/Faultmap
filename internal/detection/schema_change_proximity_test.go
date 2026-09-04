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
