package application

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const defaultConfig = `server:
  otlp_http_address: "0.0.0.0:4318"
  health_address: "0.0.0.0:8081"

storage:
  driver: "sqlite"
  path: "./faultmap.db"
  retention: "7d"

investigation:
  default_incident_window: "30m"
  default_baseline_window: "60m"
  top_suspects: 3
`

// InitializeProject cria os artefatos locais exigidos pelo comando faultmap init.
// A função verifica o contexto antes de cada efeito colateral e não sobrescreve artefatos existentes.
func InitializeProject(ctx context.Context, projectDir string) error {
	configPath := filepath.Join(projectDir, "faultmap.yaml")
	databasePath := filepath.Join(projectDir, "faultmap.db")
	outputPath := filepath.Join(projectDir, "faultmap-out")

	for _, path := range []string{configPath, databasePath, outputPath} {
		if err := ensureContext(ctx); err != nil {
			return err
		}
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("initialize project: %q already exists", path)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("initialize project: inspect %q: %w", path, err)
		}
	}

	if err := ensureContext(ctx); err != nil {
		return err
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return fmt.Errorf("initialize project: create project directory: %w", err)
	}
	if err := ensureContext(ctx); err != nil {
		return err
	}
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0o600); err != nil {
		return fmt.Errorf("initialize project: write configuration: %w", err)
	}
	if err := ensureContext(ctx); err != nil {
		return err
	}
	if err := os.WriteFile(databasePath, nil, 0o600); err != nil {
		return fmt.Errorf("initialize project: create database: %w", err)
	}
	if err := ensureContext(ctx); err != nil {
		return err
	}
	if err := os.Mkdir(outputPath, 0o755); err != nil {
		return fmt.Errorf("initialize project: create output directory: %w", err)
	}

	return nil
}

func ensureContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("initialize project: context canceled: %w", err)
	}
	return nil
}
