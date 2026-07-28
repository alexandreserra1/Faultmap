package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

const (
	busyTimeoutStatement = "PRAGMA busy_timeout = 5000"
	maxOpenConnections   = 1
	maxIdleConnections   = 1
)

// Open cria o pool SQLite pertencente ao processo e aplica as proteções exigidas pelo Faultmap.
// O chamador é responsável pelo pool retornado e deve fechá-lo no encerramento controlado.
func Open(ctx context.Context, databasePath string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}

	database.SetMaxOpenConns(maxOpenConnections)
	database.SetMaxIdleConns(maxIdleConnections)
	database.SetConnMaxLifetime(0)
	database.SetConnMaxIdleTime(0)

	if err := database.PingContext(ctx); err != nil {
		return closeAfterFailure(database, "ping SQLite database", err)
	}
	if _, err := database.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return closeAfterFailure(database, "enable SQLite foreign keys", err)
	}
	if _, err := database.ExecContext(ctx, busyTimeoutStatement); err != nil {
		return closeAfterFailure(database, "configure SQLite busy timeout", err)
	}
	if _, err := database.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		return closeAfterFailure(database, "enable SQLite WAL mode", err)
	}

	return database, nil
}

// closeAfterFailure preserva o erro da operação e também informa uma falha ao fechar o pool.
func closeAfterFailure(database *sql.DB, operation string, cause error) (*sql.DB, error) {
	if closeErr := database.Close(); closeErr != nil {
		return nil, fmt.Errorf("%s: %w; close SQLite database: %v", operation, cause, closeErr)
	}
	return nil, fmt.Errorf("%s: %w", operation, cause)
}
