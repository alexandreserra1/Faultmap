// O payment-service persiste pagamentos em PostgreSQL com um pool único e instrumentado.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/faultmap/faultmap/examples/demo-shop/internal/demoruntime"
	"github.com/faultmap/faultmap/examples/demo-shop/internal/observability"
	"github.com/faultmap/faultmap/examples/demo-shop/internal/payment"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	shutdownTimeout = 5 * time.Second
	pingTimeout     = 5 * time.Second
)

type config struct {
	port                  int
	serviceVersion        string
	deploymentEnvironment string
	otelEndpoint          string
	databaseURL           string
	maxOpenConns          int
	maxIdleConns          int
	connMaxLifetime       time.Duration
	connMaxIdleTime       time.Duration
	dbDelay               time.Duration
	forceHTTPStatus       int
	chronicErrorPercent   int
}

// main transforma falhas de bootstrap em encerramento explícito do processo.
func main() {
	ctx, cancel := demoruntime.SignalContext(context.Background())
	defer cancel()
	if err := run(ctx, demoruntime.NewEnvironment(os.LookupEnv)); err != nil {
		log.Printf("payment-service encerrado com erro: %v", err)
		os.Exit(1)
	}
}

// loadConfig valida pool, cenário e identidade antes de abrir o PostgreSQL.
func loadConfig(environment demoruntime.Environment) (config, error) {
	port, err := environment.Port("PORT", 8080)
	if err != nil {
		return config{}, err
	}
	serviceVersion, err := environment.Required("SERVICE_VERSION")
	if err != nil {
		return config{}, err
	}
	deploymentEnvironment, err := environment.Required("DEPLOYMENT_ENVIRONMENT")
	if err != nil {
		return config{}, err
	}
	otelEndpoint, err := environment.Required("OTEL_EXPORTER_OTLP_ENDPOINT")
	if err != nil {
		return config{}, err
	}
	databaseURL, err := environment.Required("DATABASE_URL")
	if err != nil {
		return config{}, err
	}
	maxOpenConns, err := environment.Int("DB_MAX_OPEN_CONNS", 10, 1, 1000)
	if err != nil {
		return config{}, err
	}
	maxIdleConns, err := environment.Int("DB_MAX_IDLE_CONNS", 5, 0, 1000)
	if err != nil {
		return config{}, err
	}
	if maxIdleConns > maxOpenConns {
		return config{}, errors.New("DB_MAX_IDLE_CONNS não pode exceder DB_MAX_OPEN_CONNS")
	}
	connMaxLifetime, err := environment.Duration("DB_CONN_MAX_LIFETIME", 30*time.Minute, true)
	if err != nil {
		return config{}, err
	}
	connMaxIdleTime, err := environment.Duration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute, true)
	if err != nil {
		return config{}, err
	}
	dbDelay, err := environment.Duration("DB_DELAY", 0, true)
	if err != nil {
		return config{}, err
	}
	forceHTTPStatus, err := environment.Int("FORCE_HTTP_STATUS", 0, 0, 599)
	if err != nil {
		return config{}, err
	}
	if forceHTTPStatus != 0 && forceHTTPStatus < 400 {
		return config{}, errors.New("FORCE_HTTP_STATUS deve ser zero ou um status entre 400 e 599")
	}
	chronicErrorPercent, err := environment.Int("CHRONIC_ERROR_PERCENT", 0, 0, 100)
	if err != nil {
		return config{}, err
	}
	return config{
		port: port, serviceVersion: serviceVersion, deploymentEnvironment: deploymentEnvironment, otelEndpoint: otelEndpoint,
		databaseURL: databaseURL, maxOpenConns: maxOpenConns, maxIdleConns: maxIdleConns,
		connMaxLifetime: connMaxLifetime, connMaxIdleTime: connMaxIdleTime, dbDelay: dbDelay, forceHTTPStatus: forceHTTPStatus,
		chronicErrorPercent: chronicErrorPercent,
	}, nil
}

// run abre um único pool, comprova conectividade e o injeta no repositório.
func run(ctx context.Context, environment demoruntime.Environment) (runErr error) {
	settings, err := loadConfig(environment)
	if err != nil {
		return fmt.Errorf("carregar configuração: %w", err)
	}
	shutdown, err := observability.Setup(ctx, observability.Config{
		Endpoint: settings.otelEndpoint, ServiceName: "payment-service", ServiceVersion: settings.serviceVersion,
		DeploymentEnvironment: settings.deploymentEnvironment,
	})
	if err != nil {
		return fmt.Errorf("configurar observabilidade: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		runErr = errors.Join(runErr, shutdown(shutdownCtx))
	}()

	database, err := sql.Open("pgx", settings.databaseURL)
	if err != nil {
		return fmt.Errorf("abrir pool PostgreSQL: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, database.Close()) }()
	database.SetMaxOpenConns(settings.maxOpenConns)
	database.SetMaxIdleConns(settings.maxIdleConns)
	database.SetConnMaxLifetime(settings.connMaxLifetime)
	database.SetConnMaxIdleTime(settings.connMaxIdleTime)
	pingCtx, cancelPing := context.WithTimeout(ctx, pingTimeout)
	pingErr := database.PingContext(pingCtx)
	cancelPing()
	if pingErr != nil {
		return fmt.Errorf("conectar ao PostgreSQL: %w", pingErr)
	}

	repository, err := payment.NewPostgresRepository(database, payment.RepositoryConfig{Delay: settings.dbDelay})
	if err != nil {
		return fmt.Errorf("criar repositório de pagamentos: %w", err)
	}
	handler, err := payment.NewHandler(repository, payment.Scenario{
		ForceHTTPStatus:     settings.forceHTTPStatus,
		ChronicErrorPercent: settings.chronicErrorPercent,
	})
	if err != nil {
		return fmt.Errorf("criar handler de pagamentos: %w", err)
	}
	server, err := demoruntime.NewHTTPServer(settings.port, "POST /payment", handler)
	if err != nil {
		return fmt.Errorf("criar servidor HTTP: %w", err)
	}
	log.Printf("payment-service ouvindo em %s", server.Addr)
	return demoruntime.RunHTTPServer(ctx, server, shutdownTimeout)
}
