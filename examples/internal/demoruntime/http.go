package demoruntime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

// SignalContext cancela o contexto ao receber interrupção ou encerramento do
// orquestrador, permitindo que recursos sejam fechados na ordem correta.
func SignalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}

// NewHTTPServer cria health e rota de negócio instrumentada com limites iguais
// para os serviços, sem armazenar sessão ou outro estado no servidor HTTP.
func NewHTTPServer(port int, route string, handler http.Handler) (*http.Server, error) {
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("porta deve estar entre 1 e 65535")
	}
	if route == "" || handler == nil {
		return nil, errors.New("rota e handler são obrigatórios")
	}
	mux := http.NewServeMux()
	path := route
	if fields := strings.Fields(route); len(fields) > 1 {
		path = fields[len(fields)-1]
	}
	routeHandler := otelhttp.WithRouteTag(path, handler)
	mux.Handle(route, otelhttp.NewHandler(routeHandler, path, otelhttp.WithSpanNameFormatter(func(_ string, request *http.Request) string {
		return request.Method + " " + path
	})))
	mux.HandleFunc("GET /health", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if _, err := writer.Write([]byte(`{"status":"ok"}`)); err != nil {
			return
		}
	})
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}, nil
}

// RunHTTPServer serve até cancelamento e limita o tempo reservado ao shutdown.
func RunHTTPServer(ctx context.Context, server *http.Server, shutdownTimeout time.Duration) error {
	if err := ctx.Err(); err != nil {
		return nil
	}
	errorsChannel := make(chan error, 1)
	go func() {
		errorsChannel <- server.ListenAndServe()
	}()
	select {
	case err := <-errorsChannel:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("servir HTTP: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("encerrar servidor HTTP: %w", err)
		}
		if err := <-errorsChannel; !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("servir HTTP durante encerramento: %w", err)
		}
		return nil
	}
}
