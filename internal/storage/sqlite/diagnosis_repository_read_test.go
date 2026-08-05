package sqlite

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
)

// TestDiagnosisRepositoryListReturnsBoundedNewestIncidents garante que a
// listagem seja limitada e ordenada deterministicamente sem carregar detalhes.
func TestDiagnosisRepositoryListReturnsBoundedNewestIncidents(t *testing.T) {
	t.Parallel()

	_, repository := openDiagnosisRepository(t)
	ctx := context.Background()
	incidentStart := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	for _, diagnosis := range []application.Diagnosis{
		testDiagnosisAt(t, "incident-z-newest", incidentStart.Add(time.Minute)),
		testDiagnosisAt(t, "incident-a-newest", incidentStart.Add(time.Minute)),
		testDiagnosisAt(t, "incident-older", incidentStart),
	} {
		if created, err := repository.Save(ctx, diagnosis); err != nil || !created {
			t.Fatalf("Save(%q) = (%v, %v), want (true, nil)", diagnosis.ID, created, err)
		}
	}

	got, err := repository.List(ctx, 2)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []application.IncidentSummary{
		{
			ID:            "incident-a-newest",
			ServiceName:   "checkout-service",
			Status:        "diagnosed",
			IncidentStart: incidentStart.Add(time.Minute),
			IncidentEnd:   incidentStart.Add(2 * time.Minute),
		},
		{
			ID:            "incident-z-newest",
			ServiceName:   "checkout-service",
			Status:        "diagnosed",
			IncidentStart: incidentStart.Add(time.Minute),
			IncidentEnd:   incidentStart.Add(2 * time.Minute),
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %#v, want %#v", got, want)
	}
}

// TestDiagnosisRepositoryListRequiresPositiveLimit protege o repositório
// contra coleções ilimitadas por ausência ou invalidez do limite.
func TestDiagnosisRepositoryListRequiresPositiveLimit(t *testing.T) {
	t.Parallel()

	_, repository := openDiagnosisRepository(t)
	for _, limit := range []int{0, -1} {
		t.Run(fmt.Sprintf("limit_%d", limit), func(t *testing.T) {
			if _, err := repository.List(context.Background(), limit); err == nil {
				t.Fatalf("List(limit=%d) error = nil, want validation error", limit)
			}
		})
	}
}

// TestDiagnosisRepositoryListRespectsCanceledContext garante que a consulta
// reutilize o contexto recebido e preserve seu cancelamento.
func TestDiagnosisRepositoryListRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	_, repository := openDiagnosisRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := repository.List(ctx, 10); !errors.Is(err, context.Canceled) {
		t.Fatalf("List() error = %v, want context.Canceled", err)
	}
}

// TestDiagnosisRepositoryGetRestoresCompleteSnapshot garante que a leitura
// preserve metadados, findings e ranking serializados pelo snapshot atômico.
func TestDiagnosisRepositoryGetRestoresCompleteSnapshot(t *testing.T) {
	t.Parallel()

	_, repository := openDiagnosisRepository(t)
	diagnosis := testDiagnosis(t, "incident-complete-read")
	diagnosis.Findings[0].Evidence = append(diagnosis.Findings[0].Evidence, detection.Evidence{
		Summary:       "Evidência com acentuação, aspas \"e JSON\" preservado.",
		SignalIDs:     []string{"signal-db-3", "signal-db-4"},
		BaselineValue: 0.1,
		IncidentValue: 0.7,
	})
	if created, err := repository.Save(context.Background(), diagnosis); err != nil || !created {
		t.Fatalf("Save() = (%v, %v), want (true, nil)", created, err)
	}

	got, err := repository.Get(context.Background(), diagnosis.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	wantIncident := application.IncidentSummary{
		ID:            diagnosis.ID,
		ServiceName:   diagnosis.ServiceName,
		Status:        "diagnosed",
		IncidentStart: diagnosis.Windows.Incident.Start,
		IncidentEnd:   diagnosis.Windows.Incident.End,
	}
	if got.Incident != wantIncident {
		t.Fatalf("Get().Incident = %#v, want %#v", got.Incident, wantIncident)
	}
	assertTimePointerEqual(t, "BaselineStart", got.BaselineStart, diagnosis.Windows.Baseline.Start)
	assertTimePointerEqual(t, "BaselineEnd", got.BaselineEnd, diagnosis.Windows.Baseline.End)
	assertIntPointerEqual(t, "BaselineSignalCount", got.BaselineSignalCount, diagnosis.BaselineSignalCount)
	assertIntPointerEqual(t, "IncidentSignalCount", got.IncidentSignalCount, diagnosis.IncidentSignalCount)
	if !reflect.DeepEqual(got.Findings, diagnosis.Findings) {
		t.Fatalf("Get().Findings = %#v, want %#v", got.Findings, diagnosis.Findings)
	}
	if !reflect.DeepEqual(got.Suspects, diagnosis.Suspects) {
		t.Fatalf("Get().Suspects = %#v, want %#v", got.Suspects, diagnosis.Suspects)
	}
}

// TestDiagnosisRepositoryGetReturnsTypedNotFound garante que ausência de
// registro não seja confundida com indisponibilidade do banco.
func TestDiagnosisRepositoryGetReturnsTypedNotFound(t *testing.T) {
	t.Parallel()

	_, repository := openDiagnosisRepository(t)
	_, err := repository.Get(context.Background(), "incident-missing")
	if !errors.Is(err, application.ErrIncidentNotFound) {
		t.Fatalf("Get() error = %v, want application.ErrIncidentNotFound", err)
	}
}

// TestDiagnosisRepositoryGetRespectsCanceledContext garante que todas as
// leituras necessárias ao snapshot respeitem o contexto recebido.
func TestDiagnosisRepositoryGetRespectsCanceledContext(t *testing.T) {
	t.Parallel()

	_, repository := openDiagnosisRepository(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := repository.Get(ctx, "incident-any"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get() error = %v, want context.Canceled", err)
	}
}

// TestDiagnosisSnapshotMigrationAddsNullableMetadataColumns documenta a
// evolução necessária para recuperar snapshots novos sem invalidar os antigos.
func TestDiagnosisSnapshotMigrationAddsNullableMetadataColumns(t *testing.T) {
	t.Parallel()

	database, _ := openDiagnosisRepository(t)
	rows, err := database.QueryContext(context.Background(), `PRAGMA table_info(incidents)`)
	if err != nil {
		t.Fatalf("read incidents schema: %v", err)
	}
	defer rows.Close()

	nullableByColumn := make(map[string]bool)
	for rows.Next() {
		var position, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&position, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan incidents column: %v", err)
		}
		nullableByColumn[name] = notNull == 0
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate incidents schema: %v", err)
	}

	for _, column := range []string{"baseline_start", "baseline_end", "baseline_signal_count", "incident_signal_count"} {
		nullable, found := nullableByColumn[column]
		if !found {
			t.Errorf("incidents column %q is missing", column)
			continue
		}
		if !nullable {
			t.Errorf("incidents column %q is NOT NULL, want nullable for legacy rows", column)
		}
	}
}

// TestDiagnosisRepositoryGetSupportsLegacySnapshot garante que as novas
// colunas anuláveis não tornem incidentes anteriores à migration ilegíveis.
func TestDiagnosisRepositoryGetSupportsLegacySnapshot(t *testing.T) {
	t.Parallel()

	database, repository := openDiagnosisRepository(t)
	incidentStart := time.Date(2025, time.December, 1, 10, 1, 0, 0, time.UTC)
	incidentEnd := incidentStart.Add(time.Minute)
	if _, err := database.ExecContext(context.Background(), `
		INSERT INTO incidents (id, service_name, environment, started_at, ended_at, status)
		VALUES (?, ?, ?, ?, ?, ?)
	`, "incident-legacy", "checkout-service", "", incidentStart, incidentEnd, "diagnosed"); err != nil {
		t.Fatalf("insert legacy incident: %v", err)
	}
	if _, err := database.ExecContext(context.Background(), `
		INSERT INTO ranking_results (id, incident_id, generated_at, suspects_json)
		VALUES (?, ?, ?, ?)
	`, "ranking:incident-legacy", "incident-legacy", incidentEnd, `[]`); err != nil {
		t.Fatalf("insert legacy ranking: %v", err)
	}

	got, err := repository.Get(context.Background(), "incident-legacy")
	if err != nil {
		t.Fatalf("Get() legacy error = %v", err)
	}
	if got.Incident.ID != "incident-legacy" || !got.Incident.IncidentStart.Equal(incidentStart) || !got.Incident.IncidentEnd.Equal(incidentEnd) {
		t.Fatalf("Get() legacy incident = %#v", got.Incident)
	}
	if got.BaselineStart != nil || got.BaselineEnd != nil || got.BaselineSignalCount != nil || got.IncidentSignalCount != nil {
		t.Fatalf("Get() legacy metadata = (%v, %v, %v, %v), want all nil", got.BaselineStart, got.BaselineEnd, got.BaselineSignalCount, got.IncidentSignalCount)
	}
	if len(got.Findings) != 0 || len(got.Suspects) != 0 {
		t.Fatalf("Get() legacy details = (%#v, %#v), want empty", got.Findings, got.Suspects)
	}
}

func testDiagnosisAt(t *testing.T, id string, incidentStart time.Time) application.Diagnosis {
	t.Helper()

	diagnosis := testDiagnosis(t, id)
	delta := incidentStart.Sub(diagnosis.Windows.Incident.Start)
	diagnosis.Windows.Baseline.Start = diagnosis.Windows.Baseline.Start.Add(delta)
	diagnosis.Windows.Baseline.End = diagnosis.Windows.Baseline.End.Add(delta)
	diagnosis.Windows.Incident.Start = diagnosis.Windows.Incident.Start.Add(delta)
	diagnosis.Windows.Incident.End = diagnosis.Windows.Incident.End.Add(delta)
	return diagnosis
}

func assertTimePointerEqual(t *testing.T, field string, got *time.Time, want time.Time) {
	t.Helper()
	if got == nil || !got.Equal(want) {
		t.Fatalf("%s = %v, want %s", field, got, want)
	}
}

func assertIntPointerEqual(t *testing.T, field string, got *int, want int) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s = %v, want %d", field, got, want)
	}
}
