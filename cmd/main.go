package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	logger "github.com/jamersom/go-logging"
	httpadapter "github.com/jamersom/market-data-api/internal/adapters/inbound/http"
	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/handlers"
	metricsadapter "github.com/jamersom/market-data-api/internal/adapters/outbound/metrics"
	"github.com/jamersom/market-data-api/internal/adapters/outbound/postgres"
	"github.com/jamersom/market-data-api/internal/application/services"
	"github.com/jamersom/market-data-api/internal/infra/database"
	"github.com/jamersom/market-data-api/internal/observability"
	"github.com/joho/godotenv"
)

var version = "development"

func main() {
	envErr := godotenv.Load()

	appLogger := logger.New(logger.Config{
		Level:       os.Getenv("LOG_LEVEL"),
		Format:      os.Getenv("LOG_FORMAT"),
		Service:     "market-data-api",
		Environment: os.Getenv("APP_ENV"),
		Version:     version,
		AddSource:   true,
	})
	slog.SetDefault(appLogger)

	if envErr != nil {
		appLogger.Debug(
			".env not loaded; using system environment variables",
			"error", envErr,
		)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, appLogger); err != nil {
		appLogger.Error("market-data-api stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, appLogger *slog.Logger) error {
	db, err := database.NewPostgresPool(ctx)
	if err != nil {
		return fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	defer func() {
		db.Close()
		appLogger.Info("PostgreSQL connection pool closed")
	}()

	appLogger.Info("PostgreSQL connection established")

	registry := observability.NewRegistry()
	quoteRepository := metricsadapter.NewQuoteRepository(postgres.NewQuoteRepository(db), registry)
	getQuoteService := services.NewGetQuoteService(quoteRepository, appLogger)
	getQuotesByPeriodService := services.NewGetQuotesByPeriodService(quoteRepository, appLogger)
	quoteHandler := handlers.NewQuoteHandler(getQuoteService)
	quoteHistoryHandler := handlers.NewQuoteHistoryHandler(getQuotesByPeriodService)
	compareQuotesService := services.NewCompareQuotesService(quoteRepository, appLogger)
	comparisonsHandler := handlers.NewComparisonsHandler(compareQuotesService)
	intelligenceRepository := metricsadapter.NewIntelligenceQuoteRepository(postgres.NewObservedIntelligenceQuoteRepository(db), registry)
	intelligenceService := services.NewGetAssetIntelligenceService(intelligenceRepository, appLogger)
	intelligenceHandle := handlers.NewIntelligenceHandler(intelligenceService)

	mux := http.NewServeMux()
	httpadapter.RegisterRoutes(mux, quoteHandler, quoteHistoryHandler, comparisonsHandler, intelligenceHandle)
	operationsHandler := handlers.NewOperationsHandler(db, registry, durationFromEnv("HEALTH_CHECK_TIMEOUT", 2*time.Second))
	httpadapter.RegisterOperationalRoutes(mux, operationsHandler)

	server := &http.Server{
		Addr:              serverAddress(),
		Handler:           httpadapter.MetricsMiddleware(mux, registry),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	appLogger.Info(
		"HTTP server started",
		"address", server.Addr,
	)

	return serveHTTP(ctx, server, appLogger, durationFromEnv("SHUTDOWN_TIMEOUT", 15*time.Second))
}

type managedHTTPServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

func serveHTTP(ctx context.Context, server managedHTTPServer, appLogger *slog.Logger, shutdownTimeout time.Duration) error {
	serverResult := make(chan error, 1)
	go func() { serverResult <- server.ListenAndServe() }()
	select {
	case err := <-serverResult:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-ctx.Done():
		appLogger.Info("HTTP shutdown requested", "timeout", shutdownTimeout)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			appLogger.Error("HTTP graceful shutdown failed", "error", err)
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		appLogger.Info("HTTP graceful shutdown completed")
		return nil
	}
}

func serverAddress() string {
	if value := os.Getenv("SERVER_ADDRESS"); value != "" {
		return value
	}
	return ":8080"
}

func durationFromEnv(name string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return defaultValue
	}
	return duration
}
