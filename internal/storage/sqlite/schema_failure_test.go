package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
)

// TestSaveSnapshotRecusaColetaVaziaContraCatalogoPovoado é a falha mais
// perigosa desta funcionalidade, e ela não produz erro nenhum no PostgreSQL:
// information_schema filtra por privilégio. Se o usuário do Faultmap perder
// SELECT nas tabelas, a consulta devolve zero linhas com sucesso, e a
// comparação leria isso como "o schema inteiro foi removido".
//
// O efeito seria o pior possível: no incidente seguinte, todo serviço que fala
// com aquela base apareceria acusado por mudança de schema, com evidência
// inventada por uma permissão revogada.
func TestSaveSnapshotRecusaColetaVaziaContraCatalogoPovoado(t *testing.T) {
	t.Parallel()

	repository := openSchemaRepository(t)
	ctx := context.Background()
	if _, err := repository.SaveSnapshot(ctx, catalogo(primeiraColeta,
		changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"},
		changedomain.SchemaObject{Kind: changedomain.SchemaObjectIndex, Name: "idx_payments_created_at"},
	)); err != nil {
		t.Fatalf("SaveSnapshot() primeira erro = %v", err)
	}

	_, err := repository.SaveSnapshot(ctx, catalogo(segundaColeta))
	if err == nil {
		t.Fatal("SaveSnapshot() aceitou coleta vazia e registraria remoção em massa")
	}
	if !strings.Contains(err.Error(), "vazia") {
		t.Fatalf("erro = %v, esperado explicar que a coleta veio vazia", err)
	}

	// A recusa precisa deixar o banco intacto: nenhuma mudança gravada e a
	// coleta anterior preservada como linha de base para a próxima tentativa.
	changes, err := repository.ListSchemaChangesForDatabases(ctx, []string{"payments"}, primeiraColeta, terceiraColeta, 100)
	if err != nil {
		t.Fatalf("ListSchemaChangesForDatabases() erro = %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("a recusa deixou %d mudanças gravadas: %#v", len(changes), changes)
	}
}

// TestSaveSnapshotAceitaBaseVaziaDesdeOInicio distingue perda de privilégio de
// uma base que realmente não tem nada: se a anterior também estava vazia, não
// há remoção alguma a inventar.
func TestSaveSnapshotAceitaBaseVaziaDesdeOInicio(t *testing.T) {
	t.Parallel()

	repository := openSchemaRepository(t)
	ctx := context.Background()
	if _, err := repository.SaveSnapshot(ctx, catalogo(primeiraColeta)); err != nil {
		t.Fatalf("SaveSnapshot() primeira erro = %v", err)
	}
	if _, err := repository.SaveSnapshot(ctx, catalogo(segundaColeta)); err != nil {
		t.Fatalf("SaveSnapshot() segunda erro = %v", err)
	}
}

// TestSaveSnapshotRejeitaColetaSemIdentidade impede que uma coleta incompleta
// chegue ao disco, onde não poderia ser comparada nem referenciada.
func TestSaveSnapshotRejeitaColetaSemIdentidade(t *testing.T) {
	t.Parallel()

	repository := openSchemaRepository(t)
	valida := catalogo(primeiraColeta,
		changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"},
	)
	for nome, snapshot := range map[string]changedomain.SchemaSnapshot{
		"sem ID":       {DatabaseName: valida.DatabaseName, CapturedAt: valida.CapturedAt, Objects: valida.Objects},
		"sem base":     {ID: valida.ID, CapturedAt: valida.CapturedAt, Objects: valida.Objects},
		"sem instante": {ID: valida.ID, DatabaseName: valida.DatabaseName, Objects: valida.Objects},
	} {
		if _, err := repository.SaveSnapshot(context.Background(), snapshot); err == nil {
			t.Fatalf("%s: SaveSnapshot() aceitou coleta incompleta", nome)
		}
	}
}

// TestSaveSnapshotFalhaComBancoFechado cobre o SQLite indisponível: o comando
// precisa relatar, e não seguir como se tivesse gravado.
func TestSaveSnapshotFalhaComBancoFechado(t *testing.T) {
	t.Parallel()

	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "faultmap.db"))
	if err != nil {
		t.Fatalf("Open() erro = %v", err)
	}
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() erro = %v", err)
	}
	repository := NewSchemaRepository(database)
	if err := database.Close(); err != nil {
		t.Fatalf("Close() erro = %v", err)
	}

	if _, err := repository.SaveSnapshot(context.Background(), catalogo(primeiraColeta,
		changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"},
	)); err == nil {
		t.Fatal("SaveSnapshot() não relatou o banco fechado")
	}
	if _, err := repository.ListSchemaChangesForDatabases(
		context.Background(), []string{"payments"}, primeiraColeta, terceiraColeta, 10,
	); err == nil {
		t.Fatal("ListSchemaChangesForDatabases() não relatou o banco fechado")
	}
}

// TestSaveSnapshotRespeitaContextoCancelado garante que um Ctrl+C não deixe
// transação pendurada no único escritor do SQLite.
func TestSaveSnapshotRespeitaContextoCancelado(t *testing.T) {
	t.Parallel()

	repository := openSchemaRepository(t)
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()

	if _, err := repository.SaveSnapshot(ctx, catalogo(primeiraColeta,
		changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"},
	)); err == nil {
		t.Fatal("SaveSnapshot() ignorou o contexto cancelado")
	}
}

// TestSaveSnapshotFalhaComColetaAnteriorCorrompida cobre o JSON ilegível no
// disco — corrupção, ou um formato gravado por versão futura. Tratar como
// "sem coleta anterior" seria pior: viraria linha de base silenciosa e a
// migração seguinte passaria despercebida.
func TestSaveSnapshotFalhaComColetaAnteriorCorrompida(t *testing.T) {
	t.Parallel()

	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "faultmap.db"))
	if err != nil {
		t.Fatalf("Open() erro = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() erro = %v", err)
	}
	if _, err := database.ExecContext(context.Background(), `
		INSERT INTO schema_snapshots (id, database_name, captured_at, objects_json)
		VALUES (?, ?, ?, ?)
	`, "snap:corrompida", "payments", primeiraColeta, "{isso não é json}"); err != nil {
		t.Fatalf("preparar coleta corrompida: %v", err)
	}

	_, err = NewSchemaRepository(database).SaveSnapshot(context.Background(), catalogo(segundaColeta,
		changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"},
	))
	if err == nil {
		t.Fatal("SaveSnapshot() ignorou a coleta anterior corrompida e criaria linha de base silenciosa")
	}

	var gravadas int
	if err := database.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM schema_changes`).Scan(&gravadas); err != nil {
		t.Fatalf("contar mudanças: %v", err)
	}
	if gravadas != 0 {
		t.Fatalf("a falha deixou %d mudanças gravadas", gravadas)
	}
}

// TestListSchemaChangesLimitaAConsulta impede que um limite absurdo vindo da
// configuração transforme a leitura em varredura do histórico inteiro.
func TestListSchemaChangesLimitaAConsulta(t *testing.T) {
	t.Parallel()

	repository := openSchemaRepository(t)
	ctx := context.Background()
	objetos := make([]changedomain.SchemaObject, 0, 40)
	for indice := range 40 {
		objetos = append(objetos, changedomain.SchemaObject{
			Kind: changedomain.SchemaObjectColumn,
			Name: "payments.coluna_" + string(rune('a'+indice%26)) + string(rune('a'+indice/26)),
		})
	}
	if _, err := repository.SaveSnapshot(ctx, catalogo(primeiraColeta, objetos...)); err != nil {
		t.Fatalf("SaveSnapshot() primeira erro = %v", err)
	}
	if _, err := repository.SaveSnapshot(ctx, catalogo(segundaColeta, objetos[:1]...)); err != nil {
		t.Fatalf("SaveSnapshot() segunda erro = %v", err)
	}

	limitadas, err := repository.ListSchemaChangesForDatabases(ctx, []string{"payments"}, primeiraColeta, terceiraColeta, 5)
	if err != nil {
		t.Fatalf("ListSchemaChangesForDatabases() erro = %v", err)
	}
	if len(limitadas) != 5 {
		t.Fatalf("mudanças = %d, esperado respeitar o limite de 5", len(limitadas))
	}

	// Limite absurdo cai para o teto interno em vez de varrer tudo.
	comTeto, err := repository.ListSchemaChangesForDatabases(ctx, []string{"payments"}, primeiraColeta, terceiraColeta, 1<<30)
	if err != nil {
		t.Fatalf("ListSchemaChangesForDatabases() com limite absurdo erro = %v", err)
	}
	if len(comTeto) != 39 {
		t.Fatalf("mudanças = %d, esperado as 39 remoções", len(comTeto))
	}
}

// TestSaveSnapshotRecusaColetaGigante protege a memória do processo contra uma
// base com um número irreal de objetos.
func TestSaveSnapshotRecusaColetaGigante(t *testing.T) {
	t.Parallel()

	objetos := make([]changedomain.SchemaObject, maxSchemaObjectsPerSnapshot+1)
	for indice := range objetos {
		objetos[indice] = changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "t.c"}
	}
	if _, err := openSchemaRepository(t).SaveSnapshot(context.Background(), catalogo(primeiraColeta, objetos...)); err == nil {
		t.Fatal("SaveSnapshot() aceitou coleta acima do teto")
	}
}

var _ = sql.ErrNoRows
var _ = time.Second
