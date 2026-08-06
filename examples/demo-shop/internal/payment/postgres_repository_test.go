package payment

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestPostgresRepositoryCreateUsaConsultaParametrizada(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() erro = %v", err)
	}
	defer database.Close()

	mock.ExpectExec(`INSERT INTO payments \(order_id, amount_cents\) VALUES \(\$1, \$2\)`).
		WithArgs("order-1", int64(1990)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repository := mustPostgresRepository(t, database, RepositoryConfig{})
	if err := repository.Create(context.Background(), Payment{OrderID: "order-1", AmountCents: 1990}); err != nil {
		t.Fatalf("Create() erro = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectativas SQL: %v", err)
	}
}

func TestPostgresRepositoryRetornaErroComCausa(t *testing.T) {
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() erro = %v", err)
	}
	defer database.Close()

	mock.ExpectExec(`INSERT INTO payments`).WillReturnError(sql.ErrConnDone)
	repository := mustPostgresRepository(t, database, RepositoryConfig{})
	err = repository.Create(context.Background(), Payment{OrderID: "order-1", AmountCents: 1990})
	if !errors.Is(err, sql.ErrConnDone) {
		t.Fatalf("Create() erro = %v, esperado causa %v", err, sql.ErrConnDone)
	}
}

func TestPostgresRepositoryAtrasoRespeitaContexto(t *testing.T) {
	database, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() erro = %v", err)
	}
	defer database.Close()

	repository := mustPostgresRepository(t, database, RepositoryConfig{Delay: time.Minute})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = repository.Create(ctx, Payment{OrderID: "order-1", AmountCents: 1990})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Create() erro = %v, esperado context.Canceled", err)
	}
	if err == nil {
		t.Fatal("Create() deveria retornar erro")
	}
}

func TestPostgresRepositoryMarcaTimeoutComAtributoSeguro(t *testing.T) {
	database, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() erro = %v", err)
	}
	defer database.Close()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	oldProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(oldProvider)
	repository := mustPostgresRepository(t, database, RepositoryConfig{Delay: time.Minute})
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := repository.Create(ctx, Payment{OrderID: "order-timeout", AmountCents: 1990}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Create() erro = %v, esperado deadline", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, esperado 1", len(spans))
	}
	found := false
	for _, item := range spans[0].Attributes() {
		if string(item.Key) == "error.type" && item.Value.AsString() == "timeout" {
			found = true
		}
	}
	if !found {
		t.Fatalf("span não contém error.type=timeout: %+v", spans[0].Attributes())
	}
}

func TestNewPostgresRepositoryRejeitaPoolAusente(t *testing.T) {
	t.Parallel()

	if _, err := NewPostgresRepository(nil, RepositoryConfig{}); err == nil {
		t.Fatal("NewPostgresRepository() deveria rejeitar pool ausente")
	}
}

func mustPostgresRepository(t *testing.T, database *sql.DB, config RepositoryConfig) *PostgresRepository {
	t.Helper()
	repository, err := NewPostgresRepository(database, config)
	if err != nil {
		t.Fatalf("NewPostgresRepository() erro = %v", err)
	}
	return repository
}
