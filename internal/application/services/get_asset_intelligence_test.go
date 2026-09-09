package services

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type intelligenceRepositoryStub struct {
	history outbound.IntelligenceHistory
	err     error
	calls   int
	ticker  string
	market  int
	cutoff  time.Time
	cancel  context.CancelFunc
}

func (r *intelligenceRepositoryStub) FindIntelligenceHistory(ctx context.Context, ticker string, market int, cutoff time.Time) (outbound.IntelligenceHistory, error) {
	r.calls++
	r.ticker, r.market, r.cutoff = ticker, market, cutoff
	if r.cancel != nil {
		r.cancel()
	}
	return r.history, r.err
}

func intelligenceFixture(n int) *intelligenceRepositoryStub {
	r := &intelligenceRepositoryStub{history: outbound.IntelligenceHistory{CalendarVerified: true, DataVersion: "fixture-v1"}}
	// Synthetic calendar: consecutive fixture dates are declared sessions.
	for i := 0; i < n; i++ {
		date := time.Date(2024, 1, 1+i, 0, 0, 0, 0, time.UTC)
		r.history.Sessions = append(r.history.Sessions, date)
		r.history.Records = append(r.history.Records, domain.QuoteRecord{Quote: domain.Quote{
			Ticker: "PETR4", MarketType: 10, Currency: "BRL", TradingDate: date,
			ClosePriceCents: 10000, TradedVolumeCents: 100000,
		}})
	}
	return r
}

func intelligenceService(r *intelligenceRepositoryStub) *GetAssetIntelligenceService {
	s := NewGetAssetIntelligenceService(r, nil)
	s.now = func() time.Time { return time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC) }
	return s
}

func hasUnavailable(out inbound.GetAssetIntelligenceOutput, field, reason string) bool {
	for _, entry := range out.Unavailable {
		if entry.Field == field && entry.Reason == reason {
			return true
		}
	}
	return false
}

func TestIntelligenceCompleteCore(t *testing.T) {
	r := intelligenceFixture(252)
	out, err := intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: " petr4 "})
	if err != nil {
		t.Fatal(err)
	}
	if r.calls != 1 || r.ticker != "PETR4" || r.market != 10 || r.cutoff.Format(time.DateOnly) != "2026-09-06" {
		t.Fatalf("query: %+v", r)
	}
	for name, value := range map[string]*float64{"return": out.Return7D, "distance": out.DistanceSMA20, "volatility": out.Volatility30D, "current": out.DrawdownCurrent, "maximum": out.MaximumDrawdown252D} {
		if value == nil || *value != 0 {
			t.Fatalf("%s = %v, want available zero", name, value)
		}
	}
	if out.SMA20Cents == nil || *out.SMA20Cents != 10000 || out.RSI14 == nil || *out.RSI14 != 50 || out.AverageDailyVolume20DCents == nil || *out.AverageDailyVolume20DCents != 100000 {
		t.Fatalf("metrics: %+v", out)
	}
	if out.RequestedAsOf != nil || out.Status != "complete" || out.DataVersion != "fixture-v1" || out.PriceAdjustment != "unadjusted" || len(out.Unavailable) != 0 {
		t.Fatalf("metadata: %+v", out)
	}
}

func TestIntelligenceAsOfAndNoMutation(t *testing.T) {
	r := intelligenceFixture(32)
	r.history.Records[31].Quote.ClosePriceCents = 999999
	// Out-of-order input must be copied and sorted, not modified.
	r.history.Records[0], r.history.Records[30] = r.history.Records[30], r.history.Records[0]
	before := append([]domain.QuoteRecord(nil), r.history.Records...)
	date := r.history.Sessions[30]
	out, err := intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4", AsOf: date})
	if err != nil {
		t.Fatal(err)
	}
	if out.AsOf != date || out.RequestedAsOf == nil || *out.RequestedAsOf != date || out.Price.ClosePriceCents != 10000 || out.Return7D == nil || *out.Return7D != 0 {
		t.Fatalf("future data leaked: %+v", out)
	}
	if !reflect.DeepEqual(before, r.history.Records) {
		t.Fatal("repository history mutated")
	}
}

func TestIntelligenceHistoryBoundaries(t *testing.T) {
	for _, n := range []int{1, 7, 8, 14, 15, 19, 20, 30, 31, 251, 252} {
		out, err := intelligenceService(intelligenceFixture(n)).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4"})
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			value *float64
			min   int
		}{{out.Return7D, 8}, {out.SMA20Cents, 20}, {out.DistanceSMA20, 20}, {out.RSI14, 15}, {out.Volatility30D, 31}, {out.DrawdownCurrent, 252}, {out.MaximumDrawdown252D, 252}} {
			if (tc.value != nil) != (n >= tc.min) {
				t.Fatalf("history %d: minimum %d, value %v", n, tc.min, tc.value)
			}
		}
		if (out.AverageDailyVolume20DCents != nil) != (n >= 20) {
			t.Fatalf("volume boundary %d", n)
		}
	}
}

func TestIntelligenceCalendarAndBadData(t *testing.T) {
	for _, tc := range []struct {
		name          string
		change        func(*intelligenceRepositoryStub)
		field, reason string
	}{
		{"calendar absent", func(r *intelligenceRepositoryStub) { r.history.CalendarVerified = false }, "returns.return_7d", "calendar_unavailable"},
		{"missing session", func(r *intelligenceRepositoryStub) {
			r.history.Records = append(r.history.Records[:28], r.history.Records[29:]...)
		}, "returns.return_7d", "missing_sessions"},
		{"invalid close", func(r *intelligenceRepositoryStub) { r.history.Records[30].Quote.ClosePriceCents = 0 }, "returns.return_7d", "invalid_data"},
		{"invalid volume", func(r *intelligenceRepositoryStub) { r.history.Records[30].Quote.TradedVolumeCents = -1 }, "liquidity.average_daily_volume_20d", "invalid_data"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := intelligenceFixture(31)
			tc.change(r)
			out, err := intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4"})
			if err != nil {
				t.Fatal(err)
			}
			if !hasUnavailable(out, tc.field, tc.reason) {
				t.Fatalf("unavailable: %+v", out.Unavailable)
			}
		})
	}
}

func TestIntelligenceErrors(t *testing.T) {
	for _, tc := range []struct {
		input inbound.GetAssetIntelligenceInput
		want  error
	}{
		{inbound.GetAssetIntelligenceInput{Ticker: "!"}, domain.ErrInvalidTicker},
		{inbound.GetAssetIntelligenceInput{Ticker: "PETR4", MarketType: -1}, domain.ErrInvalidMarketType},
		{inbound.GetAssetIntelligenceInput{Ticker: "PETR4", AsOf: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)}, domain.ErrInvalidDateRange},
	} {
		r := intelligenceFixture(31)
		_, err := intelligenceService(r).Execute(context.Background(), tc.input)
		if !errors.Is(err, tc.want) || r.calls != 0 {
			t.Fatalf("error %v, calls %d", err, r.calls)
		}
	}
	r := intelligenceFixture(0)
	_, err := intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4"})
	if !errors.Is(err, domain.ErrQuoteNotFound) {
		t.Fatalf("not found: %v", err)
	}
	r.err = errors.New("database unavailable")
	_, err = intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4"})
	if !errors.Is(err, r.err) {
		t.Fatalf("infrastructure: %v", err)
	}
	for _, after := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		r := intelligenceFixture(31)
		if after {
			r.cancel = cancel
		} else {
			cancel()
		}
		_, err := intelligenceService(r).Execute(ctx, inbound.GetAssetIntelligenceInput{Ticker: "PETR4"})
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation: %v", err)
		}
	}
}

func TestIntelligenceRSIUsesFullSeed(t *testing.T) {
	r := intelligenceFixture(16)
	// First 14 changes: seven gains of 100 and seven losses of 100.
	for i := range r.history.Records {
		if i%2 == 1 {
			r.history.Records[i].Quote.ClosePriceCents = 10100
		}
	}
	// Seed averages are 50/50; next gain 100 gives 750/14 and 650/14.
	out, err := intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4"})
	if err != nil {
		t.Fatal(err)
	}
	if out.RSI14 == nil || math.Abs(*out.RSI14-750.0/14) > 1e-10 || out.RSISeedFrom != r.history.Sessions[0] {
		t.Fatalf("RSI seed: %+v", out)
	}
}

func TestIntelligenceRejectsCorruptHistory(t *testing.T) {
	for _, change := range []func(*intelligenceRepositoryStub){
		func(r *intelligenceRepositoryStub) {
			r.history.Records[1].Quote.TradingDate = r.history.Records[0].Quote.TradingDate
		},
		func(r *intelligenceRepositoryStub) { r.history.Records[0].Quote.TradingDate = time.Time{} },
		func(r *intelligenceRepositoryStub) { r.history.Records[0].Quote.Currency = "USD" },
		func(r *intelligenceRepositoryStub) { r.history.Records[0].Quote.Ticker = "VALE3" },
		func(r *intelligenceRepositoryStub) { r.history.Records[0].Quote.MarketType = 20 },
	} {
		r := intelligenceFixture(31)
		change(r)
		if _, err := intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4"}); err == nil {
			t.Fatal("accepted corrupt history")
		}
	}
}
