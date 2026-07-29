package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/platform/config"
	storage "github.com/faultmap/faultmap/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

// newRootCommand monta a CLI principal do Faultmap e seus subcomandos.
func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "faultmap",
		Short: "Investigação determinística de incidentes",
	}
	root.AddCommand(newInitCommand())
	root.AddCommand(newIngestCommand())
	return root
}

// newInitCommand cria um workspace local do Faultmap e aplica seu schema inicial.
func newInitCommand() *cobra.Command {
	var projectDir string

	command := &cobra.Command{
		Use:   "init",
		Short: "Cria a configuração e a base local do Faultmap",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			if err := application.InitializeProject(command.Context(), projectDir); err != nil {
				return err
			}

			databasePath := filepath.Join(projectDir, "faultmap.db")
			database, err := storage.Open(command.Context(), databasePath)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil {
					if runErr == nil {
						runErr = fmt.Errorf("fechar banco SQLite: %w", closeErr)
					}
				}
			}()

			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations SQLite: %w", err)
			}

			_, err = fmt.Fprintln(command.OutOrStdout(), "Faultmap inicializado.")
			return err
		},
	}
	command.Flags().StringVarP(&projectDir, "directory", "d", ".", "diretório do workspace do Faultmap")
	return command
}

// newIngestCommand agrupa as entradas que transformam telemetria externa em sinais internos.
func newIngestCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "ingest",
		Short: "Importa telemetria para o Faultmap",
	}
	command.AddCommand(newIngestFileCommand())
	return command
}

// newIngestFileCommand importa um arquivo OTLP JSON para o banco definido pela configuração local.
func newIngestFileCommand() *cobra.Command {
	var configPath string
	var inputPath string

	command := &cobra.Command{
		Use:   "file",
		Short: "Importa um arquivo OTLP JSON",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			if strings.TrimSpace(inputPath) == "" {
				return fmt.Errorf("ingerir arquivo: --input é obrigatório")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			databasePath := resolveStoragePath(configPath, loadedConfig.Storage.Path)
			database, err := storage.Open(command.Context(), databasePath)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco SQLite: %w", closeErr)
				}
			}()

			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations SQLite: %w", err)
			}

			result, err := application.IngestTelemetryFile(
				command.Context(),
				inputPath,
				storage.NewSignalRepository(database),
			)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "Ingeridos %d sinais; %d novos.\n", result.Normalized, result.Persisted)
			return err
		},
	}
	command.Flags().StringVar(&inputPath, "input", "", "caminho do arquivo OTLP JSON")
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	return command
}

// resolveStoragePath interpreta caminhos relativos do banco a partir do diretório do YAML.
func resolveStoragePath(configPath string, storagePath string) string {
	if filepath.IsAbs(storagePath) {
		return filepath.Clean(storagePath)
	}
	return filepath.Join(filepath.Dir(configPath), storagePath)
}
