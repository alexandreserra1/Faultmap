package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// TestEphemeralProjectDirCriaForaDoProjeto é o que a sessão efêmera promete:
// experimentar o Faultmap sem sujar o diretório de trabalho de quem testa.
func TestEphemeralProjectDirCriaForaDoProjeto(t *testing.T) {
	t.Parallel()

	dir, err := EphemeralProjectDir()
	if err != nil {
		t.Fatalf("EphemeralProjectDir() erro = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	trabalho, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() erro = %v", err)
	}
	if strings.HasPrefix(dir, trabalho) {
		t.Fatalf("workspace efêmero %q ficou dentro do diretório de trabalho %q", dir, trabalho)
	}
	if !strings.HasPrefix(dir, os.TempDir()) {
		t.Fatalf("workspace efêmero %q não ficou no diretório temporário do sistema %q", dir, os.TempDir())
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("workspace efêmero não é um diretório utilizável: %v", err)
	}
}

// TestEphemeralProjectDirNaoColide impede que duas sessões simultâneas —
// dois terminais, dois cenários de teste — gravem uma por cima da outra.
func TestEphemeralProjectDirNaoColide(t *testing.T) {
	t.Parallel()

	vistos := make(map[string]struct{}, 20)
	for range 20 {
		dir, err := EphemeralProjectDir()
		if err != nil {
			t.Fatalf("EphemeralProjectDir() erro = %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		if _, repetido := vistos[dir]; repetido {
			t.Fatalf("dois workspaces efêmeros no mesmo caminho: %q", dir)
		}
		vistos[dir] = struct{}{}
	}
}

// TestEphemeralProjectDirServeAoInitCompleto fecha o ciclo: o diretório
// devolvido precisa aceitar a inicialização normal, senão a flag entregaria um
// caminho que não funciona.
func TestEphemeralProjectDirServeAoInitCompleto(t *testing.T) {
	t.Parallel()

	dir, err := EphemeralProjectDir()
	if err != nil {
		t.Fatalf("EphemeralProjectDir() erro = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := InitializeProject(context.Background(), dir); err != nil {
		t.Fatalf("InitializeProject() no workspace efêmero erro = %v", err)
	}
	for _, artefato := range []string{"faultmap.yaml", "faultmap-out"} {
		if _, err := os.Stat(filepath.Join(dir, artefato)); err != nil {
			t.Fatalf("artefato %q ausente no workspace efêmero: %v", artefato, err)
		}
	}
}
