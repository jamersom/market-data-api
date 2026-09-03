package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	httpadapter "github.com/jamersom/market-data-api/internal/adapters/inbound/http"
	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/handlers"
	"github.com/jamersom/market-data-api/internal/adapters/outbound/postgres"
	"github.com/jamersom/market-data-api/internal/application/services"
	"github.com/jamersom/market-data-api/internal/infra/database"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf(".env not loaded: %v", err)
	}

	ctx := context.Background()

	db, err := database.NewPostgresPool(ctx)
	if err != nil {
		log.Fatalf("initialize PostgreSQL: %v", err)
	}

	defer db.Close()

	quoteRepository := postgres.NewQuoteRepository(db)
	getQuoteService := services.NewGetQuoteService(quoteRepository)
	getQuotesByPeriodService := services.NewGetQuotesByPeriodService(quoteRepository)
	quoteHandler := handlers.NewQuoteHandler(getQuoteService)
	quoteHistoryHandler := handlers.NewQuoteHistoryHandler(getQuotesByPeriodService)

	mux := http.NewServeMux()
	httpadapter.RegisterRoutes(mux, quoteHandler, quoteHistoryHandler)

	server := &http.Server{
		Addr:              serverAddress(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("market-data-api listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func serverAddress() string {
	if value := os.Getenv("SERVER_ADDRESS"); value != "" {
		return value
	}
	return ":8080"
}
