package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/faultmap/faultmap/internal/application"
	"github.com/faultmap/faultmap/internal/integrations/otlphttp"
	"github.com/faultmap/faultmap/internal/platform/config"
	storage "github.com/faultmap/faultmap/internal/storage/sqlite"
	"github.com/faultmap/faultmap/internal/telemetry/normalizer"
	"github.com/spf13/cobra"
)

type serverTimeouts struct {
	readHeader time.Duration
	read       time.Duration
	write      time.Duration
	idle       time.Duration
	shutdown   time.Duration
}

// newServeCommand inicia os listeners OTLP e de health usando um único pool
// SQLite durante todo o ciclo de vida do processo.
func newServeCommand() *cobra.Command {
	var configPath string
	command := &cobra.Command{
		Use:   "serve",
		Short: "Recebe traces por OTLP HTTP",
		RunE: func(command *cobra.Command, _ []string) (runErr error) {
			loadedConfig, err := config.Load(command.Context(), configPath)
			if err != nil {
				return fmt.Errorf("carregar configuração: %w", err)
			}

			database, err := storage.Open(command.Context(), resolveStoragePath(configPath, loadedConfig.Storage.Path))
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

			repository := storage.NewSignalRepository(database)
			// A política é construída uma vez no bootstrap e reutilizada por todas
			// as requisições; o receiver permanece stateless.
			privacyPolicy := privacyPolicyFrom(loadedConfig)
			ingester := otlphttp.IngestFunc(func(ctx context.Context, reader io.Reader, encoding otlphttp.Encoding) error {
				normalizerEncoding, err := mapOTLPEncoding(encoding)
				if err != nil {
					return errors.Join(otlphttp.ErrInvalidPayload, err)
				}
				_, err = application.IngestTelemetry(ctx, reader, normalizerEncoding, privacyPolicy, repository)
				if errors.Is(err, normalizer.ErrInvalidOTLP) {
					return errors.Join(otlphttp.ErrInvalidPayload, err)
				}
				return err
			})
			handler, err := otlphttp.NewHandler(ingester, otlphttp.Options{MaxRequestBodyBytes: loadedConfig.Server.MaxRequestBodyBytes})
			if err != nil {
				return err
			}
			return serveHTTP(command.Context(), command.OutOrStdout(), loadedConfig.Server, handler, otlphttp.NewHealthHandler())
		},
	}
	command.Flags().StringVar(&configPath, "config", "faultmap.yaml", "caminho da configuração YAML")
	return command
}

func mapOTLPEncoding(encoding otlphttp.Encoding) (normalizer.OTLPEncoding, error) {
	switch encoding {
	case otlphttp.EncodingJSON:
		return normalizer.OTLPEncodingJSON, nil
	case otlphttp.EncodingProtobuf:
		return normalizer.OTLPEncodingProtobuf, nil
	default:
		return "", fmt.Errorf("codificação OTLP HTTP %q não suportada", encoding)
	}
}

// serveHTTP abre ambos os listeners antes de anunciar disponibilidade e drena
// requisições em andamento quando o contexto do processo é cancelado.
func serveHTTP(ctx context.Context, output io.Writer, serverConfig config.ServerConfig, otlpHandler, healthHandler http.Handler) error {
	timeouts, err := parseServerTimeouts(serverConfig)
	if err != nil {
		return err
	}

	listenConfig := net.ListenConfig{}
	otlpListener, err := listenConfig.Listen(ctx, "tcp", serverConfig.OTLPHTTPAddress)
	if err != nil {
		return fmt.Errorf("abrir listener OTLP HTTP %q: %w", serverConfig.OTLPHTTPAddress, err)
	}
	healthListener, err := listenConfig.Listen(ctx, "tcp", serverConfig.HealthAddress)
	if err != nil {
		if closeErr := otlpListener.Close(); closeErr != nil {
			return fmt.Errorf("abrir listener de health %q: %w; fechar listener OTLP: %v", serverConfig.HealthAddress, err, closeErr)
		}
		return fmt.Errorf("abrir listener de health %q: %w", serverConfig.HealthAddress, err)
	}

	servers := []*http.Server{
		newHTTPServer(serverConfig.OTLPHTTPAddress, otlpHandler, timeouts),
		newHTTPServer(serverConfig.HealthAddress, healthHandler, timeouts),
	}
	listeners := []net.Listener{otlpListener, healthListener}
	serveErrors := make(chan error, len(servers))
	for index, server := range servers {
		go func(server *http.Server, listener net.Listener) {
			serveErrors <- server.Serve(listener)
		}(server, listeners[index])
	}
	if _, err := fmt.Fprintf(output, "Faultmap recebendo OTLP HTTP em %s; health em %s.\n", serverConfig.OTLPHTTPAddress, serverConfig.HealthAddress); err != nil {
		shutdownErr := shutdownHTTPServers(ctx, servers, timeouts.shutdown)
		return errors.Join(fmt.Errorf("escrever status do servidor: %w", err), shutdownErr)
	}

	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			serveErr = fmt.Errorf("executar servidor HTTP: %w", err)
		}
	}

	shutdownErr := shutdownHTTPServers(ctx, servers, timeouts.shutdown)
	return errors.Join(serveErr, shutdownErr)
}

func newHTTPServer(address string, handler http.Handler, timeouts serverTimeouts) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: timeouts.readHeader,
		ReadTimeout:       timeouts.read,
		WriteTimeout:      timeouts.write,
		IdleTimeout:       timeouts.idle,
	}
}

// shutdownHTTPServers usa um contexto derivado sem o cancelamento original para
// permitir que requisições já aceitas terminem dentro do prazo configurado.
func shutdownHTTPServers(ctx context.Context, servers []*http.Server, timeout time.Duration) error {
	shutdownContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	errorsByServer := make(chan error, len(servers))
	for _, server := range servers {
		go func(server *http.Server) {
			errorsByServer <- server.Shutdown(shutdownContext)
		}(server)
	}
	var shutdownErr error
	for range servers {
		if err := <-errorsByServer; err != nil {
			shutdownErr = errors.Join(shutdownErr, fmt.Errorf("encerrar servidor HTTP: %w", err))
		}
	}
	return shutdownErr
}

func parseServerTimeouts(serverConfig config.ServerConfig) (serverTimeouts, error) {
	values := []struct {
		name        string
		value       string
		destination *time.Duration
	}{
		{name: "read_header_timeout", value: serverConfig.ReadHeaderTimeout},
		{name: "read_timeout", value: serverConfig.ReadTimeout},
		{name: "write_timeout", value: serverConfig.WriteTimeout},
		{name: "idle_timeout", value: serverConfig.IdleTimeout},
		{name: "shutdown_timeout", value: serverConfig.ShutdownTimeout},
	}
	parsed := serverTimeouts{}
	values[0].destination = &parsed.readHeader
	values[1].destination = &parsed.read
	values[2].destination = &parsed.write
	values[3].destination = &parsed.idle
	values[4].destination = &parsed.shutdown
	for _, value := range values {
		duration, err := time.ParseDuration(value.value)
		if err != nil || duration <= 0 {
			return serverTimeouts{}, fmt.Errorf("interpretar server.%s: duração deve ser válida e positiva", value.name)
		}
		*value.destination = duration
	}
	return parsed, nil
}
