package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	"github.com/faultmap/faultmap/internal/integrations/postgres"
)

var coletadoEm = time.Date(2026, time.September, 3, 9, 30, 0, 0, time.UTC)

// esperarCatalogo programa as três consultas do coletor na ordem em que ele as
// executa: colunas, índices e restrições.
func esperarCatalogo(mock sqlmock.Sqlmock, colunas, indices, restricoes *sqlmock.Rows) {
	mock.ExpectQuery("information_schema.columns").WillReturnRows(colunas)
	mock.ExpectQuery("pg_indexes").WillReturnRows(indices)
	mock.ExpectQuery("information_schema.table_constraints").WillReturnRows(restricoes)
}

func linhasDeColuna() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"table_schema", "table_name", "column_name", "data_type", "is_nullable", "column_default"}).
		AddRow("public", "payments", "amount", "integer", "NO", nil).
		AddRow("public", "payments", "plano", "text", "YES", "'plano-gratuito'::text")
}

func linhasVazias(colunas ...string) *sqlmock.Rows {
	return sqlmock.NewRows(colunas)
}

func coletar(t *testing.T, colunas, indices, restricoes *sqlmock.Rows) changedomain.SchemaSnapshot {
	t.Helper()

	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() erro = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	esperarCatalogo(mock, colunas, indices, restricoes)

	client, err := postgres.NewClient(database, "payments", func() time.Time { return coletadoEm })
	if err != nil {
		t.Fatalf("NewClient() erro = %v", err)
	}
	snapshot, err := client.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() erro = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("consultas esperadas não foram executadas: %v", err)
	}
	return snapshot
}

// TestFetchNormalizaColunasIndicesERestricoes cobre o mapeamento das três
// consultas para a forma que o resto do produto entende.
func TestFetchNormalizaColunasIndicesERestricoes(t *testing.T) {
	t.Parallel()

	snapshot := coletar(t,
		linhasDeColuna(),
		sqlmock.NewRows([]string{"schemaname", "indexname"}).AddRow("public", "idx_payments_created_at"),
		sqlmock.NewRows([]string{"table_schema", "constraint_name", "constraint_type"}).AddRow("public", "payments_pkey", "PRIMARY KEY"),
	)

	if snapshot.DatabaseName != "payments" || !snapshot.CapturedAt.Equal(coletadoEm) {
		t.Fatalf("coleta = %#v", snapshot)
	}
	if strings.TrimSpace(snapshot.ID) == "" {
		t.Fatal("coleta sem ID: a persistência não conseguiria referenciá-la")
	}

	porNome := make(map[string]changedomain.SchemaObject, len(snapshot.Objects))
	for _, object := range snapshot.Objects {
		porNome[object.Name] = object
	}
	for nome, kind := range map[string]changedomain.SchemaObjectKind{
		"public.payments.amount":         changedomain.SchemaObjectColumn,
		"public.payments.plano":          changedomain.SchemaObjectColumn,
		"public.idx_payments_created_at": changedomain.SchemaObjectIndex,
		"public.payments_pkey":           changedomain.SchemaObjectConstraint,
	} {
		object, existe := porNome[nome]
		if !existe {
			t.Fatalf("objeto %q ausente na coleta: %#v", nome, snapshot.Objects)
		}
		if object.Kind != kind {
			t.Fatalf("objeto %q com tipo %q, esperado %q", nome, object.Kind, kind)
		}
	}
	if !strings.Contains(porNome["public.payments.amount"].Detail, "integer") {
		t.Fatalf("detalhe da coluna = %q, esperado conter o tipo", porNome["public.payments.amount"].Detail)
	}
}

// TestFetchNaoGuardaOTextoDoDefault é a ADR 0014 no ponto onde o dado entra.
// Se a expressão vazasse aqui, todas as camadas seguintes a guardariam.
func TestFetchNaoGuardaOTextoDoDefault(t *testing.T) {
	t.Parallel()

	snapshot := coletar(t,
		linhasDeColuna(),
		linhasVazias("schemaname", "indexname"),
		linhasVazias("table_schema", "constraint_name", "constraint_type"),
	)

	for _, object := range snapshot.Objects {
		for _, vazamento := range []string{"plano-gratuito", "'plano-gratuito'::text"} {
			if strings.Contains(object.Detail, vazamento) {
				t.Fatalf("objeto %q vazou a expressão de default: %q", object.Name, object.Detail)
			}
		}
	}

	var plano changedomain.SchemaObject
	for _, object := range snapshot.Objects {
		if object.Name == "public.payments.plano" {
			plano = object
		}
	}
	if plano.ExpressionDigest == "" {
		t.Fatal("coluna com default deveria ter resumo da expressão, para que a mudança seja detectável")
	}
	if strings.Contains(plano.ExpressionDigest, "plano") {
		t.Fatalf("o resumo contém a expressão: %q", plano.ExpressionDigest)
	}

	var amount changedomain.SchemaObject
	for _, object := range snapshot.Objects {
		if object.Name == "public.payments.amount" {
			amount = object
		}
	}
	if amount.ExpressionDigest != "" {
		t.Fatalf("coluna sem default deveria ter resumo vazio, tem %q", amount.ExpressionDigest)
	}
}

// TestFetchDevolveObjetosEmOrdemEstavel protege a reprodutibilidade já na
// entrada: a coleta é gravada como JSON e comparada com a próxima, e uma ordem
// instável produziria diffs falsos a cada execução.
func TestFetchDevolveObjetosEmOrdemEstavel(t *testing.T) {
	t.Parallel()

	primeira := coletar(t,
		sqlmock.NewRows([]string{"table_schema", "table_name", "column_name", "data_type", "is_nullable", "column_default"}).
			AddRow("public", "payments", "amount", "integer", "NO", nil).
			AddRow("public", "refunds", "id", "uuid", "NO", nil),
		sqlmock.NewRows([]string{"schemaname", "indexname"}).AddRow("public", "idx_b").AddRow("public", "idx_a"),
		linhasVazias("table_schema", "constraint_name", "constraint_type"),
	)
	segunda := coletar(t,
		sqlmock.NewRows([]string{"table_schema", "table_name", "column_name", "data_type", "is_nullable", "column_default"}).
			AddRow("public", "refunds", "id", "uuid", "NO", nil).
			AddRow("public", "payments", "amount", "integer", "NO", nil),
		sqlmock.NewRows([]string{"schemaname", "indexname"}).AddRow("public", "idx_a").AddRow("public", "idx_b"),
		linhasVazias("table_schema", "constraint_name", "constraint_type"),
	)

	if len(primeira.Objects) != len(segunda.Objects) {
		t.Fatalf("coletas com tamanhos diferentes: %d e %d", len(primeira.Objects), len(segunda.Objects))
	}
	for posicao := range primeira.Objects {
		if primeira.Objects[posicao] != segunda.Objects[posicao] {
			t.Fatalf("posição %d divergiu: %#v e %#v", posicao, primeira.Objects[posicao], segunda.Objects[posicao])
		}
	}
	if changedomain.Diff(primeira, segunda) != nil && len(changedomain.Diff(
		changedomain.SchemaSnapshot{ID: "a", DatabaseName: "payments", CapturedAt: coletadoEm, Objects: primeira.Objects},
		segunda,
	)) != 0 {
		t.Fatal("duas coletas do mesmo catálogo produziram diferenças")
	}
}

// TestNewClientExigeBaseNomeada impede uma coleta anônima, que a persistência
// não conseguiria comparar com a anterior nem o detector ligar a um serviço.
func TestNewClientExigeBaseNomeada(t *testing.T) {
	t.Parallel()

	database, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() erro = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if _, err := postgres.NewClient(database, "   ", nil); err == nil {
		t.Fatal("NewClient() aceitou base sem nome")
	}
	if _, err := postgres.NewClient(nil, "payments", nil); err == nil {
		t.Fatal("NewClient() aceitou conexão nula")
	}
}

// TestFetchIgnoraConstraintsInternasDeNotNull vem de um achado do teste contra
// um PostgreSQL real: o servidor nomeia as restrições implícitas de NOT NULL
// com OIDs, como "2200_16385_1_not_null". O OID muda quando a tabela é
// recriada, então recriar uma tabela sem alterar nada apareceria como restrição
// removida e outra adicionada. A informação também é redundante: a nulabilidade
// já viaja no detalhe da coluna.
func TestFetchIgnoraConstraintsInternasDeNotNull(t *testing.T) {
	t.Parallel()

	snapshot := coletar(t,
		linhasVazias("table_schema", "table_name", "column_name", "data_type", "is_nullable", "column_default"),
		linhasVazias("schemaname", "indexname"),
		sqlmock.NewRows([]string{"table_schema", "constraint_name", "constraint_type"}).
			AddRow("public", "2200_16385_1_not_null", "CHECK").
			AddRow("public", "payments_pkey", "PRIMARY KEY"),
	)

	for _, object := range snapshot.Objects {
		if strings.HasSuffix(object.Name, "_not_null") {
			t.Fatalf("restrição interna do PostgreSQL entrou na coleta: %#v", object)
		}
	}
	if len(snapshot.Objects) != 1 || snapshot.Objects[0].Name != "public.payments_pkey" {
		t.Fatalf("objetos = %#v, esperado apenas a chave primária", snapshot.Objects)
	}
}

// TestFetchQualificaObjetosPeloSchema cobre um banco com mais de um schema, que
// é o normal em PostgreSQL: `public` mais os schemas da aplicação, ou um schema
// por inquilino.
//
// Sem o schema no nome, `public.pedidos.valor` e `tenant_a.pedidos.valor` viram
// a mesma chave na comparação. O efeito é pior que perder informação: uma
// mudança em um schema mascara a do outro, e dropar uma tabela que existe com o
// mesmo nome em outro schema aparece como "nada mudou".
func TestFetchQualificaObjetosPeloSchema(t *testing.T) {
	t.Parallel()

	snapshot := coletar(t,
		sqlmock.NewRows([]string{"table_schema", "table_name", "column_name", "data_type", "is_nullable", "column_default"}).
			AddRow("public", "pedidos", "valor", "integer", "NO", nil).
			AddRow("tenant_a", "pedidos", "valor", "bigint", "NO", nil),
		sqlmock.NewRows([]string{"schemaname", "indexname"}).
			AddRow("public", "idx_pedidos_valor").
			AddRow("tenant_a", "idx_pedidos_valor"),
		sqlmock.NewRows([]string{"table_schema", "constraint_name", "constraint_type"}).
			AddRow("public", "pedidos_pkey", "PRIMARY KEY").
			AddRow("tenant_a", "pedidos_pkey", "PRIMARY KEY"),
	)

	nomes := make(map[string]struct{}, len(snapshot.Objects))
	for _, object := range snapshot.Objects {
		if _, repetido := nomes[object.Name]; repetido {
			t.Fatalf("dois objetos com o mesmo nome %q: schemas diferentes colidiram", object.Name)
		}
		nomes[object.Name] = struct{}{}
	}
	if len(snapshot.Objects) != 6 {
		t.Fatalf("objetos = %d, esperado 6 (2 colunas, 2 índices, 2 restrições): %#v", len(snapshot.Objects), snapshot.Objects)
	}
	for _, esperado := range []string{
		"public.pedidos.valor", "tenant_a.pedidos.valor",
		"public.idx_pedidos_valor", "tenant_a.idx_pedidos_valor",
		"public.pedidos_pkey", "tenant_a.pedidos_pkey",
	} {
		if _, existe := nomes[esperado]; !existe {
			t.Fatalf("objeto %q ausente: %#v", esperado, nomes)
		}
	}
}
