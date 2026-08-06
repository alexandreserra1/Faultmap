package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RepositoryConfig controla somente comportamentos necessários aos cenários da demonstração.
type RepositoryConfig struct {
	Delay time.Duration
}

// PostgresRepository persiste pagamentos usando o pool compartilhado da aplicação.
type PostgresRepository struct {
	database *sql.DB
	delay    time.Duration
	tracer   trace.Tracer
}

// NewPostgresRepository injeta o pool criado no bootstrap; o repositório nunca abre conexões próprias.
func NewPostgresRepository(database *sql.DB, config RepositoryConfig) (*PostgresRepository, error) {
	if database == nil {
		return nil, fmt.Errorf("criar repositório PostgreSQL: pool é obrigatório")
	}
	if config.Delay < 0 {
		return nil, fmt.Errorf("criar repositório PostgreSQL: atraso não pode ser negativo")
	}
	return &PostgresRepository{
		database: database,
		delay:    config.Delay,
		tracer:   otel.Tracer("demo-shop/payment/repository"),
	}, nil
}

// Create registra o pagamento de modo idempotente por order_id e emite somente atributos DB seguros.
func (repository *PostgresRepository) Create(ctx context.Context, payment Payment) (createErr error) {
	ctx, span := repository.tracer.Start(ctx, "INSERT payments", trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(
		attribute.String("db.system.name", "postgresql"),
		attribute.String("db.operation.name", "INSERT"),
		attribute.String("db.collection.name", "payments"),
	))
	defer span.End()

	execContext := repository.database.ExecContext
	if repository.delay > 0 {
		// O atraso controlado ocorre depois do checkout da conexão para que o
		// cenário de pool reduzido reproduza espera real no *sql.DB compartilhado.
		connection, err := repository.database.Conn(ctx)
		if err != nil {
			recordDatabaseSpanError(span, err)
			return fmt.Errorf("obter conexão PostgreSQL do pool: %w", err)
		}
		defer func() {
			if closeErr := connection.Close(); closeErr != nil {
				recordDatabaseSpanError(span, closeErr)
				createErr = errors.Join(createErr, fmt.Errorf("devolver conexão PostgreSQL ao pool: %w", closeErr))
			}
		}()
		execContext = connection.ExecContext

		timer := time.NewTimer(repository.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			recordDatabaseSpanError(span, ctx.Err())
			return fmt.Errorf("aguardar atraso simulado do banco: %w", ctx.Err())
		case <-timer.C:
		}
	}
	const query = `INSERT INTO payments (order_id, amount_cents) VALUES ($1, $2) ON CONFLICT (order_id) DO NOTHING`
	if _, err := execContext(ctx, query, payment.OrderID, payment.AmountCents); err != nil {
		recordDatabaseSpanError(span, err)
		return fmt.Errorf("inserir pagamento: %w", err)
	}
	return nil
}

// recordDatabaseSpanError classifica somente categorias operacionais seguras;
// a mensagem original continua na cadeia do erro, mas não vira atributo.
func recordDatabaseSpanError(span trace.Span, err error) {
	span.RecordError(err)
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
		span.SetAttributes(attribute.String("error.type", "timeout"))
		span.SetStatus(codes.Error, "database timeout")
		return
	}
	if errors.Is(err, context.Canceled) {
		span.SetAttributes(attribute.String("error.type", "canceled"))
		span.SetStatus(codes.Error, "database operation canceled")
		return
	}
	span.SetAttributes(attribute.String("error.type", "database_error"))
	span.SetStatus(codes.Error, "database operation failed")
}
