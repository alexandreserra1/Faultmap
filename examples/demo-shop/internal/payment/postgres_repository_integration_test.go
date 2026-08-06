package payment

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresRepositoryIntegracao(t *testing.T) {
	databaseURL := os.Getenv("DEMO_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DEMO_DATABASE_URL não definido; teste de integração PostgreSQL ignorado")
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("sql.Open() erro = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar pool PostgreSQL de integração: %v", closeErr)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repository, err := NewPostgresRepository(database, RepositoryConfig{})
	if err != nil {
		t.Fatalf("NewPostgresRepository() erro = %v", err)
	}

	orderID := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := database.ExecContext(cleanupContext, `DELETE FROM payments WHERE order_id = $1`, orderID); cleanupErr != nil {
			t.Errorf("limpar pagamento de integração: %v", cleanupErr)
		}
	})
	valid := Payment{OrderID: orderID, AmountCents: 1990}
	if err := repository.Create(ctx, valid); err != nil {
		t.Fatalf("Create() erro = %v", err)
	}
	if err := repository.Create(ctx, valid); err != nil {
		t.Fatalf("Create() idempotente erro = %v", err)
	}

	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM payments WHERE order_id = $1`, orderID).Scan(&count); err != nil {
		t.Fatalf("contar pagamentos: %v", err)
	}
	if count != 1 {
		t.Fatalf("pagamentos para order_id = %d, esperado 1", count)
	}
	if err := repository.Create(ctx, Payment{OrderID: orderID + "-invalid", AmountCents: -1}); err == nil {
		t.Fatal("constraint amount_cents deveria rejeitar valor negativo")
	}
}

func TestPostgresRepositoryAtrasoOcupaConexaoDoPool(t *testing.T) {
	databaseURL := os.Getenv("DEMO_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DEMO_DATABASE_URL não definido; teste de integração PostgreSQL ignorado")
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("sql.Open() erro = %v", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("fechar pool PostgreSQL de integração: %v", closeErr)
		}
	})

	const simulatedDelay = 125 * time.Millisecond
	repository, err := NewPostgresRepository(database, RepositoryConfig{Delay: simulatedDelay})
	if err != nil {
		t.Fatalf("NewPostgresRepository() erro = %v", err)
	}
	orderIDs := []string{
		fmt.Sprintf("pool-integration-%d-a", time.Now().UnixNano()),
		fmt.Sprintf("pool-integration-%d-b", time.Now().UnixNano()),
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := database.ExecContext(cleanupContext, `DELETE FROM payments WHERE order_id = ANY($1)`, orderIDs); cleanupErr != nil {
			t.Errorf("limpar pagamentos do teste de pool: %v", cleanupErr)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	errorsChannel := make(chan error, len(orderIDs))
	for _, orderID := range orderIDs {
		orderID := orderID
		go func() {
			errorsChannel <- repository.Create(ctx, Payment{OrderID: orderID, AmountCents: 1990})
		}()
	}
	for range orderIDs {
		if createErr := <-errorsChannel; createErr != nil {
			t.Fatalf("Create() concorrente erro = %v", createErr)
		}
	}
	elapsed := time.Since(start)
	if elapsed < 2*simulatedDelay-25*time.Millisecond {
		t.Fatalf("duas operações com pool unitário levaram %s; atraso ocorreu fora da conexão", elapsed)
	}
}
