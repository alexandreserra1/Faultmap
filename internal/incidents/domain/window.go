// Package domain contém os conceitos de incidente independentes de transporte e persistência.
package domain

import (
	"fmt"
	"time"
)

// TimeWindow representa um intervalo temporal semiaberto [Start, End).
// O fim exclusivo permite que janelas consecutivas compartilhem a fronteira sem duplicar sinais.
type TimeWindow struct {
	Start time.Time
	End   time.Time
}

// NewTimeWindow constrói uma janela temporal válida cujo início é estritamente anterior ao fim.
func NewTimeWindow(start time.Time, end time.Time) (TimeWindow, error) {
	window := TimeWindow{Start: start, End: end}
	if err := window.Validate(); err != nil {
		return TimeWindow{}, fmt.Errorf("criar janela temporal: %w", err)
	}
	return window, nil
}

// Validate verifica se a janela possui duração positiva.
func (window TimeWindow) Validate() error {
	if !window.Start.Before(window.End) {
		return fmt.Errorf("início da janela deve ser anterior ao fim")
	}
	return nil
}

// InvestigationWindow agrupa a janela sob investigação e a baseline anterior usada para comparação.
type InvestigationWindow struct {
	Baseline TimeWindow
	Incident TimeWindow
}

// NewInvestigationWindow constrói uma investigação válida quando a baseline termina no máximo na fronteira de início do incidente.
func NewInvestigationWindow(baseline TimeWindow, incident TimeWindow) (InvestigationWindow, error) {
	window := InvestigationWindow{Baseline: baseline, Incident: incident}
	if err := window.Validate(); err != nil {
		return InvestigationWindow{}, fmt.Errorf("criar janela de investigação: %w", err)
	}
	return window, nil
}

// NewInvestigationWindowFromIncident cria uma investigação com baseline contígua e inteiramente anterior ao incidente.
func NewInvestigationWindowFromIncident(
	incidentStart time.Time,
	incidentEnd time.Time,
	baselineDuration time.Duration,
) (InvestigationWindow, error) {
	if baselineDuration <= 0 {
		return InvestigationWindow{}, fmt.Errorf("criar janela de investigação: duração da baseline deve ser maior que zero")
	}

	incident, err := NewTimeWindow(incidentStart, incidentEnd)
	if err != nil {
		return InvestigationWindow{}, fmt.Errorf("criar janela de investigação: incidente inválido: %w", err)
	}
	baseline, err := NewTimeWindow(incidentStart.Add(-baselineDuration), incidentStart)
	if err != nil {
		return InvestigationWindow{}, fmt.Errorf("criar janela de investigação: baseline inválida: %w", err)
	}
	return NewInvestigationWindow(baseline, incident)
}

// Validate verifica as janelas individuais e garante que a baseline não se sobreponha ao incidente.
func (window InvestigationWindow) Validate() error {
	if err := window.Baseline.Validate(); err != nil {
		return fmt.Errorf("baseline inválida: %w", err)
	}
	if err := window.Incident.Validate(); err != nil {
		return fmt.Errorf("incidente inválido: %w", err)
	}
	if window.Baseline.End.After(window.Incident.Start) {
		return fmt.Errorf("baseline deve ser inteiramente anterior ao incidente")
	}
	return nil
}
