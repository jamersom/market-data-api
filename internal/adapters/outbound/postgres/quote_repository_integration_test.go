package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/domain"
	"github.com/jamersom/market-data-api/internal/infra/database"
	"github.com/joho/godotenv"
)

func TestQuoteRepositoryIntegration(t *testing.T) {
	if os.Getenv("DATABASE_INTEGRATION_TEST") != "1" {
		t.Skip("set DATABASE_INTEGRATION_TEST=1 to run")
	}
	if err := godotenv.Load("../../../../.env"); err != nil {
		t.Fatalf("load env: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.NewPostgresPool(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	repository := NewQuoteRepository(pool)

	latest, err := repository.FindLatestByTicker(ctx, "PETR4", 10)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest.Quote.Currency != "BRL" || latest.Quote.ISIN == "" || latest.PreviousCloseCents == nil {
		t.Fatalf("unexpected latest quote: %+v", latest)
	}

	page, err := repository.FindByTickerAndPeriod(ctx, "PETR4",
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC),
		10, 10, 0, domain.SortAscending)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if page.Total == 0 || len(page.Records) == 0 {
		t.Fatalf("expected history, got %+v", page)
	}
}
