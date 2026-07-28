package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

// TestOpenConfiguresSQLiteForFaultmap verifica o pool compartilhado e as proteções do SQLite.
func TestOpenConfiguresSQLiteForFaultmap(t *testing.T) {
	t.Parallel()

	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "faultmap.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close database: %v", closeErr)
		}
	})

	var foreignKeys int
	if err := database.QueryRowContext(context.Background(), "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var busyTimeout int
	if err := database.QueryRowContext(context.Background(), "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout != 5_000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}
}
