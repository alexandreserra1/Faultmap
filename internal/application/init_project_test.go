package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestInitializeProjectCreatesLocalWorkspace verifica os artefatos criados pela inicialização.
func TestInitializeProjectCreatesLocalWorkspace(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	if err := InitializeProject(context.Background(), projectDir); err != nil {
		t.Fatalf("InitializeProject() error = %v", err)
	}

	assertFileExists(t, filepath.Join(projectDir, "faultmap.yaml"))
	assertFileExists(t, filepath.Join(projectDir, "faultmap.db"))
	assertDirectoryExists(t, filepath.Join(projectDir, "faultmap-out"))
}

// TestInitializeProjectDoesNotOverwriteExistingConfiguration protege a configuração existente.
func TestInitializeProjectDoesNotOverwriteExistingConfiguration(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	configPath := filepath.Join(projectDir, "faultmap.yaml")
	const existingConfig = "server:\n  otlp_http_address: existing\n"

	if err := os.WriteFile(configPath, []byte(existingConfig), 0o600); err != nil {
		t.Fatalf("write existing configuration: %v", err)
	}

	err := InitializeProject(context.Background(), projectDir)
	if err == nil {
		t.Fatal("InitializeProject() error = nil, want an error for existing configuration")
	}

	content, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read existing configuration: %v", readErr)
	}
	if string(content) != existingConfig {
		t.Fatalf("existing configuration changed = %q, want %q", content, existingConfig)
	}
}

// TestInitializeProjectHonorsCancelledContext evita efeitos colaterais após o cancelamento.
func TestInitializeProjectHonorsCancelledContext(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := InitializeProject(ctx, projectDir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InitializeProject() error = %v, want context.Canceled", err)
	}

	entries, readErr := os.ReadDir(projectDir)
	if readErr != nil {
		t.Fatalf("read project directory: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("workspace entries = %d, want 0 after cancellation", len(entries))
	}
}

// assertFileExists falha no teste quando o caminho não corresponde a um arquivo.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("%q is a directory, want a file", path)
	}
}

// assertDirectoryExists falha no teste quando o caminho não corresponde a um diretório.
func assertDirectoryExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", path)
	}
}
