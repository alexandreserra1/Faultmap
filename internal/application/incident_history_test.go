package application

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestListIncidentsValidaLimiteAntesDoRepositorio impede leituras ilimitadas do histórico.
func TestListIncidentsValidaLimiteAntesDoRepositorio(t *testing.T) {
	t.Parallel()

	reader := &incidentHistoryReaderFake{}
	if _, err := ListIncidents(context.Background(), 0, reader); err == nil {
		t.Fatal("ListIncidents() erro = nil para limite zero")
	}
	if reader.listCalls != 0 {
		t.Fatalf("consultas = %d, esperado zero", reader.listCalls)
	}
}

// TestListIncidentsPreservaOrdemDoRepositorio mantém a ordenação estável definida pela persistência.
func TestListIncidentsPreservaOrdemDoRepositorio(t *testing.T) {
	t.Parallel()

	want := []IncidentSummary{{ID: "inc-2"}, {ID: "inc-1"}}
	reader := &incidentHistoryReaderFake{summaries: want}
	got, err := ListIncidents(context.Background(), 2, reader)
	if err != nil {
		t.Fatalf("ListIncidents() erro = %v", err)
	}
	if len(got) != 2 || got[0].ID != "inc-2" || reader.listLimit != 2 {
		t.Fatalf("resultado = %#v; limite recebido = %d", got, reader.listLimit)
	}
}

// TestGetIncidentValidaIDEPropagaAusencia distingue entrada inválida de snapshot inexistente.
func TestGetIncidentValidaIDEPropagaAusencia(t *testing.T) {
	t.Parallel()

	reader := &incidentHistoryReaderFake{}
	if _, err := GetIncident(context.Background(), " ", reader); err == nil {
		t.Fatal("GetIncident() erro = nil para ID vazio")
	}
	if reader.getCalls != 0 {
		t.Fatalf("consultas = %d, esperado zero", reader.getCalls)
	}

	reader.err = ErrIncidentNotFound
	_, err := GetIncident(context.Background(), "inc-missing", reader)
	if !errors.Is(err, ErrIncidentNotFound) {
		t.Fatalf("GetIncident() erro = %v, esperado ErrIncidentNotFound", err)
	}
}

// TestPersistedDiagnosisMetadataComplete diferencia snapshots novos de linhas legadas.
func TestPersistedDiagnosisMetadataComplete(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, time.December, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Minute)
	baselineCount := 40
	incidentCount := 40
	complete := PersistedDiagnosis{
		BaselineStart:       &start,
		BaselineEnd:         &end,
		BaselineSignalCount: &baselineCount,
		IncidentSignalCount: &incidentCount,
	}
	if !complete.MetadataComplete() {
		t.Fatal("MetadataComplete() = false para snapshot completo")
	}
	complete.BaselineStart = nil
	if complete.MetadataComplete() {
		t.Fatal("MetadataComplete() = true para snapshot legado")
	}
}

type incidentHistoryReaderFake struct {
	summaries []IncidentSummary
	diagnosis PersistedDiagnosis
	err       error
	listCalls int
	getCalls  int
	listLimit int
}

// List implementa a consulta limitada do histórico para o caso de uso.
func (reader *incidentHistoryReaderFake) List(_ context.Context, limit int) ([]IncidentSummary, error) {
	reader.listCalls++
	reader.listLimit = limit
	return reader.summaries, reader.err
}

// Get implementa a leitura consistente de um snapshot persistido.
func (reader *incidentHistoryReaderFake) Get(_ context.Context, _ string) (PersistedDiagnosis, error) {
	reader.getCalls++
	return reader.diagnosis, reader.err
}
