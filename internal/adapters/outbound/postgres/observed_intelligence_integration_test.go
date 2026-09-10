package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/handlers"
	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/response"
	"github.com/jamersom/market-data-api/internal/application/services"
	"github.com/jamersom/market-data-api/internal/infra/database"
	"github.com/joho/godotenv"
)

func TestObservedIntelligenceIntegration(t *testing.T) {
	if os.Getenv("CALENDAR_INTEGRATION_TEST") != "1" {
		t.Skip("set CALENDAR_INTEGRATION_TEST=1 for read-only calendar/HTTP checks")
	}
	_ = godotenv.Load("../../../../.env")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.NewPostgresPool(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repository := NewObservedIntelligenceQuoteRepository(pool)
	latest, err := NewQuoteRepository(pool).FindLatestByTicker(ctx, "PETR4", 10)
	if err != nil {
		t.Fatal(err)
	}
	asOf := latest.Quote.TradingDate
	start := time.Now()
	h, err := repository.FindIntelligenceHistory(ctx, "PETR4", 10, asOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Sessions) == 0 || len(h.Calendar.Coverage) == 0 || h.Calendar.Version == "" || h.CalendarVerified {
		t.Fatalf("calendar missing: %+v", h.Calendar)
	}
	for _, date := range h.Sessions {
		if date.After(asOf) {
			t.Fatal("future session leaked")
		}
	}
	old, err := repository.FindIntelligenceHistory(ctx, "PETR4", 10, asOf.AddDate(0, 0, -30))
	if err != nil {
		t.Fatal(err)
	}
	for _, date := range old.Sessions {
		if date.After(asOf.AddDate(0, 0, -30)) {
			t.Fatal("historical cutoff leaked")
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /assets/{ticker}/intelligence", handlers.NewIntelligenceHandler(services.NewGetAssetIntelligenceService(repository, nil)).Get)
	rec := httptest.NewRecorder()
	httpStart := time.Now()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/assets/PETR4/intelligence?asOf="+asOf.Format(time.DateOnly)+"&includeDetails=true", nil).WithContext(ctx))
	if rec.Code != 200 {
		t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
	}
	var body response.IntelligenceEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Meta.Calendar.Source != "cotahist_observed" || body.Meta.Calendar.OfficialVerified || body.Meta.Calendar.Version == "" || body.Data.Trend.SMA20 == nil || body.Data.Returns.Return7D == nil || body.Data.Risk.Volatility30D == nil {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
	t.Logf("records=%d sessions=%d coverage_years=%d total=%s HTTP=%s RSI_available=%v", len(h.Records), len(h.Sessions), len(h.Calendar.Coverage), time.Since(start), time.Since(httpStart), body.Data.Momentum.RSI14 != nil)
	t.Logf("unavailable=%+v", body.Meta.Unavailable)
	for year := h.Records[0].Quote.TradingDate.Year(); year <= asOf.Year(); year++ {
		found := false
		for _, coverage := range h.Calendar.Coverage {
			if coverage.Year == year {
				found = true
			}
		}
		if !found {
			t.Logf("missing calendar year=%d", year)
		}
	}
}
