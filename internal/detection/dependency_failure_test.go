package detection

import (
	"fmt"
	"strings"
	"testing"

	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// spanSignal monta um sinal de span com o mínimo que os detectores de topologia
// leem, para que os testes descrevam apenas a cadeia observada.
func spanSignal(id, service, traceID, spanID, parentID, severity string) domain.Signal {
	attributes := map[string]string{}
	if parentID != "" {
		attributes["span.parent_id"] = parentID
	}
	return domain.Signal{
		ID:          id,
		Type:        domain.SignalTypeSpan,
		ServiceName: service,
		TraceID:     traceID,
		SpanID:      spanID,
		Severity:    severity,
		Attributes:  attributes,
	}
}

// TestDetectDependencyFailureAtribuiFindingAoServicoDownstream garante que a
// hipótese aponte o serviço mais profundo da cadeia, candidato à origem, e não
// o serviço que apenas sofreu a consequência.
func TestDetectDependencyFailureAtribuiFindingAoServicoDownstream(t *testing.T) {
	t.Parallel()

	baseline := []domain.Signal{
		spanSignal("b1", "checkout-service", "trace-b1", "span-b1", "", ""),
		spanSignal("b2", "payments-service", "trace-b1", "span-b2", "span-b1", "error"),
		spanSignal("b3", "checkout-service", "trace-b2", "span-b3", "", ""),
		spanSignal("b4", "payments-service", "trace-b2", "span-b4", "span-b3", ""),
	}
	incident := []domain.Signal{
		spanSignal("i1", "checkout-service", "trace-i1", "span-i1", "", "error"),
		spanSignal("i2", "payments-service", "trace-i1", "span-i2", "span-i1", "error"),
		spanSignal("i3", "checkout-service", "trace-i2", "span-i3", "", "error"),
		spanSignal("i4", "payments-service", "trace-i2", "span-i4", "span-i3", "error"),
	}

	findings := DetectDependencyFailure(baseline, incident)
	if len(findings) != 1 {
		t.Fatalf("findings = %#v, esperado apenas um", findings)
	}
	finding := findings[0]
	if finding.Rule != RuleDependencyFailure || finding.ServiceName != "payments-service" {
		t.Fatalf("finding = %#v", finding)
	}
	if len(finding.Evidence) != 1 {
		t.Fatalf("evidência = %#v", finding.Evidence)
	}
	summary := finding.Evidence[0].Summary
	if !strings.Contains(summary, "checkout-service") || !strings.Contains(summary, "trace-i1") {
		t.Fatalf("resumo = %q, esperado citar serviço e trace envolvidos", summary)
	}
	if !containsLimitation(finding.Limitations, "topologia") {
		t.Fatalf("limitações = %#v, esperado aviso sobre topologia observada", finding.Limitations)
	}
	if len(finding.Evidence[0].SignalIDs) == 0 {
		t.Fatalf("evidência sem sinais: %#v", finding.Evidence[0])
	}
}

// TestDetectDependencyFailureIgnoraPropagacaoConstante protege contra o sistema
// em que a falha sempre atravessa a cadeia: sem aumento não há hipótese nova.
func TestDetectDependencyFailureIgnoraPropagacaoConstante(t *testing.T) {
	t.Parallel()

	baseline := []domain.Signal{
		spanSignal("b1", "checkout-service", "trace-b1", "span-b1", "", "error"),
		spanSignal("b2", "payments-service", "trace-b1", "span-b2", "span-b1", "error"),
	}
	incident := []domain.Signal{
		spanSignal("i1", "checkout-service", "trace-i1", "span-i1", "", "error"),
		spanSignal("i2", "payments-service", "trace-i1", "span-i2", "span-i1", "error"),
	}

	if findings := DetectDependencyFailure(baseline, incident); len(findings) != 0 {
		t.Fatalf("findings = %#v, esperado silêncio sem aumento", findings)
	}
}

// TestDetectDependencyFailureSuportaCadeiaCiclica garante que telemetria com
// span.parent_id cíclico não trave a investigação.
func TestDetectDependencyFailureSuportaCadeiaCiclica(t *testing.T) {
	t.Parallel()

	incident := []domain.Signal{
		spanSignal("i1", "checkout-service", "trace-i1", "span-i1", "span-i2", "error"),
		spanSignal("i2", "payments-service", "trace-i1", "span-i2", "span-i1", "error"),
	}

	findings := DetectDependencyFailure(nil, incident)
	for _, finding := range findings {
		if finding.Rule != RuleDependencyFailure {
			t.Fatalf("finding = %#v", finding)
		}
	}
}

// TestDetectDependencyFailureOrdenaSaidaPorServico garante saída determinística
// quando mais de um serviço downstream aparece.
func TestDetectDependencyFailureOrdenaSaidaPorServico(t *testing.T) {
	t.Parallel()

	incident := []domain.Signal{
		spanSignal("i1", "checkout-service", "trace-i1", "span-i1", "", "error"),
		spanSignal("i2", "payments-service", "trace-i1", "span-i2", "span-i1", "error"),
		spanSignal("i3", "checkout-service", "trace-i2", "span-i3", "", "error"),
		spanSignal("i4", "auth-service", "trace-i2", "span-i4", "span-i3", "error"),
	}

	findings := DetectDependencyFailure(nil, incident)
	if len(findings) != 2 {
		t.Fatalf("findings = %#v, esperado dois serviços downstream", findings)
	}
	if findings[0].ServiceName != "auth-service" || findings[1].ServiceName != "payments-service" {
		t.Fatalf("ordem = %q, %q", findings[0].ServiceName, findings[1].ServiceName)
	}
}

// TestDependencyFailureContaOsTracesDaBaselineMesmoSemPropagação corrige uma
// evidência enganosa: quando a baseline não tinha propagação alguma — o caso
// normal de um sistema saudável — o denominador vinha zerado e a saída dizia
// "0 de 0 traces", como se o serviço não tivesse sido observado antes.
//
// O denominador precisa ser quantos traces o serviço realmente teve na
// baseline. Ele também alimenta a confiança do finding: com zero, um serviço
// com histórico farto era rotulado como amostra pequena.
func TestDependencyFailureContaOsTracesDaBaselineMesmoSemPropagação(t *testing.T) {
	t.Parallel()

	// Baseline: os dois serviços aparecem em 8 traces, todos saudáveis.
	baseline := make([]domain.Signal, 0, 16)
	for index := 0; index < 8; index++ {
		baseline = append(baseline, cadeiaDeFalha(index, "trace-base", false, false)...)
	}
	// Incidente: a falha do downstream aparece sob a falha do upstream.
	incident := make([]domain.Signal, 0, 16)
	for index := 0; index < 8; index++ {
		incident = append(incident, cadeiaDeFalha(index, "trace-inc", true, true)...)
	}

	findings := DetectDependencyFailure(baseline, incident)
	if len(findings) != 1 {
		t.Fatalf("findings = %d, esperado 1", len(findings))
	}
	resumo := findings[0].Evidence[0].Summary
	if strings.Contains(resumo, "em 0 de 0 traces") {
		t.Fatalf("denominador da baseline zerado, escondendo o histórico do serviço: %s", resumo)
	}
	if !strings.Contains(resumo, "0 de 8 traces") {
		t.Fatalf("a baseline deveria declarar 0 de 8 traces: %s", resumo)
	}
	if findings[0].Confidence != ConfidenceHigh {
		t.Fatalf("confiança = %q, esperado alta com 8 traces em cada janela", findings[0].Confidence)
	}
}

// cadeiaDeFalha monta um trace com um span de checkout-service e, sob ele, um
// span de payment-service, marcando cada um como falho conforme solicitado.
func cadeiaDeFalha(index int, prefixo string, upstreamFalhou, downstreamFalhou bool) []domain.Signal {
	trace := fmt.Sprintf("%s-%03d", prefixo, index)
	paiID := fmt.Sprintf("pai-%03d", index)
	pai := domain.Signal{
		ID: "ck-" + trace, ServiceName: "checkout-service", TraceID: trace, SpanID: paiID,
		Attributes: map[string]string{},
	}
	filho := domain.Signal{
		ID: "pm-" + trace, ServiceName: "payment-service", TraceID: trace,
		SpanID:     fmt.Sprintf("filho-%03d", index),
		Attributes: map[string]string{"span.parent_id": paiID},
	}
	if upstreamFalhou {
		pai.Severity = "error"
	}
	if downstreamFalhou {
		filho.Severity = "error"
	}
	return []domain.Signal{pai, filho}
}
