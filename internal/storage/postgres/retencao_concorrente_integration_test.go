package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/faultmap/faultmap/internal/storage/postgres"
	telemetrydomain "github.com/faultmap/faultmap/internal/telemetry/domain"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Instantes fixos deste arquivo. A telemetria semeada fica bem antes do corte e
// a janela é larga o bastante para que nada dependa do relógio da máquina.
var (
	inicioRetencao = time.Date(2026, time.September, 20, 12, 0, 0, 0, time.UTC)
	corteRetencao  = inicioRetencao.Add(24 * time.Hour)
)

// TestRetencaoIntegracaoNaoEsperaLoteAlheio cobra a propriedade que separa
// "correto" de "usável" quando duas execuções de `retention apply` se cruzam
// no mesmo PostgreSQL.
//
// Reservar o lote pode ser feito de dois jeitos, e os dois deixam o banco no
// estado certo: esperar a outra execução soltar as linhas, ou pular as que ela
// já tomou. A diferença não aparece no resultado final, só no tempo — e tempo
// medido por cronômetro em teste é promessa que quebra sozinha. Aqui a espera
// é observada sem cronômetro: uma transação segura o lote mais antigo e não o
// solta, e a limpeza precisa devolver um lote cheio das linhas seguintes
// dentro de um prazo curto. Se ela esperar, o prazo estoura e o caso falha.
//
// O caso existe porque a versão que bloqueia passa em toda a bateria de
// conformidade: sem ele, trocar SKIP LOCKED por FOR UPDATE não quebraria nada
// e a retenção voltaria a serializar em silêncio.
func TestRetencaoIntegracaoNaoEsperaLoteAlheio(t *testing.T) {
	dsn := exigirDSNDeIntegracao(t)
	database := abrirEmSchemaIsolado(t, dsn)
	if err := postgres.Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() erro = %v", err)
	}

	const lote = 50
	semearTelemetriaExpirada(t, database, 2*lote)

	// Uma execução em andamento, parada no meio da transação: é exatamente o
	// que a outra encontra quando os dois comandos se cruzam.
	segurando, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx() erro = %v", err)
	}
	defer func() { _ = segurando.Rollback() }()
	presos := idsTravados(t, segurando, lote)
	if len(presos) != lote {
		t.Fatalf("linhas travadas = %d, esperado %d: o caso não estabeleceu a contenção", len(presos), lote)
	}

	prazo, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	repositorio := postgres.NewRetentionRepository(database)
	removidos, err := repositorio.DeleteSignalsBefore(prazo, corteRetencao, lote)
	if err != nil {
		t.Fatalf("DeleteSignalsBefore() erro = %v: a limpeza ficou esperando o lote da outra execução "+
			"em vez de seguir para as linhas livres", err)
	}
	if removidos != lote {
		t.Fatalf("removidos = %d, esperado %d: o lote reservado por outra execução encurtou este", removidos, lote)
	}

	// E o que foi apagado são as linhas livres, não as travadas: a reserva
	// alheia é respeitada, não ignorada.
	if err := segurando.Rollback(); err != nil {
		t.Fatalf("Rollback() erro = %v", err)
	}
	sobreviventes := idsRestantes(t, database)
	if len(sobreviventes) != lote {
		t.Fatalf("sinais restantes = %d, esperado %d", len(sobreviventes), lote)
	}
	travados := make(map[string]struct{}, len(presos))
	for _, id := range presos {
		travados[id] = struct{}{}
	}
	for _, id := range sobreviventes {
		if _, estava := travados[id]; !estava {
			t.Errorf("sinal %q sobreviveu sem estar travado: a limpeza escolheu o lote errado", id)
		}
	}
}

// TestPodaIntegracaoNaoEsperaLoteAlheio é o mesmo contrato para o catálogo.
// A liberação e a limpeza de telemetria avançam na mesma execução, e uma poda
// que espera segura a outra frente junto.
func TestPodaIntegracaoNaoEsperaLoteAlheio(t *testing.T) {
	dsn := exigirDSNDeIntegracao(t)
	database := abrirEmSchemaIsolado(t, dsn)
	if err := postgres.Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() erro = %v", err)
	}

	const lote = 10
	// Duas bases, cada uma com lote+1 coletas: a mais recente de cada é
	// protegida pela ADR 0015, sobrando exatamente lote coletas por base.
	semearCatalogosExpirados(t, database, []string{"alfa", "beta"}, lote+1)

	segurando, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx() erro = %v", err)
	}
	defer func() { _ = segurando.Rollback() }()
	if travadas := idsDeCatalogoTravados(t, segurando, lote); travadas != lote {
		t.Fatalf("coletas travadas = %d, esperado %d", travadas, lote)
	}

	prazo, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	repositorio := postgres.NewRetentionRepository(database)
	podados, err := repositorio.PruneSchemaCatalogsBefore(prazo, corteRetencao, lote)
	if err != nil {
		t.Fatalf("PruneSchemaCatalogsBefore() erro = %v: a poda ficou esperando o lote da outra execução", err)
	}
	if podados != lote {
		t.Fatalf("podados = %d, esperado %d: o lote reservado por outra execução encurtou este", podados, lote)
	}
}

// ------------------------------------------------------------------ apoio ---

func exigirDSNDeIntegracao(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("FAULTMAP_TEST_PG_DSN"))
	if dsn == "" {
		t.Skip("FAULTMAP_TEST_PG_DSN não definida; contenção de retenção ignorada")
	}
	return dsn
}

// idsTravados reserva o lote mais antigo com a mesma consulta que a limpeza
// usaria e o mantém preso até o fim da transação recebida.
func idsTravados(t *testing.T, transacao *sql.Tx, limite int) []string {
	t.Helper()
	linhas, err := transacao.QueryContext(context.Background(), `
		SELECT id FROM signals
		WHERE timestamp < $1
		ORDER BY timestamp ASC, id ASC
		LIMIT $2
		FOR UPDATE
	`, corteRetencao, limite)
	if err != nil {
		t.Fatalf("travar lote: %v", err)
	}
	defer func() { _ = linhas.Close() }()
	return lerIDs(t, linhas)
}

func idsDeCatalogoTravados(t *testing.T, transacao *sql.Tx, limite int) int {
	t.Helper()
	linhas, err := transacao.QueryContext(context.Background(), `
		SELECT s.id FROM schema_snapshots s
		WHERE s.captured_at < $1
		  AND s.objects_json <> ''
		  AND s.id <> (
			SELECT u.id FROM schema_snapshots u
			WHERE u.database_name = s.database_name
			ORDER BY u.captured_at DESC, u.id DESC
			LIMIT 1
		  )
		ORDER BY s.captured_at ASC, s.id ASC
		LIMIT $2
		FOR UPDATE
	`, corteRetencao, limite)
	if err != nil {
		t.Fatalf("travar coletas: %v", err)
	}
	defer func() { _ = linhas.Close() }()
	return len(lerIDs(t, linhas))
}

func idsRestantes(t *testing.T, database *sql.DB) []string {
	t.Helper()
	linhas, err := database.QueryContext(context.Background(),
		`SELECT id FROM signals ORDER BY timestamp ASC, id ASC`)
	if err != nil {
		t.Fatalf("listar sinais restantes: %v", err)
	}
	defer func() { _ = linhas.Close() }()
	return lerIDs(t, linhas)
}

func lerIDs(t *testing.T, linhas *sql.Rows) []string {
	t.Helper()
	ids := make([]string, 0)
	for linhas.Next() {
		var id string
		if err := linhas.Scan(&id); err != nil {
			t.Fatalf("ler id: %v", err)
		}
		ids = append(ids, id)
	}
	if err := linhas.Err(); err != nil {
		t.Fatalf("percorrer ids: %v", err)
	}
	return ids
}

func semearTelemetriaExpirada(t *testing.T, database *sql.DB, quantidade int) {
	t.Helper()
	sinais := make([]telemetrydomain.Signal, 0, quantidade)
	for indice := range quantidade {
		sinais = append(sinais, telemetrydomain.Signal{
			ID:           fmt.Sprintf("expirado-%05d", indice),
			Type:         telemetrydomain.SignalTypeSpan,
			ServiceName:  "checkout",
			Timestamp:    inicioRetencao.Add(time.Duration(indice) * time.Millisecond),
			TraceID:      "trace-a",
			SpanID:       fmt.Sprintf("span-%05d", indice),
			Attributes:   map[string]string{},
			Measurements: map[string]float64{},
		})
	}
	if _, err := postgres.NewSignalRepository(database).Save(context.Background(), sinais); err != nil {
		t.Fatalf("semear telemetria: %v", err)
	}
}

// semearCatalogosExpirados grava as coletas por SQL direto. O caminho de
// produção recalcula o diff a cada coleta, e este caso não mede o diff.
func semearCatalogosExpirados(t *testing.T, database *sql.DB, bases []string, porBase int) {
	t.Helper()
	for _, base := range bases {
		for indice := range porBase {
			if _, err := database.ExecContext(context.Background(), `
				INSERT INTO schema_snapshots (id, database_name, captured_at, objects_json)
				VALUES ($1, $2, $3, $4)
			`,
				fmt.Sprintf("snap-%s-%03d", base, indice), base,
				inicioRetencao.Add(time.Duration(indice)*time.Minute),
				`[{"kind":"column","name":"amount","table_name":"payments","detail":"integer"}]`,
			); err != nil {
				t.Fatalf("semear coleta de %q: %v", base, err)
			}
		}
	}
}
