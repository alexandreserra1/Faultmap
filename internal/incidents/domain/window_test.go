package domain

import (
	"strings"
	"testing"
	"time"
)

func TestNewTimeWindow(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, time.December, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)

	window, err := NewTimeWindow(start, end)
	if err != nil {
		t.Fatalf("NewTimeWindow() retornou erro: %v", err)
	}
	if !window.Start.Equal(start) {
		t.Errorf("Start = %s, esperado %s", window.Start, start)
	}
	if !window.End.Equal(end) {
		t.Errorf("End = %s, esperado %s", window.End, end)
	}
}

func TestNewTimeWindowRejectsInvalidBoundaries(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, time.December, 1, 10, 0, 0, 0, time.UTC)

	testCases := []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{name: "limites iguais", start: start, end: start},
		{name: "fim anterior ao início", start: start, end: start.Add(-time.Second)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewTimeWindow(testCase.start, testCase.end)
			if err == nil {
				t.Fatal("NewTimeWindow() não retornou erro")
			}
			if !strings.Contains(err.Error(), "início") {
				t.Errorf("erro = %q, esperado contexto sobre início", err)
			}
		})
	}
}

func TestNewInvestigationWindowFromIncidentCalculatesContiguousBaseline(t *testing.T) {
	t.Parallel()

	incidentStart := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	incidentEnd := incidentStart.Add(time.Minute)

	window, err := NewInvestigationWindowFromIncident(incidentStart, incidentEnd, 30*time.Minute)
	if err != nil {
		t.Fatalf("NewInvestigationWindowFromIncident() retornou erro: %v", err)
	}

	if !window.Incident.Start.Equal(incidentStart) || !window.Incident.End.Equal(incidentEnd) {
		t.Errorf("Incident = %#v, esperado %s até %s", window.Incident, incidentStart, incidentEnd)
	}
	if !window.Baseline.Start.Equal(incidentStart.Add(-30 * time.Minute)) {
		t.Errorf("Baseline.Start = %s, esperado %s", window.Baseline.Start, incidentStart.Add(-30*time.Minute))
	}
	if !window.Baseline.End.Equal(incidentStart) {
		t.Errorf("Baseline.End = %s, esperado %s", window.Baseline.End, incidentStart)
	}
}

func TestNewInvestigationWindowAllowsBaselineToMeetIncidentBoundary(t *testing.T) {
	t.Parallel()

	baseline, err := NewTimeWindow(
		time.Date(2025, time.December, 1, 9, 0, 0, 0, time.UTC),
		time.Date(2025, time.December, 1, 10, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewTimeWindow() retornou erro: %v", err)
	}
	incident, err := NewTimeWindow(
		time.Date(2025, time.December, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2025, time.December, 1, 10, 30, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewTimeWindow() retornou erro: %v", err)
	}

	if _, err := NewInvestigationWindow(baseline, incident); err != nil {
		t.Fatalf("NewInvestigationWindow() retornou erro: %v", err)
	}
}

func TestNewInvestigationWindowRejectsOverlappingOrInvalidWindows(t *testing.T) {
	t.Parallel()

	incident, err := NewTimeWindow(
		time.Date(2025, time.December, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2025, time.December, 1, 10, 30, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewTimeWindow() retornou erro: %v", err)
	}
	overlappingBaseline, err := NewTimeWindow(
		time.Date(2025, time.December, 1, 9, 45, 0, 0, time.UTC),
		time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("NewTimeWindow() retornou erro: %v", err)
	}

	_, err = NewInvestigationWindow(overlappingBaseline, incident)
	if err == nil {
		t.Fatal("NewInvestigationWindow() não retornou erro para janelas sobrepostas")
	}
	if !strings.Contains(err.Error(), "anterior") {
		t.Errorf("erro = %q, esperado contexto sobre anterioridade", err)
	}
}

func TestNewInvestigationWindowFromIncidentRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	start := time.Date(2025, time.December, 1, 10, 0, 0, 0, time.UTC)
	testCases := []struct {
		name             string
		incidentStart    time.Time
		incidentEnd      time.Time
		baselineDuration time.Duration
	}{
		{name: "incidente vazio", incidentStart: start, incidentEnd: start, baselineDuration: time.Minute},
		{name: "incidente invertido", incidentStart: start, incidentEnd: start.Add(-time.Minute), baselineDuration: time.Minute},
		{name: "baseline vazia", incidentStart: start, incidentEnd: start.Add(time.Minute), baselineDuration: 0},
		{name: "baseline negativa", incidentStart: start, incidentEnd: start.Add(time.Minute), baselineDuration: -time.Minute},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewInvestigationWindowFromIncident(
				testCase.incidentStart,
				testCase.incidentEnd,
				testCase.baselineDuration,
			)
			if err == nil {
				t.Fatal("NewInvestigationWindowFromIncident() não retornou erro")
			}
		})
	}
}
