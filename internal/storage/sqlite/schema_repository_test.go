package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
)

func openSchemaRepository(t *testing.T) *SchemaRepository {
	t.Helper()

	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "faultmap.db"))
	if err != nil {
		t.Fatalf("Open() erro = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Fatalf("Close() erro = %v", closeErr)
		}
	})
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() erro = %v", err)
	}
	return NewSchemaRepository(database)
}

func catalogo(capturedAt time.Time, objects ...changedomain.SchemaObject) changedomain.SchemaSnapshot {
	return changedomain.SchemaSnapshot{
		ID:           "snap:" + capturedAt.Format(time.RFC3339Nano),
		DatabaseName: "payments",
		CapturedAt:   capturedAt,
		Objects:      objects,
	}
}

var (
	primeiraColeta = time.Date(2026, time.September, 3, 8, 0, 0, 0, time.UTC)
	segundaColeta  = time.Date(2026, time.September, 3, 9, 0, 0, 0, time.UTC)
	terceiraColeta = time.Date(2026, time.September, 3, 10, 0, 0, 0, time.UTC)
)

// TestSchemaRepositoryPrimeiraColetaEstabeleceLinhaDeBase garante que instalar
// o produto e coletar pela primeira vez não acuse o schema inteiro como novo.
func TestSchemaRepositoryPrimeiraColetaEstabeleceLinhaDeBase(t *testing.T) {
	t.Parallel()

	repository := openSchemaRepository(t)
	result, err := repository.SaveSnapshot(context.Background(), catalogo(primeiraColeta,
		changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"},
	))
	if err != nil {
		t.Fatalf("SaveSnapshot() erro = %v", err)
	}
	if result.ObjectsCollected != 1 {
		t.Fatalf("objetos coletados = %d, esperado 1", result.ObjectsCollected)
	}
	if result.ChangesPersisted != 0 {
		t.Fatalf("mudanças = %d, esperado 0 na primeira coleta", result.ChangesPersisted)
	}
}

// TestSchemaRepositoryComparaComAColetaAnteriorDaMesmaBase é o caminho central:
// a segunda coleta produz as diferenças, que é o que o detector consome.
func TestSchemaRepositoryComparaComAColetaAnteriorDaMesmaBase(t *testing.T) {
	t.Parallel()

	repository := openSchemaRepository(t)
	ctx := context.Background()
	if _, err := repository.SaveSnapshot(ctx, catalogo(primeiraColeta,
		changedomain.SchemaObject{Kind: changedomain.SchemaObjectIndex, Name: "idx_payments_created_at"},
		changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"},
	)); err != nil {
		t.Fatalf("SaveSnapshot() primeira erro = %v", err)
	}

	result, err := repository.SaveSnapshot(ctx, catalogo(segundaColeta,
		changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "bigint"},
	))
	if err != nil {
		t.Fatalf("SaveSnapshot() segunda erro = %v", err)
	}
	if result.ChangesPersisted != 2 {
		t.Fatalf("mudanças = %d, esperado 2 (índice removido, tipo alterado)", result.ChangesPersisted)
	}

	changes, err := repository.ListSchemaChangesForScope(
		ctx, []string{"payments"}, nil, primeiraColeta, terceiraColeta, 100,
	)
	if err != nil {
		t.Fatalf("ListSchemaChangesForScope() erro = %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("mudanças lidas = %d, esperado 2: %#v", len(changes), changes)
	}
	// A ordem vem do produto, não da consulta: idx_ vem antes de payments.amount.
	if changes[0].ObjectName != "idx_payments_created_at" || changes[0].ChangeKind != changedomain.SchemaChangeRemoved {
		t.Fatalf("primeira mudança = %#v", changes[0])
	}
	if !changes[0].ObservedAfter.Equal(primeiraColeta) || !changes[0].ObservedBefore.Equal(segundaColeta) {
		t.Fatalf("intervalo = %v a %v, esperado %v a %v",
			changes[0].ObservedAfter, changes[0].ObservedBefore, primeiraColeta, segundaColeta)
	}
}

// TestSchemaRepositoryRecoletaSemMudancaNaoAcusaNada protege contra o pior
// falso positivo desta regra: rodar o comando de novo, sem migração alguma, e o
// diagnóstico seguinte apontar mudança de schema.
func TestSchemaRepositoryRecoletaSemMudancaNaoAcusaNada(t *testing.T) {
	t.Parallel()

	repository := openSchemaRepository(t)
	ctx := context.Background()
	objeto := changedomain.SchemaObject{
		Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer",
	}
	if _, err := repository.SaveSnapshot(ctx, catalogo(primeiraColeta, objeto)); err != nil {
		t.Fatalf("SaveSnapshot() primeira erro = %v", err)
	}

	result, err := repository.SaveSnapshot(ctx, catalogo(segundaColeta, objeto))
	if err != nil {
		t.Fatalf("SaveSnapshot() segunda erro = %v", err)
	}
	if result.ChangesPersisted != 0 {
		t.Fatalf("mudanças = %d, esperado 0 para catálogo idêntico", result.ChangesPersisted)
	}
}

// TestSchemaRepositoryNaoMisturaBasesDiferentes garante que a comparação seja
// sempre contra a coleta anterior da mesma base. Misturar faria duas bases com
// schemas distintos aparecerem como migração massiva a cada coleta alternada.
func TestSchemaRepositoryNaoMisturaBasesDiferentes(t *testing.T) {
	t.Parallel()

	repository := openSchemaRepository(t)
	ctx := context.Background()
	payments := catalogo(primeiraColeta,
		changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"},
	)
	catalog := changedomain.SchemaSnapshot{
		ID: "snap:catalog", DatabaseName: "catalog", CapturedAt: segundaColeta,
		Objects: []changedomain.SchemaObject{
			{Kind: changedomain.SchemaObjectColumn, Name: "produtos.nome", Detail: "text"},
		},
	}

	if _, err := repository.SaveSnapshot(ctx, payments); err != nil {
		t.Fatalf("SaveSnapshot() payments erro = %v", err)
	}
	result, err := repository.SaveSnapshot(ctx, catalog)
	if err != nil {
		t.Fatalf("SaveSnapshot() catalog erro = %v", err)
	}
	if result.ChangesPersisted != 0 {
		t.Fatalf("mudanças = %d, esperado 0: a primeira coleta de catalog não tem anterior", result.ChangesPersisted)
	}
}

// TestSchemaRepositoryFiltraPelaJanelaEPelaBase confirma que o detector receba
// somente o que pediu, sem varrer o histórico inteiro.
func TestSchemaRepositoryFiltraPelaJanelaEPelaBase(t *testing.T) {
	t.Parallel()

	repository := openSchemaRepository(t)
	ctx := context.Background()
	for _, snapshot := range []changedomain.SchemaSnapshot{
		catalogo(primeiraColeta, changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "integer"}),
		catalogo(segundaColeta, changedomain.SchemaObject{Kind: changedomain.SchemaObjectColumn, Name: "payments.amount", Detail: "bigint"}),
	} {
		if _, err := repository.SaveSnapshot(ctx, snapshot); err != nil {
			t.Fatalf("SaveSnapshot() erro = %v", err)
		}
	}

	fora, err := repository.ListSchemaChangesForScope(ctx, []string{"payments"}, nil, terceiraColeta, terceiraColeta.Add(time.Hour), 100)
	if err != nil {
		t.Fatalf("ListSchemaChangesForScope() erro = %v", err)
	}
	if len(fora) != 0 {
		t.Fatalf("mudanças fora da janela = %d, esperado 0", len(fora))
	}

	outraBase, err := repository.ListSchemaChangesForScope(ctx, []string{"catalog"}, nil, primeiraColeta, terceiraColeta, 100)
	if err != nil {
		t.Fatalf("ListSchemaChangesForScope() erro = %v", err)
	}
	if len(outraBase) != 0 {
		t.Fatalf("mudanças de outra base = %d, esperado 0", len(outraBase))
	}

	semBase, err := repository.ListSchemaChangesForScope(ctx, nil, nil, primeiraColeta, terceiraColeta, 100)
	if err != nil {
		t.Fatalf("ListSchemaChangesForScope() sem bases erro = %v", err)
	}
	if len(semBase) != 0 {
		t.Fatalf("mudanças sem base informada = %d, esperado 0", len(semBase))
	}
}
