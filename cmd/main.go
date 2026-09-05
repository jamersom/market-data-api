package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	logger "github.com/jamersom/go-logging"
	httpadapter "github.com/jamersom/market-data-api/internal/adapters/inbound/http"
	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/handlers"
	"github.com/jamersom/market-data-api/internal/adapters/outbound/postgres"
	"github.com/jamersom/market-data-api/internal/application/services"
	"github.com/jamersom/market-data-api/internal/infra/database"
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

	if err := run(appLogger); err != nil {
		appLogger.Error("market-data-api stopped", "error", err)
		os.Exit(1)
	}
}

func run(appLogger *slog.Logger) error {
	ctx := context.Background()

	db, err := database.NewPostgresPool(ctx)
	if err != nil {
		return fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	defer db.Close()

	appLogger.Info("PostgreSQL connection established")

	quoteRepository := postgres.NewQuoteRepository(db)
	getQuoteService := services.NewGetQuoteService(quoteRepository, appLogger)
	getQuotesByPeriodService := services.NewGetQuotesByPeriodService(quoteRepository, appLogger)
	quoteHandler := handlers.NewQuoteHandler(getQuoteService)
	quoteHistoryHandler := handlers.NewQuoteHistoryHandler(getQuotesByPeriodService)
	compareQuotesService := services.NewCompareQuotesService(quoteRepository, appLogger)
	comparisonsHandler := handlers.NewComparisonsHandler(compareQuotesService)

	mux := http.NewServeMux()
	httpadapter.RegisterRoutes(mux, quoteHandler, quoteHistoryHandler, comparisonsHandler)

	server := &http.Server{
		Addr:              serverAddress(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	appLogger.Info(
		"HTTP server started",
		"address", server.Addr,
	)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	return nil
}

func serverAddress() string {
	if value := os.Getenv("SERVER_ADDRESS"); value != "" {
		return value
	}
	return ":8080"
}
