package application

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
)

type fonteDeSchemaFake struct {
	snapshot changedomain.SchemaSnapshot
	erro     error
	chamadas int
}

func (fake *fonteDeSchemaFake) Fetch(_ context.Context) (changedomain.SchemaSnapshot, error) {
	fake.chamadas++
	return fake.snapshot, fake.erro
}

type escritorDeSchemaFake struct {
	resultado SchemaImportResult
	erro      error
	chamadas  int
	recebido  changedomain.SchemaSnapshot
}

func (fake *escritorDeSchemaFake) SaveSnapshot(
	_ context.Context, snapshot changedomain.SchemaSnapshot,
) (SchemaImportResult, error) {
	fake.chamadas++
	fake.recebido = snapshot
	return fake.resultado, fake.erro
}

func coletaComDoisObjetos() changedomain.SchemaSnapshot {
	return changedomain.SchemaSnapshot{
		ID: "catalog:payments:1", DatabaseName: "payments",
		CapturedAt: time.Date(2026, time.September, 3, 9, 0, 0, 0, time.UTC),
		Objects: []changedomain.SchemaObject{
			{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"},
			{Kind: changedomain.SchemaObjectIndex, Name: "idx_payments_created_at"},
		},
	}
}

// TestIngestSchemaNaoPersisteQuandoAColetaFalha é a ordem que mantém a rede
// fora da transação: falhar ao ler o PostgreSQL não pode abrir transação no
// SQLite, nem gravar coleta pela metade.
func TestIngestSchemaNaoPersisteQuandoAColetaFalha(t *testing.T) {
	t.Parallel()

	falha := errors.New("conexão recusada")
	fonte := &fonteDeSchemaFake{erro: falha}
	escritor := &escritorDeSchemaFake{}

	_, err := IngestSchema(context.Background(), fonte, escritor)
	if err == nil {
		t.Fatal("IngestSchema() erro = nil com a coleta falhando")
	}
	if !errors.Is(err, falha) {
		t.Fatalf("erro = %v, esperado envolver a causa", err)
	}
	if escritor.chamadas != 0 {
		t.Fatalf("o escritor foi chamado %d vezes apesar da coleta ter falhado", escritor.chamadas)
	}
}

// TestIngestSchemaRelataFalhaDePersistencia garante que gravar mal não seja
// confundido com sucesso silencioso.
func TestIngestSchemaRelataFalhaDePersistencia(t *testing.T) {
	t.Parallel()

	falha := errors.New("banco somente leitura")
	_, err := IngestSchema(context.Background(),
		&fonteDeSchemaFake{snapshot: coletaComDoisObjetos()},
		&escritorDeSchemaFake{erro: falha},
	)
	if err == nil {
		t.Fatal("IngestSchema() erro = nil com a persistência falhando")
	}
	if !errors.Is(err, falha) {
		t.Fatalf("erro = %v, esperado envolver a causa", err)
	}
	if !strings.Contains(err.Error(), "persistir") {
		t.Fatalf("erro = %v, esperado distinguir a etapa que falhou", err)
	}
}

// TestIngestSchemaColetaAntesDePersistir fixa a ordem das etapas, que é o que
// impede uma consulta lenta ao PostgreSQL de segurar transação no SQLite.
func TestIngestSchemaColetaAntesDePersistir(t *testing.T) {
	t.Parallel()

	fonte := &fonteDeSchemaFake{snapshot: coletaComDoisObjetos()}
	escritor := &escritorDeSchemaFake{resultado: SchemaImportResult{ChangesPersisted: 3}}

	resultado, err := IngestSchema(context.Background(), fonte, escritor)
	if err != nil {
		t.Fatalf("IngestSchema() erro = %v", err)
	}
	if fonte.chamadas != 1 || escritor.chamadas != 1 {
		t.Fatalf("chamadas: fonte %d, escritor %d; esperado uma de cada", fonte.chamadas, escritor.chamadas)
	}
	if escritor.recebido.ID != fonte.snapshot.ID {
		t.Fatalf("o escritor recebeu %q, esperado a coleta da fonte %q", escritor.recebido.ID, fonte.snapshot.ID)
	}
	if resultado.ObjectsCollected != 2 {
		t.Fatalf("objetos coletados = %d, esperado 2", resultado.ObjectsCollected)
	}
	if resultado.ChangesPersisted != 3 {
		t.Fatalf("mudanças = %d, esperado preservar o que o escritor relatou", resultado.ChangesPersisted)
	}
}
