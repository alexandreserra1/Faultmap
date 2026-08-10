package detection

import (
	"fmt"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestDetectTraceBreakReportaLigacaoQuePerdida cobre o único caso legítimo:
// dois serviços que estavam ligados por traces na baseline e deixaram de estar
// no incidente, com ambos ainda emitindo telemetria.
func TestDetectTraceBreakReportaLigacaoQuePerdida(t *testing.T) {
	t.Parallel()

	baseline := []domain.Signal{
		spanSignal("b1", "cart-service", "trace-b1", "span-b1", "", ""),
		spanSignal("b2", "checkout-service", "trace-b1", "span-b2", "span-b1", ""),
		spanSignal("b3", "cart-service", "trace-b2", "span-b3", "", ""),
		spanSignal("b4", "checkout-service", "trace-b2", "span-b4", "span-b3", ""),
	}
	incident := []domain.Signal{
		spanSignal("i1", "cart-service", "trace-i1", "span-i1", "", ""),
		spanSignal("i2", "checkout-service", "trace-i2", "span-i2", "", ""),
	}

	findings := DetectTraceBreak(baseline, incident)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, esperado apenas um", findings)
	}
	finding := findings[0]
	if finding.Rule != RuleTraceBreak || finding.ServiceName != "checkout-service" {
		t.Fatalf("finding = %#v", finding)
	}
	summary := finding.Evidence[0].Summary
	if !strings.Contains(summary, "cart-service") || !strings.Contains(summary, "2") {
		t.Fatalf("resumo = %q, esperado citar a ligação e os números", summary)
	}
	if finding.Evidence[0].BaselineValue != 2 || finding.Evidence[0].IncidentValue != 0 {
		t.Fatalf("valores = %#v", finding.Evidence[0])
	}
	if !containsLimitation(finding.Limitations, "instrumenta") {
		t.Fatalf("limitações = %#v, esperado ressalva sobre instrumentação", finding.Limitations)
	}
}

// TestDetectTraceBreakIgnoraLacunaPermanente é a proteção central do detector:
// instrumentação parcial produz spans órfãos nas duas janelas e isso descreve o
// sistema, não o incidente.
func TestDetectTraceBreakIgnoraLacunaPermanente(t *testing.T) {
	t.Parallel()

	baseline := []domain.Signal{
		spanSignal("b1", "cart-service", "trace-b1", "span-b1", "", ""),
		spanSignal("b2", "checkout-service", "trace-b1", "span-b2", "", ""),
	}
	incident := []domain.Signal{
		spanSignal("i1", "cart-service", "trace-i1", "span-i1", "", ""),
		spanSignal("i2", "checkout-service", "trace-i1", "span-i2", "", ""),
	}

	if findings := DetectTraceBreak(baseline, incident); len(findings) != 0 {
		t.Fatalf("findings = %#v, esperado silêncio para lacuna permanente", findings)
	}
}

// TestDetectTraceBreakIgnoraServicoSemTelemetriaNoIncidente evita descrever
// como perda de contexto o caso em que o serviço simplesmente parou de aparecer.
func TestDetectTraceBreakIgnoraServicoSemTelemetriaNoIncidente(t *testing.T) {
	t.Parallel()

	baseline := []domain.Signal{
		spanSignal("b1", "cart-service", "trace-b1", "span-b1", "", ""),
		spanSignal("b2", "checkout-service", "trace-b1", "span-b2", "span-b1", ""),
	}
	incident := []domain.Signal{
		spanSignal("i1", "cart-service", "trace-i1", "span-i1", "", ""),
	}

	if findings := DetectTraceBreak(baseline, incident); len(findings) != 0 {
		t.Fatalf("findings = %#v, esperado silêncio sem telemetria do downstream", findings)
	}
}

// TestDetectTraceBreakIgnoraLigacaoPreservada garante que uma ligação ainda
// observada no incidente não produza hipótese.
func TestDetectTraceBreakIgnoraLigacaoPreservada(t *testing.T) {
	t.Parallel()

	baseline := []domain.Signal{
		spanSignal("b1", "cart-service", "trace-b1", "span-b1", "", ""),
		spanSignal("b2", "checkout-service", "trace-b1", "span-b2", "span-b1", ""),
	}
	incident := []domain.Signal{
		spanSignal("i1", "cart-service", "trace-i1", "span-i1", "", ""),
		spanSignal("i2", "checkout-service", "trace-i1", "span-i2", "span-i1", ""),
	}

	if findings := DetectTraceBreak(baseline, incident); len(findings) != 0 {
		t.Fatalf("findings = %#v, esperado silêncio com ligação preservada", findings)
	}
}

// TestTraceBreakPesaAImportânciaDaLigaçãoPerdida distingue uma ligação central
// de uma marginal. Perder a ligação que sustentava quase todos os spans do
// serviço é evidência muito mais forte do que perder uma que aparecia
// raramente, e um score fixo trataria as duas como idênticas — num detector que
// já nasce sob suspeita de falso positivo, isso é caro demais.
func TestTraceBreakPesaAImportânciaDaLigaçãoPerdida(t *testing.T) {
	t.Parallel()

	central := DetectTraceBreak(
		ligacaoBaseline("checkout-service", "payment-service", 20, 20),
		spansSoltos("checkout-service", "payment-service", 20),
	)
	marginal := DetectTraceBreak(
		ligacaoBaseline("checkout-service", "payment-service", 2, 20),
		spansSoltos("checkout-service", "payment-service", 20),
	)

	if len(central) != 1 || len(marginal) != 1 {
		t.Fatalf("findings = %d e %d, esperado 1 em cada", len(central), len(marginal))
	}
	if central[0].Score <= marginal[0].Score {
		t.Fatalf("ligação central pontuou %.2f e a marginal %.2f; a central precisa pesar mais",
			central[0].Score, marginal[0].Score)
	}
}

// TestTraceBreakExplicaAmostraPequena garante que a limitação de volume seja
// declarada como nos demais detectores, e não omitida por construção manual.
func TestTraceBreakExplicaAmostraPequena(t *testing.T) {
	t.Parallel()

	findings := DetectTraceBreak(
		ligacaoBaseline("checkout-service", "payment-service", 2, 2),
		spansSoltos("checkout-service", "payment-service", 2),
	)
	if len(findings) != 1 {
		t.Fatalf("findings = %d, esperado 1", len(findings))
	}
	if findings[0].Confidence != ConfidenceLow {
		t.Fatalf("confiança = %q, esperado baixa com duas ligações", findings[0].Confidence)
	}
	explicou := false
	for _, limitacao := range findings[0].Limitations {
		if strings.Contains(limitacao, "Amostra pequena") {
			explicou = true
		}
	}
	if !explicou {
		t.Fatalf("a limitação de amostra pequena não foi declarada: %v", findings[0].Limitations)
	}
}

// ligacaoBaseline monta uma janela em que ligados spans do filho apontam para o
// pai, dentro de um total de spans do filho.
func ligacaoBaseline(parent, child string, ligados, total int) []domain.Signal {
	signals := make([]domain.Signal, 0, total*2)
	for index := 0; index < total; index++ {
		trace := fmt.Sprintf("trace-%03d", index)
		paiID := fmt.Sprintf("pai-%03d", index)
		signals = append(signals, domain.Signal{
			ID: parent + "-" + trace, ServiceName: parent, TraceID: trace, SpanID: paiID,
			Attributes: map[string]string{},
		})
		filho := domain.Signal{
			ID: child + "-" + trace, ServiceName: child, TraceID: trace,
			SpanID: fmt.Sprintf("filho-%03d", index), Attributes: map[string]string{},
		}
		if index < ligados {
			filho.Attributes["span.parent_id"] = paiID
		}
		signals = append(signals, filho)
	}
	return signals
}

// spansSoltos monta uma janela em que os dois serviços seguem emitindo, mas
// nenhum span do filho declara pai.
func spansSoltos(parent, child string, total int) []domain.Signal {
	signals := make([]domain.Signal, 0, total*2)
	for index := 0; index < total; index++ {
		trace := fmt.Sprintf("trace-inc-%03d", index)
		signals = append(signals,
			domain.Signal{ID: parent + "-i-" + trace, ServiceName: parent, TraceID: trace,
				SpanID: fmt.Sprintf("pai-i-%03d", index), Attributes: map[string]string{}},
			domain.Signal{ID: child + "-i-" + trace, ServiceName: child, TraceID: trace,
				SpanID: fmt.Sprintf("filho-i-%03d", index), Attributes: map[string]string{}},
		)
	}
	return signals
}
