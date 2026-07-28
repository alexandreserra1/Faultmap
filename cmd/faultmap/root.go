package main

import (
	"fmt"
	"path/filepath"

	"github.com/faultmap/faultmap/internal/application"
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
						runErr = fmt.Errorf("close SQLite database: %w", closeErr)
					}
				}
			}()

			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("migrate SQLite database: %w", err)
			}

			_, err = fmt.Fprintln(command.OutOrStdout(), "Faultmap initialized.")
			return err
		},
	}
	command.Flags().StringVarP(&projectDir, "directory", "d", ".", "directory for the Faultmap workspace")
	return command
}
