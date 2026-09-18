package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	changedomain "github.com/faultmap/faultmap/internal/changes/domain"
	incidentdomain "github.com/faultmap/faultmap/internal/incidents/domain"
	githubintegration "github.com/faultmap/faultmap/internal/integrations/github"
	postgresintegration "github.com/faultmap/faultmap/internal/integrations/postgres"
	mcpserver "github.com/faultmap/faultmap/internal/mcp"
	"github.com/faultmap/faultmap/internal/platform/config"
	"github.com/faultmap/faultmap/internal/ranking"
	"github.com/faultmap/faultmap/internal/reporting/artifacts"
	jsonreport "github.com/faultmap/faultmap/internal/reporting/json"
	"github.com/faultmap/faultmap/internal/reporting/markdown"
	"github.com/faultmap/faultmap/internal/reporting/mermaid"
	terminal "github.com/faultmap/faultmap/internal/reporting/terminal"
	"github.com/faultmap/faultmap/internal/reporting/timeline"
	storage "github.com/faultmap/faultmap/internal/storage/bootstrap"
	"github.com/faultmap/faultmap/internal/telemetry/privacy"
	"github.com/spf13/cobra"

	// O driver é registrado por efeito colateral; o coletor de catálogo recebe
	// apenas um *sql.DB e não conhece qual driver o abriu.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// newRootCommand monta a CLI principal do Faultmap e seus subcomandos.
func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "faultmap",
		Short: "Investigação determinística de incidentes",
	}
	root.AddCommand(newInitCommand())
	root.AddCommand(newServeCommand())
	root.AddCommand(newIngestCommand())
	root.AddCommand(newTelemetryCommand())
	root.AddCommand(newDiagnoseCommand())
	root.AddCommand(newIncidentCommand())
	root.AddCommand(newBlameCommand())
	root.AddCommand(newExplainCommand())
	root.AddCommand(newExportCommand())
	root.AddCommand(newRetentionCommand())
	root.AddCommand(newMCPCommand())
	return root
}

// newRetentionCommand agrupa a manutenção explícita do banco local.
func newRetentionCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "retention",
		Short: "Aplica a política de retenção do banco local",
	}
	command.AddCommand(newRetentionApplyCommand())
	return command
}

// newRetentionApplyCommand executa a limpeza sob controle explícito do operador.
// A retenção nunca roda durante a ingestão OTLP: o receiver precisa continuar
// previsível, e apagar dados é uma decisão que merece um comando próprio.
func newRetentionApplyCommand() *cobra.Command {
	var configPath string
	var batchSize int

	command := &cobra.Command{
		Use:   "apply",
		Short: "Remove telemetria mais antiga que storage.retention",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			retention, err := loadedConfig.Storage.RetentionDuration()
			if err != nil {
				return fmt.Errorf("aplicar retenção: %w", err)
			}

			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			retencao := storage.NewRetentionRepository(database)
			result, err := application.ApplyRetention(
				command.Context(),
				application.RetentionRequest{
					Retention: retention,
					Now:       time.Now().UTC(),
					BatchSize: batchSize,
				},
				// O mesmo repositório cuida das duas frentes da retenção, e é
				// sempre ligado: quem nunca coletou catálogo simplesmente não
				// tem o que liberar, e o comando relata zero.
				retencao,
				retencao,
			)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(
				command.OutOrStdout(),
				"Retenção aplicada: %d sinais removidos e %d catálogos de schema liberados, anteriores a %s.\n",
				result.SignalsRemoved,
				result.SchemaCatalogsPruned,
				result.Cutoff.Format(time.RFC3339),
			); err != nil {
				return err
			}
			if result.Truncated {
				_, err = fmt.Fprintln(
					command.OutOrStdout(),
					"Ainda existe telemetria expirada: execute o comando novamente.",
				)
			}
			return err
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().IntVar(&batchSize, "batch-size", application.DefaultRetentionBatchSize, "quantidade máxima de sinais por transação")
	return command
}

// newIncidentCommand agrupa consultas de snapshots de diagnósticos já persistidos.
func newIncidentCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "incident",
		Short: "Consulta diagnósticos persistidos",
	}
	command.AddCommand(newIncidentListCommand())
	command.AddCommand(newIncidentShowCommand())
	return command
}

// newIncidentListCommand apresenta resumos recentes sem carregar findings ou ranking.
func newIncidentListCommand() *cobra.Command {
	var configPath string
	var limit int

	command := &cobra.Command{
		Use:   "list",
		Short: "Lista diagnósticos persistidos",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			// A validação antecede I/O para não abrir o pool quando a entrada já é inválida.
			if limit <= 0 || limit > 1_000 {
				return fmt.Errorf("listar incidentes: --limit deve estar entre 1 e 1000")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			incidents, err := application.ListIncidents(
				command.Context(),
				limit,
				storage.NewDiagnosisRepository(database),
			)
			if err != nil {
				return err
			}
			return terminal.RenderIncidentList(command.OutOrStdout(), incidents)
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().IntVar(&limit, "limit", 20, "quantidade máxima de incidentes")
	return command
}

// newIncidentShowCommand recupera um diagnóstico completo sem executar novamente os detectores.
func newIncidentShowCommand() *cobra.Command {
	var configPath string
	var incidentID string

	command := &cobra.Command{
		Use:   "show",
		Short: "Exibe um diagnóstico persistido",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			// O identificador é validado antes de qualquer leitura de arquivo ou banco.
			if strings.TrimSpace(incidentID) == "" {
				return fmt.Errorf("consultar incidente: --id é obrigatório")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			diagnosis, err := application.GetIncident(
				command.Context(),
				incidentID,
				storage.NewDiagnosisRepository(database),
			)
			if err != nil {
				return err
			}
			return terminal.RenderPersistedDiagnosis(command.OutOrStdout(), diagnosis)
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().StringVar(&incidentID, "id", "", "identificador do incidente")
	return command
}

// newExplainCommand agrupa as explicações detalhadas de um diagnóstico gravado.
func newExplainCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "explain",
		Short: "Explica em detalhe um resultado do diagnóstico",
	}
	command.AddCommand(newExplainSuspectCommand())
	return command
}

// newExplainSuspectCommand detalha as contribuições de um suspeito a partir do
// snapshot gravado. Assim como incident show, ele não reexecuta detectores nem
// ranking: a explicação precisa descrever a investigação publicada, e não uma
// nova leitura da telemetria que pode ter mudado desde então.
func newExplainSuspectCommand() *cobra.Command {
	var configPath string
	var incidentID string

	command := &cobra.Command{
		Use:   "suspect <serviço>",
		Short: "Explica por que um suspeito foi apontado",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) (runErr error) {
			if strings.TrimSpace(incidentID) == "" {
				return fmt.Errorf("explicar suspeito: --incident é obrigatório")
			}
			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("explicar suspeito: informe o nome do serviço")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			diagnosis, err := application.GetIncident(
				command.Context(), incidentID, storage.NewDiagnosisRepository(database),
			)
			if err != nil {
				return err
			}
			explanation, err := application.ExplainSuspect(diagnosis, args[0])
			if err != nil {
				return err
			}
			return terminal.RenderSuspectExplanation(command.OutOrStdout(), explanation)
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().StringVar(&incidentID, "incident", "", "identificador do incidente persistido")
	return command
}

// newExportCommand agrupa formatos estruturados derivados das evidências persistidas.
func newExportCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "export",
		Short: "Exporta evidências do Faultmap",
	}
	command.AddCommand(newExportReportCommand())
	command.AddCommand(newExportGraphCommand())
	command.AddCommand(newExportTimelineCommand())
	command.AddCommand(newExportArtifactsCommand())
	return command
}

// newExportArtifactsCommand grava de uma vez os cinco artefatos previstos para
// o diretório de saída, em vez de exigir um redirecionamento manual por formato.
// O diretório precisa existir: criá-lo aqui esconderia um caminho digitado
// errado e espalharia arquivos fora do workspace.
func newExportArtifactsCommand() *cobra.Command {
	var configPath string
	var incidentID string
	var outputDir string

	command := &cobra.Command{
		Use:   "artifacts",
		Short: "Grava os artefatos do incidente em faultmap-out/",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			if strings.TrimSpace(incidentID) == "" {
				return fmt.Errorf("exportar artefatos: --incident é obrigatório")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			diagnosis, err := application.GetIncident(
				command.Context(), incidentID, storage.NewDiagnosisRepository(database),
			)
			if err != nil {
				return err
			}

			destination := strings.TrimSpace(outputDir)
			if destination == "" {
				destination = filepath.Join(filepath.Dir(configPath), "faultmap-out")
			}
			// O instante fica na borda: os renderizadores o recebem pronto para
			// que a mesma investigação produza artefatos comparáveis em testes.
			if err := artifacts.Write(destination, diagnosis, nil, time.Now().UTC()); err != nil {
				return err
			}
			_, err = fmt.Fprintf(
				command.OutOrStdout(),
				"Artefatos de %s gravados em %s.\n", diagnosis.Incident.ID, destination,
			)
			return err
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().StringVar(&incidentID, "incident", "", "identificador do incidente persistido")
	command.Flags().StringVar(&outputDir, "output", "", "diretório de saída (padrão: faultmap-out/ ao lado da configuração)")
	return command
}

// newExportTimelineCommand reconstrói a cronologia do incidente a partir do
// snapshot já gravado. Ele não relê telemetria nem recalcula o diagnóstico: o
// artefato precisa refletir exatamente a investigação que foi publicada.
func newExportTimelineCommand() *cobra.Command {
	var configPath string
	var incidentID string
	var format string

	command := &cobra.Command{
		Use:   "timeline",
		Short: "Exporta a cronologia de um diagnóstico persistido",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			if strings.TrimSpace(incidentID) == "" {
				return fmt.Errorf("exportar timeline: --incident é obrigatório")
			}
			if strings.ToLower(strings.TrimSpace(format)) != "json" {
				return fmt.Errorf("exportar timeline: --format deve ser json")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			diagnosis, err := application.GetIncident(
				command.Context(),
				incidentID,
				storage.NewDiagnosisRepository(database),
			)
			if err != nil {
				return err
			}
			return timeline.Render(command.OutOrStdout(), diagnosis)
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().StringVar(&format, "format", "json", "formato da cronologia")
	command.Flags().StringVar(&incidentID, "incident", "", "identificador do incidente persistido")
	return command
}

// newExportReportCommand serializa um snapshot persistido sem recalcular sua análise.
func newExportReportCommand() *cobra.Command {
	var configPath string
	var format string
	var incidentID string

	command := &cobra.Command{
		Use:   "report",
		Short: "Exporta um diagnóstico persistido",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			if strings.TrimSpace(incidentID) == "" {
				return fmt.Errorf("exportar relatório: --incident é obrigatório")
			}
			format = strings.ToLower(strings.TrimSpace(format))
			if format != "json" && format != "markdown" {
				return fmt.Errorf("exportar relatório: --format deve ser json ou markdown")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			diagnosis, err := application.GetIncident(
				command.Context(),
				incidentID,
				storage.NewDiagnosisRepository(database),
			)
			if err != nil {
				return err
			}
			if format == "json" {
				return jsonreport.Render(command.OutOrStdout(), diagnosis)
			}
			return markdown.Render(command.OutOrStdout(), diagnosis)
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().StringVar(&format, "format", "markdown", "formato do relatório: json ou markdown")
	command.Flags().StringVar(&incidentID, "incident", "", "identificador do incidente persistido")
	return command
}

// newExportGraphCommand gera um diagrama Mermaid do trace solicitado sem persistir estado derivado.
func newExportGraphCommand() *cobra.Command {
	var configPath string
	var format string
	var traceID string
	var limit int

	command := &cobra.Command{
		Use:   "graph",
		Short: "Exporta o grafo de evidências de um trace",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			if strings.TrimSpace(traceID) == "" {
				return fmt.Errorf("exportar grafo: --trace é obrigatório")
			}
			if limit <= 0 {
				return fmt.Errorf("exportar grafo: --limit deve ser maior que zero")
			}
			if format != "mermaid" {
				return fmt.Errorf("exportar grafo: --format deve ser mermaid")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			investigation, err := application.BlameTrace(
				command.Context(),
				traceID,
				limit,
				storage.NewSignalRepository(database),
			)
			if err != nil {
				return err
			}
			return mermaid.RenderTraceGraph(command.OutOrStdout(), investigation.Graph)
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().StringVar(&format, "format", "mermaid", "formato de saída")
	command.Flags().StringVar(&traceID, "trace", "", "identificador do trace")
	command.Flags().IntVar(&limit, "limit", 1_000, "quantidade máxima de sinais do trace")
	return command
}

// newBlameCommand agrupa investigações focadas em uma evidência identificável.
func newBlameCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "blame",
		Short: "Investiga evidências relacionadas a um sinal",
	}
	command.AddCommand(newBlameTraceCommand())
	return command
}

// newBlameTraceCommand reconstrói o fluxo seguro e limitado de um trace persistido.
func newBlameTraceCommand() *cobra.Command {
	var configPath string
	var traceID string
	var limit int

	command := &cobra.Command{
		Use:   "trace",
		Short: "Explica os spans e relações de um trace",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			if strings.TrimSpace(traceID) == "" {
				return fmt.Errorf("investigar trace: --trace é obrigatório")
			}
			if limit <= 0 {
				return fmt.Errorf("investigar trace: --limit deve ser maior que zero")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			investigation, err := application.BlameTrace(
				command.Context(),
				traceID,
				limit,
				storage.NewSignalRepository(database),
			)
			if err != nil {
				return err
			}
			return terminal.RenderTraceInvestigation(
				command.OutOrStdout(),
				investigation.TraceID,
				investigation.Signals,
				investigation.Graph,
			)
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().StringVar(&traceID, "trace", "", "identificador do trace")
	command.Flags().IntVar(&limit, "limit", 1_000, "quantidade máxima de sinais do trace")
	return command
}

// newInitCommand cria um workspace local do Faultmap e aplica seu schema inicial.
func newInitCommand() *cobra.Command {
	var projectDir string
	var ephemeral bool

	command := &cobra.Command{
		Use:   "init",
		Short: "Cria a configuração e a base local do Faultmap",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			// As duas flags respondem à mesma pergunta — onde fica o workspace —
			// e atender uma calando a outra faria a pessoa procurar arquivos no
			// lugar errado.
			if ephemeral && command.Flags().Changed("directory") {
				return fmt.Errorf("inicializar: use --ephemeral ou --directory, não os dois")
			}
			if ephemeral {
				temporary, err := application.EphemeralProjectDir()
				if err != nil {
					return err
				}
				projectDir = temporary
				// Se a inicialização falhar — Ctrl+C, disco cheio — o diretório
				// recém-criado some junto. Deixá-lo para trás sem nunca imprimir
				// o caminho daria à pessoa lixo que ela não teria como achar.
				defer func() {
					if runErr != nil {
						_ = os.RemoveAll(temporary)
					}
				}()
			}
			if err := application.InitializeProject(command.Context(), projectDir); err != nil {
				return err
			}

			databasePath := filepath.Join(projectDir, "faultmap.db")
			// O init acabou de gravar a configuração padrão; o driver sai dela.
			// Ler o padrão em vez de fixar "sqlite" aqui mantém os dois em
			// sincronia se o padrão mudar um dia.
			database, err := storage.Open(command.Context(), config.Default().Storage.Driver, databasePath)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil {
					if runErr == nil {
						runErr = fmt.Errorf("fechar banco: %w", closeErr)
					}
				}
			}()

			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			if ephemeral {
				// O caminho vem antes de qualquer outra coisa: sem ele a sessão
				// é inalcançável. E a limpeza é declarada como manual porque o
				// `init` termina antes de o workspace ser usado — não existe
				// momento em que ele pudesse apagar, e prometer isso seria
				// mentira.
				_, err = fmt.Fprintf(
					command.OutOrStdout(),
					"Faultmap inicializado em modo efêmero.\n\n"+
						"  Workspace: %s\n\n"+
						"Use --config %s nos comandos seguintes.\n"+
						"Nada é apagado automaticamente: remova o diretório quando terminar.\n",
					projectDir, filepath.Join(projectDir, "faultmap.yaml"),
				)
				return err
			}
			_, err = fmt.Fprintln(command.OutOrStdout(), "Faultmap inicializado.")
			return err
		},
	}
	command.Flags().StringVarP(&projectDir, "directory", "d", ".", "diretório do workspace do Faultmap")
	command.Flags().BoolVar(&ephemeral, "ephemeral", false,
		"cria o workspace no diretório temporário do sistema, sem sujar o projeto")
	return command
}

// newMCPCommand expõe os diagnósticos já registrados a clientes MCP.
//
// A sessão fala JSON-RPC por stdin e stdout, então nada além do protocolo pode
// ser escrito na saída padrão: a auditoria vai para stderr, e as mensagens de
// erro do Cobra também. Um "Faultmap iniciado." aqui corromperia o aperto de
// mão de todo cliente.
func newMCPCommand() *cobra.Command {
	var configPath string

	command := &cobra.Command{
		Use:   "mcp",
		Short: "Expõe diagnósticos registrados por MCP (somente leitura)",
		// O servidor não investiga nem ingere: ele lê o que o Faultmap já
		// analisou. A decisão está documentada no pacote internal/mcp.
		SilenceUsage: true,
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			return mcpserver.Serve(command.Context(), mcpserver.Options{
				Input:   command.InOrStdin(),
				Output:  command.OutOrStdout(),
				Audit:   command.ErrOrStderr(),
				History: storage.NewDiagnosisRepository(database),
			})
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	return command
}

// newIngestCommand agrupa as entradas que transformam telemetria externa em sinais internos.
func newIngestCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "ingest",
		Short: "Importa telemetria para o Faultmap",
	}
	command.AddCommand(newIngestFileCommand())
	command.AddCommand(newIngestGitHubCommand())
	command.AddCommand(newIngestSchemaCommand())
	return command
}

// newIngestSchemaCommand coleta o catálogo de uma base PostgreSQL e registra o
// que mudou desde a coleta anterior.
//
// A DSN vem por variável de ambiente, e não da configuração: o `init` promete
// criar um faultmap.yaml sem tokens nem credenciais, e uma senha de banco no
// arquivo do projeto contradiz isso. O nome da base é gravado; a forma de
// conectar nela, não.
func newIngestSchemaCommand() *cobra.Command {
	var configPath string
	var databaseName string

	command := &cobra.Command{
		Use:   "schema",
		Short: "Coleta o catálogo de uma base PostgreSQL",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			databaseName = strings.TrimSpace(databaseName)
			if databaseName == "" {
				return fmt.Errorf("ingerir catálogo: --database é obrigatório")
			}
			dsn := strings.TrimSpace(os.Getenv("FAULTMAP_PG_DSN"))
			if dsn == "" {
				return fmt.Errorf("ingerir catálogo: FAULTMAP_PG_DSN é obrigatório")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			source, err := sql.Open("pgx", dsn)
			if err != nil {
				return fmt.Errorf("ingerir catálogo: abrir conexão PostgreSQL: %w", err)
			}
			defer func() {
				if closeErr := source.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar conexão PostgreSQL: %w", closeErr)
				}
			}()

			client, err := postgresintegration.NewClient(source, databaseName, nil)
			if err != nil {
				return err
			}
			result, err := application.IngestSchema(
				command.Context(), client, storage.NewSchemaRepository(database),
			)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(
				command.OutOrStdout(),
				"Coletados %d objetos do catálogo de %s; %d mudanças desde a coleta anterior.\n",
				result.ObjectsCollected, databaseName, result.ChangesPersisted,
			)
			return err
		},
	}
	command.Flags().StringVar(&databaseName, "database", "", "nome da base observada")
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	return command
}

// newIngestGitHubCommand importa uma página limitada de mudanças sem manter estado em memória.
func newIngestGitHubCommand() *cobra.Command {
	var commits bool
	var configPath string
	var deployments bool
	var environment string
	var limit int
	var repository string
	var serviceName string
	var since string
	var until string

	command := &cobra.Command{
		Use:   "github",
		Short: "Importa commits e deployments do GitHub",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			if !commits && !deployments {
				return fmt.Errorf("ingerir GitHub: selecione --commits e/ou --deployments")
			}
			if limit <= 0 || limit > 100 {
				return fmt.Errorf("ingerir GitHub: --limit deve estar entre 1 e 100")
			}
			windowDuration, err := time.ParseDuration(since)
			if err != nil || windowDuration <= 0 {
				return fmt.Errorf("ingerir GitHub: --since deve ser uma duração positiva")
			}
			importUntil, err := githubImportEnd(until)
			if err != nil {
				return err
			}
			token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
			if token == "" {
				return fmt.Errorf("ingerir GitHub: GITHUB_TOKEN é obrigatório")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			repository = strings.TrimSpace(repository)
			if repository == "" {
				repository = strings.TrimSpace(loadedConfig.GitHub.Repository)
			}
			parts := strings.Split(repository, "/")
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
				return fmt.Errorf("ingerir GitHub: --repo deve usar owner/repository")
			}
			environment = strings.TrimSpace(environment)
			if environment == "" {
				environment = strings.TrimSpace(loadedConfig.GitHub.Environment)
			}
			serviceName = strings.TrimSpace(serviceName)
			if deployments && serviceName == "" {
				serviceName = parts[1]
			}

			client, err := githubintegration.NewClient(
				&http.Client{Timeout: 15 * time.Second},
				loadedConfig.GitHub.APIURL,
				token,
			)
			if err != nil {
				return err
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			result, err := application.IngestChanges(
				command.Context(),
				changedomain.ImportRequest{
					Repository: repository, Environment: environment, ServiceName: serviceName,
					Since: importUntil.Add(-windowDuration), Until: importUntil, Limit: limit,
					IncludeCommits: commits, IncludeDeployments: deployments,
				},
				client,
				storage.NewChangeRepository(database),
			)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(
				command.OutOrStdout(),
				"%d commits coletados; %d novos. %d deployments coletados; %d novos.\n",
				result.CommitsFetched, result.CommitsPersisted,
				result.DeploymentsFetched, result.DeploymentsPersisted,
			)
			return err
		},
	}
	command.Flags().BoolVar(&commits, "commits", false, "importar commits")
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().BoolVar(&deployments, "deployments", false, "importar deployments")
	command.Flags().StringVar(&environment, "environment", "", "ambiente dos deployments")
	command.Flags().IntVar(&limit, "limit", 100, "quantidade máxima por recurso")
	command.Flags().StringVar(&repository, "repo", "", "repositório owner/repository")
	command.Flags().StringVar(&serviceName, "service", "", "serviço associado aos deployments")
	command.Flags().StringVar(&since, "since", "168h", "janela retroativa da importação")
	command.Flags().StringVar(&until, "until", "", "fim da importação em RFC 3339")
	return command
}

func githubImportEnd(until string) (time.Time, error) {
	if strings.TrimSpace(until) == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, until)
	if err != nil {
		return time.Time{}, fmt.Errorf("ingerir GitHub: --until deve usar RFC 3339: %w", err)
	}
	return parsed.UTC(), nil
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
			database, err := storage.Open(command.Context(), loadedConfig.Storage.Driver, databasePath)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()

			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			result, err := application.IngestTelemetryFile(
				command.Context(),
				inputPath,
				privacyPolicyFrom(loadedConfig),
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

// privacyPolicyFrom traduz a configuração validada para a política aplicada
// antes da persistência, mantendo um único ponto de tradução para todos os
// caminhos de ingestão.
func privacyPolicyFrom(configuration config.Config) privacy.Policy {
	return privacy.NewPolicy(
		configuration.Privacy.BlockedAttributes,
		configuration.Privacy.MaxAttributeLength,
	)
}

// resolveStoragePath interpreta caminhos relativos do banco a partir do diretório do YAML.
func resolveStoragePath(configPath string, storagePath string) string {
	if filepath.IsAbs(storagePath) {
		return filepath.Clean(storagePath)
	}
	return filepath.Join(filepath.Dir(configPath), storagePath)
}

// newTelemetryCommand agrupa consultas de sinais já persistidos no Faultmap.
func newTelemetryCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "telemetry",
		Short: "Consulta telemetria persistida",
	}
	command.AddCommand(newTelemetryListCommand())
	return command
}

// newTelemetryListCommand apresenta sinais de um serviço em uma janela temporal limitada.
func newTelemetryListCommand() *cobra.Command {
	var configPath string
	var serviceName string
	var since string
	var limit int

	command := &cobra.Command{
		Use:   "list",
		Short: "Lista sinais de telemetria de um serviço",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			if strings.TrimSpace(serviceName) == "" {
				return fmt.Errorf("listar telemetria: --service é obrigatório")
			}
			windowDuration, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("listar telemetria: --since deve ser uma duração positiva: %w", err)
			}
			if windowDuration <= 0 {
				return fmt.Errorf("listar telemetria: --since deve ser uma duração positiva")
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			end := time.Now().UTC()
			signals, err := application.ListSignals(
				command.Context(),
				serviceName,
				end.Add(-windowDuration),
				end,
				limit,
				storage.NewSignalRepository(database),
			)
			if err != nil {
				return err
			}
			return terminal.RenderSignals(command.OutOrStdout(), serviceName, signals)
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().StringVar(&serviceName, "service", "", "nome do serviço")
	command.Flags().StringVar(&since, "since", "24h", "janela retroativa de consulta")
	command.Flags().IntVar(&limit, "limit", 20, "quantidade máxima de sinais")
	return command
}

// newDiagnoseCommand agrupa os comandos que comparam períodos e explicam hipóteses de incidente.
func newDiagnoseCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "diagnose",
		Short: "Diagnostica um incidente a partir de telemetria persistida",
	}
	command.AddCommand(newDiagnoseIncidentCommand())
	return command
}

// newDiagnoseIncidentCommand compara baseline e incidente e apresenta findings determinísticos.
func newDiagnoseIncidentCommand() *cobra.Command {
	var allServices bool
	var baseline string
	var configPath string
	var environment string
	var incidentDuration string
	var limit int
	var maxServices int
	var noExpand bool
	var depth int
	var serviceName string
	var until string

	command := &cobra.Command{
		Use:   "incident",
		Short: "Diagnostica um incidente e compara os serviços envolvidos",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			if strings.TrimSpace(serviceName) == "" && !allServices {
				return fmt.Errorf("diagnosticar incidente: informe --service ou --all")
			}
			if maxServices <= 0 {
				return fmt.Errorf("diagnosticar incidente: --max-services deve ser maior que zero")
			}
			if depth < 1 || depth > application.MaxScopeDepth {
				return fmt.Errorf("diagnosticar incidente: --depth deve estar entre 1 e %d", application.MaxScopeDepth)
			}
			incidentWindowDuration, err := time.ParseDuration(incidentDuration)
			if err != nil {
				return fmt.Errorf("diagnosticar incidente: --since deve ser uma duração positiva: %w", err)
			}
			if incidentWindowDuration <= 0 {
				return fmt.Errorf("diagnosticar incidente: --since deve ser uma duração positiva")
			}
			baselineDuration, err := time.ParseDuration(baseline)
			if err != nil {
				return fmt.Errorf("diagnosticar incidente: --baseline deve ser uma duração positiva: %w", err)
			}
			if baselineDuration <= 0 {
				return fmt.Errorf("diagnosticar incidente: --baseline deve ser uma duração positiva")
			}

			incidentEnd, err := diagnosisEnd(until)
			if err != nil {
				return err
			}
			windows, err := incidentdomain.NewInvestigationWindowFromIncident(
				incidentEnd.Add(-incidentWindowDuration),
				incidentEnd,
				baselineDuration,
			)
			if err != nil {
				return fmt.Errorf("diagnosticar incidente: calcular janelas: %w", err)
			}

			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}
			database, err := storage.Open(
				command.Context(),
				loadedConfig.Storage.Driver,
				resolveStoragePath(configPath, loadedConfig.Storage.Path),
			)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := database.Close(); closeErr != nil && runErr == nil {
					runErr = fmt.Errorf("fechar banco: %w", closeErr)
				}
			}()
			if err := storage.Migrate(command.Context(), database); err != nil {
				return fmt.Errorf("aplicar migrations: %w", err)
			}

			diagnosisEnvironment := strings.TrimSpace(environment)
			if diagnosisEnvironment == "" && loadedConfig.GitHub.Enabled {
				diagnosisEnvironment = strings.TrimSpace(loadedConfig.GitHub.Environment)
			}
			// A investigação compara serviços: o escopo nasce dos traces que
			// atravessaram o serviço de entrada, e não de um palpite de quem
			// investiga. --no-expand preserva o modo focado de um serviço só.
			request := application.ScopedDiagnosisRequest{
				EntryService: serviceName,
				Services:     splitServiceList(serviceName),
				Environment:  diagnosisEnvironment,
				Windows:      windows,
				Limit:        limit,
				MaxServices:  maxServices,
				NoExpand:     noExpand,
				AllServices:  allServices,
				Depth:        depth,
				Ranking:      rankingConfig(loadedConfig),
			}
			var deploymentReader application.ScopedDeploymentReader
			if diagnosisEnvironment != "" {
				deploymentReader = storage.NewChangeRepository(database)
			}
			diagnosis, err := application.DiagnoseIncidentInScope(
				command.Context(), request,
				storage.NewScopeRepository(database),
				storage.NewSignalRepository(database),
				deploymentReader,
				// O leitor de catálogo é sempre ligado: quem nunca rodou
				// `ingest schema` simplesmente não tem mudanças para correlacionar,
				// e a consulta em lote não é feita quando a janela não traz spans
				// de banco nomeando uma base.
				storage.NewSchemaRepository(database),
			)
			if err != nil {
				return err
			}
			created, persistenceErr := application.PersistDiagnosis(
				command.Context(),
				diagnosis,
				storage.NewDiagnosisRepository(database),
			)
			skippedEmptyIncident := errors.Is(persistenceErr, application.ErrNoIncidentSignals)
			if persistenceErr != nil && !skippedEmptyIncident {
				return persistenceErr
			}
			if err := terminal.RenderDiagnosis(
				command.OutOrStdout(),
				diagnosis.ServiceName,
				diagnosis.BaselineSignalCount,
				diagnosis.IncidentSignalCount,
				diagnosis.Findings,
				diagnosis.Suspects,
			); err != nil {
				return err
			}
			if err := terminal.RenderScopeSummary(command.OutOrStdout(), diagnosis.Scope); err != nil {
				return err
			}
			if skippedEmptyIncident {
				_, err = fmt.Fprintln(command.OutOrStdout(), "\nDiagnóstico não salvo: janela do incidente sem sinais.")
			} else if created {
				_, err = fmt.Fprintf(command.OutOrStdout(), "\nDiagnóstico salvo: %s\n", diagnosis.ID)
			} else {
				_, err = fmt.Fprintf(command.OutOrStdout(), "\nDiagnóstico já existente: %s\n", diagnosis.ID)
			}
			return err
		},
	}
	command.Flags().BoolVar(&allServices, "all", false, "comparar todos os serviços com telemetria na janela")
	command.Flags().BoolVar(&noExpand, "no-expand", false, "investigar somente o serviço informado, sem expandir pelos traces")
	command.Flags().IntVar(&maxServices, "max-services", application.DefaultMaxScopeServices, "quantidade máxima de serviços comparados")
	command.Flags().IntVar(&depth, "depth", application.DefaultScopeDepth, "saltos de trace percorridos ao expandir o escopo")
	command.Flags().StringVar(&baseline, "baseline", "60m", "duração da janela baseline")
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	command.Flags().StringVar(&environment, "environment", "", "ambiente usado para correlacionar deployments")
	command.Flags().StringVar(&incidentDuration, "since", "30m", "duração da janela de incidente")
	command.Flags().IntVar(&limit, "limit", 1_000, "quantidade máxima de sinais por janela")
	command.Flags().StringVar(&serviceName, "service", "", "nome do serviço")
	command.Flags().StringVar(&until, "until", "", "fim da janela de incidente em RFC 3339")
	return command
}

// splitServiceList aceita vários serviços separados por vírgula em --service.
// Um único nome não vira lista explícita: ele permanece o serviço de entrada da
// expansão, que é o comportamento padrão.
func splitServiceList(value string) []string {
	parts := strings.Split(value, ",")
	if len(parts) < 2 {
		return nil
	}
	services := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			services = append(services, trimmed)
		}
	}
	return services
}

// rankingConfig traduz somente opções validadas do bootstrap para o contrato
// puro do motor, evitando que o pacote de ranking dependa do formato YAML.
func rankingConfig(configuration config.Config) ranking.Config {
	return ranking.Config{
		Weights: ranking.Weights{
			ErrorRateDelta:      configuration.Ranking.Weights.ErrorRateDelta,
			DeploymentProximity: configuration.Ranking.Weights.DeploymentProximity,
			DatabaseEvidence:    configuration.Ranking.Weights.DatabaseEvidence,
			GraphProximity:      configuration.Ranking.Weights.GraphProximity,
			LatencyDelta:        configuration.Ranking.Weights.LatencyDelta,
			LogCorrelation:      configuration.Ranking.Weights.LogCorrelation,
		},
		TopN: configuration.Investigation.TopSuspects,
	}
}

// diagnosisEnd interpreta o instante de referência opcional para reproduzir investigações históricas.
func diagnosisEnd(until string) (time.Time, error) {
	if strings.TrimSpace(until) == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, until)
	if err != nil {
		return time.Time{}, fmt.Errorf("diagnosticar incidente: --until deve usar RFC 3339: %w", err)
	}
	return parsed.UTC(), nil
}
