package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faultmap/faultmap/internal/integrations/postgres"
)

func clienteComMock(t *testing.T) (*postgres.Client, sqlmock.Sqlmock) {
	t.Helper()

	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() erro = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	client, err := postgres.NewClient(database, "payments", func() time.Time { return coletadoEm })
	if err != nil {
		t.Fatalf("NewClient() erro = %v", err)
	}
	return client, mock
}

// TestFetchPropagaFalhaDeCadaConsulta cobre as três consultas: uma permissão
// negada no catálogo ou uma conexão caída no meio precisa virar erro nomeado, e
// não uma coleta parcial. Coleta parcial seria pior que falha: ela viraria
// "todo o resto do schema foi removido" na comparação seguinte.
func TestFetchPropagaFalhaDeCadaConsulta(t *testing.T) {
	t.Parallel()

	falha := errors.New("permissão negada")
	for nome, programar := range map[string]func(sqlmock.Sqlmock){
		"colunas": func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery("information_schema.columns").WillReturnError(falha)
		},
		"índices": func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery("information_schema.columns").WillReturnRows(linhasVazias("table_schema", "table_name", "column_name", "data_type", "is_nullable", "column_default"))
			mock.ExpectQuery("pg_indexes").WillReturnError(falha)
		},
		"restrições": func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery("information_schema.columns").WillReturnRows(linhasVazias("table_schema", "table_name", "column_name", "data_type", "is_nullable", "column_default"))
			mock.ExpectQuery("pg_indexes").WillReturnRows(linhasVazias("schemaname", "indexname"))
			mock.ExpectQuery("information_schema.table_constraints").WillReturnError(falha)
		},
	} {
		client, mock := clienteComMock(t)
		programar(mock)

		snapshot, err := client.Fetch(context.Background())
		if err == nil {
			t.Fatalf("%s: Fetch() erro = nil; uma coleta parcial viraria remoção em massa", nome)
		}
		if !errors.Is(err, falha) {
			t.Fatalf("%s: erro = %v, esperado envolver a causa original", nome, err)
		}
		if !strings.Contains(err.Error(), "payments") {
			t.Fatalf("%s: erro = %v, esperado nomear a base", nome, err)
		}
		if len(snapshot.Objects) != 0 {
			t.Fatalf("%s: Fetch() devolveu %d objetos junto do erro", nome, len(snapshot.Objects))
		}
	}
}

// TestFetchFalhaQuandoOCatalogoDevolveTipoInesperado protege contra uma versão
// futura do PostgreSQL, ou um pooler que reescreve a consulta, mudando a forma
// da resposta. Silenciar isso produziria coleta vazia, que é remoção em massa.
func TestFetchFalhaQuandoOCatalogoDevolveTipoInesperado(t *testing.T) {
	t.Parallel()

	client, mock := clienteComMock(t)
	mock.ExpectQuery("information_schema.columns").WillReturnRows(
		sqlmock.NewRows([]string{"table_schema"}).AddRow("public"),
	)

	if _, err := client.Fetch(context.Background()); err == nil {
		t.Fatal("Fetch() aceitou linha com colunas de menos")
	}
}

// TestFetchPropagaFalhaNoMeioDaLeitura cobre a conexão que cai depois de a
// consulta ter começado a devolver linhas — o caso em que rows.Err() é a única
// coisa entre um erro e uma coleta truncada silenciosa.
func TestFetchPropagaFalhaNoMeioDaLeitura(t *testing.T) {
	t.Parallel()

	client, mock := clienteComMock(t)
	mock.ExpectQuery("information_schema.columns").WillReturnRows(
		sqlmock.NewRows([]string{"table_schema", "table_name", "column_name", "data_type", "is_nullable", "column_default"}).
			AddRow("public", "payments", "amount", "integer", "NO", nil).
			RowError(0, errors.New("conexão perdida")),
	)

	if _, err := client.Fetch(context.Background()); err == nil {
		t.Fatal("Fetch() ignorou falha no meio da leitura e devolveria coleta truncada")
	}
}

// TestFetchRespeitaContextoCancelado garante que um Ctrl+C durante a coleta não
// deixe a conexão trabalhando.
func TestFetchRespeitaContextoCancelado(t *testing.T) {
	t.Parallel()

	client, mock := clienteComMock(t)
	mock.ExpectQuery("information_schema.columns").WillReturnError(context.Canceled)

	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	if _, err := client.Fetch(ctx); err == nil {
		t.Fatal("Fetch() ignorou o contexto cancelado")
	}
}
