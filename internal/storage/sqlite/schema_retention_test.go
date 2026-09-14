package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
)

func repositorioComRetencao(t *testing.T) (*SchemaRepository, *RetentionRepository) {
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
	return NewSchemaRepository(database), NewRetentionRepository(database)
}

func coletaDe(base string, quando time.Time, colunas ...string) changedomain.SchemaSnapshot {
	objetos := make([]changedomain.SchemaObject, 0, len(colunas))
	for _, c := range colunas {
		objetos = append(objetos, changedomain.SchemaObject{
			Kind: changedomain.SchemaObjectColumn, Name: c, TableName: "payments", Detail: "integer",
		})
	}
	return changedomain.SchemaSnapshot{
		ID: base + ":" + quando.Format(time.RFC3339Nano), DatabaseName: base,
		CapturedAt: quando, Objects: objetos,
	}
}

// TestPruneLiberaOCatalogoAntigoSemApagarAsMudancas é a razão desta política
// existir. O que cresce é o JSON do catálogo — 640 KB por coleta numa base com
// 5.000 objetos, 64 GB por ano coletando de 5 em 5 minutos. As mudanças
// derivadas dele são minúsculas e sustentam evidência de diagnósticos já
// gravados: apagá-las tiraria do relatório o que ele afirma.
func TestPruneLiberaOCatalogoAntigoSemApagarAsMudancas(t *testing.T) {
	t.Parallel()

	schema, retencao := repositorioComRetencao(t)
	ctx := context.Background()
	base := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)

	for indice, colunas := range [][]string{
		{"payments.a", "payments.b"},
		{"payments.a"},
		{"payments.a", "payments.c"},
	} {
		if _, err := schema.SaveSnapshot(ctx, coletaDe("demo", base.Add(time.Duration(indice)*time.Hour), colunas...)); err != nil {
			t.Fatalf("SaveSnapshot(%d) erro = %v", indice, err)
		}
	}

	antes, err := retencao.CountSchemaChanges(ctx)
	if err != nil {
		t.Fatalf("CountSchemaChanges() erro = %v", err)
	}
	if antes == 0 {
		t.Fatal("o teste precisa de mudanças gravadas para provar que elas sobrevivem")
	}

	liberadas, err := retencao.PruneSchemaCatalogsBefore(ctx, base.Add(90*time.Minute), 100)
	if err != nil {
		t.Fatalf("PruneSchemaCatalogsBefore() erro = %v", err)
	}
	if liberadas == 0 {
		t.Fatal("nenhum catálogo foi liberado; a política não teve efeito")
	}

	depois, err := retencao.CountSchemaChanges(ctx)
	if err != nil {
		t.Fatalf("CountSchemaChanges() erro = %v", err)
	}
	if depois != antes {
		t.Fatalf("mudanças = %d após a limpeza, esperado %d: a evidência foi apagada junto", depois, antes)
	}
}

// TestPrunePreservaAColetaMaisRecenteDeCadaBase é a regra que impede o pior
// estrago possível. A coleta mais recente é a base de comparação da próxima: se
// ela for esvaziada, o diff seguinte compara contra um catálogo vazio e reporta
// **todo objeto da base como recém-criado** — uma migração inventada em cada
// tabela, no próximo incidente.
func TestPrunePreservaAColetaMaisRecenteDeCadaBase(t *testing.T) {
	t.Parallel()

	schema, retencao := repositorioComRetencao(t)
	ctx := context.Background()
	base := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)

	for _, nome := range []string{"demo", "payments"} {
		for indice := range 3 {
			if _, err := schema.SaveSnapshot(ctx, coletaDe(nome, base.Add(time.Duration(indice)*time.Hour), "payments.a")); err != nil {
				t.Fatalf("SaveSnapshot(%s,%d) erro = %v", nome, indice, err)
			}
		}
	}

	// Corte no futuro distante: sem a regra, TUDO seria esvaziado.
	if _, err := retencao.PruneSchemaCatalogsBefore(ctx, base.Add(365*24*time.Hour), 100); err != nil {
		t.Fatalf("PruneSchemaCatalogsBefore() erro = %v", err)
	}

	for _, nome := range []string{"demo", "payments"} {
		intactas, err := retencao.CountIntactCatalogs(ctx, nome)
		if err != nil {
			t.Fatalf("CountIntactCatalogs(%s) erro = %v", nome, err)
		}
		if intactas != 1 {
			t.Fatalf("base %s ficou com %d catálogos íntegros, esperado exatamente 1 (o mais recente)", nome, intactas)
		}
	}

	// E a coleta seguinte precisa continuar comparando corretamente: uma coluna
	// nova é uma mudança, não a criação do schema inteiro.
	resultado, err := schema.SaveSnapshot(ctx, coletaDe("demo", base.Add(4*time.Hour), "payments.a", "payments.nova"))
	if err != nil {
		t.Fatalf("SaveSnapshot() após limpeza erro = %v", err)
	}
	if resultado.ChangesPersisted != 1 {
		t.Fatalf("mudanças = %d após a limpeza, esperado 1: a comparação perdeu a linha de base", resultado.ChangesPersisted)
	}
}

// TestPruneAvancaEmLotes espelha a disciplina do DeleteSignalsBefore: nenhuma
// transação ilimitada no único escritor do SQLite.
func TestPruneAvancaEmLotes(t *testing.T) {
	t.Parallel()

	schema, retencao := repositorioComRetencao(t)
	ctx := context.Background()
	base := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	for indice := range 6 {
		if _, err := schema.SaveSnapshot(ctx, coletaDe("demo", base.Add(time.Duration(indice)*time.Hour), "payments.a")); err != nil {
			t.Fatalf("SaveSnapshot(%d) erro = %v", indice, err)
		}
	}

	primeiro, err := retencao.PruneSchemaCatalogsBefore(ctx, base.Add(365*24*time.Hour), 2)
	if err != nil {
		t.Fatalf("PruneSchemaCatalogsBefore() erro = %v", err)
	}
	if primeiro != 2 {
		t.Fatalf("primeiro lote liberou %d, esperado respeitar o limite de 2", primeiro)
	}
	if _, err := retencao.PruneSchemaCatalogsBefore(ctx, base.Add(365*24*time.Hour), 0); err == nil {
		t.Fatal("limite zero deveria ser recusado")
	}
}

// TestPruneEhIdempotente garante que repetir a limpeza não conte de novo o que
// já foi liberado — senão o relatório do comando mentiria a cada execução.
func TestPruneEhIdempotente(t *testing.T) {
	t.Parallel()

	schema, retencao := repositorioComRetencao(t)
	ctx := context.Background()
	base := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	for indice := range 4 {
		if _, err := schema.SaveSnapshot(ctx, coletaDe("demo", base.Add(time.Duration(indice)*time.Hour), "payments.a")); err != nil {
			t.Fatalf("SaveSnapshot(%d) erro = %v", indice, err)
		}
	}

	corte := base.Add(365 * 24 * time.Hour)
	if _, err := retencao.PruneSchemaCatalogsBefore(ctx, corte, 100); err != nil {
		t.Fatalf("primeira limpeza erro = %v", err)
	}
	repetida, err := retencao.PruneSchemaCatalogsBefore(ctx, corte, 100)
	if err != nil {
		t.Fatalf("segunda limpeza erro = %v", err)
	}
	if repetida != 0 {
		t.Fatalf("a repetição liberou %d catálogos, esperado 0", repetida)
	}
}

// TestColetaContraLinhaDeBaseEsvaziadaFalhaEmVezDeInventarMigracao é defesa em
// profundidade. A regra da coleta mais recente já impede o caso, mas se ele
// acontecer — banco editado à mão, defeito futuro — reportar "todo o schema foi
// criado agora" seria pior que falhar.
func TestColetaContraLinhaDeBaseEsvaziadaFalhaEmVezDeInventarMigracao(t *testing.T) {
	t.Parallel()

	schema, _ := repositorioComRetencao(t)
	ctx := context.Background()
	base := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	if _, err := schema.SaveSnapshot(ctx, coletaDe("demo", base, "payments.a", "payments.b")); err != nil {
		t.Fatalf("SaveSnapshot() erro = %v", err)
	}
	// Esvaziado direto no banco, e não por um método de produção: o cenário é
	// justamente o que a política impede, então não existe caminho legítimo
	// para chegar nele — e abrir um só para o teste colocaria em produção código
	// que só o teste usa.
	if _, err := schema.database.ExecContext(ctx,
		`UPDATE schema_snapshots SET objects_json = '' WHERE database_name = ?`, "demo",
	); err != nil {
		t.Fatalf("esvaziar catálogo: %v", err)
	}

	_, err := schema.SaveSnapshot(ctx, coletaDe("demo", base.Add(time.Hour), "payments.a", "payments.b"))
	if err == nil {
		t.Fatal("comparar contra linha de base esvaziada não falhou; inventaria migração em toda tabela")
	}
}
