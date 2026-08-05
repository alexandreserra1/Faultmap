package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
)

// TestDiagnosisIDEDeterministicoEmUTC garante idempotência mesmo quando as
// mesmas fronteiras chegam representadas em fusos horários diferentes.
func TestDiagnosisIDEDeterministicoEmUTC(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(start, start.Add(time.Minute), time.Minute)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}
	offset := time.FixedZone("teste", -3*60*60)
	windowsInOffset := incidentdomain.InvestigationWindow{
		Baseline: incidentdomain.TimeWindow{Start: windows.Baseline.Start.In(offset), End: windows.Baseline.End.In(offset)},
		Incident: incidentdomain.TimeWindow{Start: windows.Incident.Start.In(offset), End: windows.Incident.End.In(offset)},
	}

	first := DiagnosisID("checkout-service", windows)
	second := DiagnosisID("checkout-service", windowsInOffset)
	if first != second {
		t.Fatalf("IDs para os mesmos instantes = %q e %q", first, second)
	}
	if !strings.HasPrefix(first, "inc_") {
		t.Fatalf("ID = %q, esperado prefixo inc_", first)
	}
	if first == DiagnosisID("payment-service", windows) {
		t.Fatal("serviços diferentes receberam o mesmo ID")
	}
	if first == diagnosisID("checkout-service", "staging", windows) {
		t.Fatal("ambientes diferentes receberam o mesmo ID")
	}
}

// TestPersistDiagnosisValidaEDelegaUmaVez mantém SQL fora do caso de uso e
// preserva o resultado idempotente informado pelo repositório.
func TestPersistDiagnosisValidaEDelegaUmaVez(t *testing.T) {
	t.Parallel()

	diagnosis := validDiagnosisForPersistence(t)
	store := &diagnosisStoreFake{created: true}
	created, err := PersistDiagnosis(context.Background(), diagnosis, store)
	if err != nil {
		t.Fatalf("PersistDiagnosis() erro = %v", err)
	}
	if !created || store.calls != 1 || store.received.ID != diagnosis.ID {
		t.Fatalf("persistência = created %v calls %d diagnosis %#v", created, store.calls, store.received)
	}
}

// TestPersistDiagnosisRejeitaEntradaAntesDoStore evita abrir uma transação para um DTO incompleto.
func TestPersistDiagnosisRejeitaEntradaAntesDoStore(t *testing.T) {
	t.Parallel()

	store := &diagnosisStoreFake{}
	if _, err := PersistDiagnosis(context.Background(), Diagnosis{}, store); err == nil {
		t.Fatal("PersistDiagnosis() erro = nil para diagnóstico vazio")
	}
	if store.calls != 0 {
		t.Fatalf("chamadas ao store = %d, esperado zero", store.calls)
	}
}

// TestPersistDiagnosisNaoSalvaIncidenteSemSinais evita congelar como imutável
// uma investigação executada antes de a telemetria do incidente chegar.
func TestPersistDiagnosisNaoSalvaIncidenteSemSinais(t *testing.T) {
	t.Parallel()

	diagnosis := validDiagnosisForPersistence(t)
	diagnosis.IncidentSignalCount = 0
	store := &diagnosisStoreFake{}
	_, err := PersistDiagnosis(context.Background(), diagnosis, store)
	if !errors.Is(err, ErrNoIncidentSignals) {
		t.Fatalf("PersistDiagnosis() erro = %v, esperado ErrNoIncidentSignals", err)
	}
	if store.calls != 0 {
		t.Fatalf("chamadas ao store = %d, esperado zero", store.calls)
	}
}

// TestPersistDiagnosisPreservaCausaDoStore garante diagnóstico operacional sem perder errors.Is.
func TestPersistDiagnosisPreservaCausaDoStore(t *testing.T) {
	t.Parallel()

	cause := errors.New("falha de infraestrutura")
	store := &diagnosisStoreFake{err: cause}
	_, err := PersistDiagnosis(context.Background(), validDiagnosisForPersistence(t), store)
	if !errors.Is(err, cause) {
		t.Fatalf("PersistDiagnosis() erro = %v, esperado preservar causa", err)
	}
}

type diagnosisStoreFake struct {
	created  bool
	err      error
	calls    int
	received Diagnosis
}

// Save implementa a gravação atômica esperada pelo caso de uso.
func (store *diagnosisStoreFake) Save(_ context.Context, diagnosis Diagnosis) (bool, error) {
	store.calls++
	store.received = diagnosis
	return store.created, store.err
}

func validDiagnosisForPersistence(t *testing.T) Diagnosis {
	t.Helper()
	start := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(start, start.Add(time.Minute), time.Minute)
	if err != nil {
		t.Fatalf("criar janelas: %v", err)
	}
	return Diagnosis{
		ID:                  DiagnosisID("checkout-service", windows),
		ServiceName:         "checkout-service",
		Windows:             windows,
		BaselineSignalCount: 40,
		IncidentSignalCount: 40,
	}
}
