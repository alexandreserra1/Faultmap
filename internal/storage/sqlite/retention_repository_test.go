package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/detection"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
	"github.com/faultmap/faultmap/internal/ranking"
	"github.com/faultmap/faultmap/internal/telemetry/domain"
)

// TestRetentionRepositoryRemoveSomenteSinaisAnterioresAoCorte garante que a
// limpeza respeita o limite temporal e nunca alcança telemetria ainda vigente.
func TestRetentionRepositoryRemoveSomenteSinaisAnterioresAoCorte(t *testing.T) {
	t.Parallel()

	database := openRetentionDatabase(t)
	signals := NewSignalRepository(database)
	repository := NewRetentionRepository(database)
	cutoff := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)

	if _, err := signals.Save(context.Background(), []domain.Signal{
		testSignal("antigo-1", "checkout", cutoff.Add(-48*time.Hour)),
		testSignal("antigo-2", "checkout", cutoff.Add(-time.Second)),
		testSignal("no-corte", "checkout", cutoff),
		testSignal("recente", "checkout", cutoff.Add(time.Hour)),
	}); err != nil {
		t.Fatalf("Save() erro = %v", err)
	}

	removed, err := repository.DeleteSignalsBefore(context.Background(), cutoff, 100)
	if err != nil {
		t.Fatalf("DeleteSignalsBefore() erro = %v", err)
	}
	if removed != 2 {
		t.Fatalf("DeleteSignalsBefore() removidos = %d, esperado 2", removed)
	}

	remaining := signalIDs(t, database)
	if len(remaining) != 2 || remaining[0] != "no-corte" || remaining[1] != "recente" {
		t.Fatalf("sinais restantes = %v, esperado [no-corte recente]", remaining)
	}
}

// TestRetentionRepositoryRespeitaLimiteDoLote mantém transações curtas: cada
// chamada apaga no máximo o lote solicitado, começando pelos sinais mais antigos.
func TestRetentionRepositoryRespeitaLimiteDoLote(t *testing.T) {
	t.Parallel()

	database := openRetentionDatabase(t)
	signals := NewSignalRepository(database)
	repository := NewRetentionRepository(database)
	cutoff := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)

	if _, err := signals.Save(context.Background(), []domain.Signal{
		testSignal("antigo-a", "checkout", cutoff.Add(-4*time.Hour)),
		testSignal("antigo-b", "checkout", cutoff.Add(-3*time.Hour)),
		testSignal("antigo-c", "checkout", cutoff.Add(-2*time.Hour)),
	}); err != nil {
		t.Fatalf("Save() erro = %v", err)
	}

	removed, err := repository.DeleteSignalsBefore(context.Background(), cutoff, 2)
	if err != nil {
		t.Fatalf("DeleteSignalsBefore() erro = %v", err)
	}
	if removed != 2 {
		t.Fatalf("DeleteSignalsBefore() removidos = %d, esperado 2", removed)
	}

	remaining := signalIDs(t, database)
	if len(remaining) != 1 || remaining[0] != "antigo-c" {
		t.Fatalf("sinais restantes = %v, esperado [antigo-c]", remaining)
	}
}

// TestRetentionRepositoryPreservaSnapshotsDeIncidentes protege a auditoria: um
// diagnóstico já publicado continua legível mesmo depois que sua telemetria
// bruta expira, evitando que incident show passe a falhar com o tempo.
func TestRetentionRepositoryPreservaSnapshotsDeIncidentes(t *testing.T) {
	t.Parallel()

	database := openRetentionDatabase(t)
	repository := NewRetentionRepository(database)
	cutoff := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)

	if _, err := NewSignalRepository(database).Save(context.Background(), []domain.Signal{
		testSignal("antigo-1", "checkout", cutoff.Add(-72*time.Hour)),
	}); err != nil {
		t.Fatalf("Save() erro = %v", err)
	}
	diagnoses := NewDiagnosisRepository(database)
	windows, err := incidentdomain.NewInvestigationWindowFromIncident(
		cutoff.Add(-72*time.Hour), cutoff.Add(-71*time.Hour), time.Hour,
	)
	if err != nil {
		t.Fatalf("NewInvestigationWindowFromIncident() erro = %v", err)
	}
	if _, err := diagnoses.Save(context.Background(), application.Diagnosis{
		ID:                  "inc_retencao",
		ServiceName:         "checkout",
		Environment:         "demo",
		Windows:             windows,
		BaselineSignalCount: 1,
		IncidentSignalCount: 1,
		Findings: []detection.Finding{{
			Rule: detection.RuleErrorRateDelta, ServiceName: "checkout", Score: 1,
			Confidence: detection.ConfidenceHigh,
			Evidence:   []detection.Evidence{{Summary: "erros", SignalIDs: []string{"antigo-1"}}},
		}},
		Suspects: []ranking.Suspect{{
			ID: "checkout", Label: "checkout", Score: 0.25, Confidence: detection.ConfidenceHigh,
		}},
	}); err != nil {
		t.Fatalf("Save() diagnóstico erro = %v", err)
	}

	if _, err := repository.DeleteSignalsBefore(context.Background(), cutoff, 100); err != nil {
		t.Fatalf("DeleteSignalsBefore() erro = %v", err)
	}

	persisted, err := diagnoses.Get(context.Background(), "inc_retencao")
	if err != nil {
		t.Fatalf("Get() erro = %v", err)
	}
	if len(persisted.Findings) != 1 || len(persisted.Suspects) != 1 {
		t.Fatalf("snapshot perdido após retenção: %+v", persisted)
	}
}

// TestRetentionRepositoryRejeitaLimiteInválido impede DELETE sem fronteira.
func TestRetentionRepositoryRejeitaLimiteInválido(t *testing.T) {
	t.Parallel()

	repository := NewRetentionRepository(openRetentionDatabase(t))
	if _, err := repository.DeleteSignalsBefore(context.Background(), time.Now().UTC(), 0); err == nil {
		t.Fatal("DeleteSignalsBefore() erro = nil, esperado erro de limite")
	}
}

func openRetentionDatabase(t *testing.T) *sql.DB {
	t.Helper()

	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "faultmap.db"))
	if err != nil {
		t.Fatalf("Open() erro = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar banco: %v", closeErr)
		}
	})
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() erro = %v", err)
	}
	return database
}

func signalIDs(t *testing.T, database *sql.DB) []string {
	t.Helper()

	rows, err := database.QueryContext(context.Background(), `SELECT id FROM signals ORDER BY id ASC`)
	if err != nil {
		t.Fatalf("consultar sinais: %v", err)
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("ler sinal: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterar sinais: %v", err)
	}
	return ids
}
