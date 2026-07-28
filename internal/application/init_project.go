package application

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/faultmap/faultmap/internal/platform/config"
)

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
	defaultConfig, err := config.DefaultYAML()
	if err != nil {
		return fmt.Errorf("initialize project: render default configuration: %w", err)
	}
	if err := ensureContext(ctx); err != nil {
		return err
	}
	if err := os.WriteFile(configPath, defaultConfig, 0o600); err != nil {
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
