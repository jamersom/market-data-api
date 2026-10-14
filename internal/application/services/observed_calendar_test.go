package services

import (
	"context"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

func calendarDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestObservedCoveragePolicy(t *testing.T) {
	base := []domain.CalendarCoverage{{Year: 2024, From: calendarDate(2024, 1, 2), To: calendarDate(2024, 12, 30), IntegrityValidated: true}, {Year: 2025, From: calendarDate(2025, 1, 2), To: calendarDate(2025, 9, 1), IntegrityValidated: true}}
	for _, tc := range []struct {
		name     string
		coverage []domain.CalendarCoverage
		from, to time.Time
		want     string
	}{
		{"cross year", base, calendarDate(2024, 12, 27), calendarDate(2025, 1, 3), ""},
		{"missing year", base[:1], calendarDate(2024, 12, 27), calendarDate(2025, 1, 3), "calendar_missing_year"},
		{"before bounds", base, calendarDate(2024, 1, 1), calendarDate(2024, 2, 1), "calendar_outside_coverage"},
		{"after bounds", base, calendarDate(2025, 8, 1), calendarDate(2025, 9, 2), "calendar_outside_coverage"},
		{"duplicate", append(append([]domain.CalendarCoverage{}, base...), base[0]), calendarDate(2024, 1, 2), calendarDate(2024, 2, 1), "calendar_invalid_coverage"},
		{"bad integrity", []domain.CalendarCoverage{{Year: 2024, From: base[0].From, To: base[0].To}}, calendarDate(2024, 1, 2), calendarDate(2024, 2, 1), "calendar_invalid_coverage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := observedCoverageReason(tc.coverage, tc.from, tc.to); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

type observedHistoryStub struct{ history outbound.IntelligenceHistory }

func (r observedHistoryStub) FindIntelligenceHistories(_ context.Context, tickers []string, _ int, _ time.Time) (map[string]outbound.IntelligenceHistory, error) {
	histories := make(map[string]outbound.IntelligenceHistory, len(tickers))
	for _, ticker := range tickers {
		histories[ticker] = r.history
	}
	return histories, nil
}

func TestObservedCalendarEnablesMetricsAndRejectsGaps(t *testing.T) {
	h := outbound.IntelligenceHistory{Calendar: domain.IntelligenceCalendarMetadata{Source: "cotahist_observed", Policy: "observed_import_integrity_v1", Version: "test", Coverage: []domain.CalendarCoverage{{Year: 2024, From: calendarDate(2024, 1, 1), To: calendarDate(2024, 12, 31), IntegrityValidated: true}}}}
	for i := 0; i < 252; i++ {
		date := calendarDate(2024, 1, 1).AddDate(0, 0, i)
		h.Sessions = append(h.Sessions, date)
		h.Records = append(h.Records, domain.QuoteRecord{Quote: domain.Quote{Ticker: "PETR4", MarketType: 10, Currency: "BRL", TradingDate: date, ClosePriceCents: 10000, TradedVolumeCents: 100}})
	}
	execute := func() inbound.GetAssetIntelligenceOutput {
		t.Helper()
		out, err := NewGetAssetIntelligenceService(observedHistoryStub{h}, nil).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4"})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	out := execute()
	if out.RSI14 == nil || *out.RSI14 != 50 || out.SMA20Cents == nil || out.Volatility30D == nil || out.DrawdownCurrent == nil || out.Calendar.OfficialVerified {
		t.Fatalf("observed metrics not available: %+v", out)
	}
	// Missing early quote only affects RSI/full-history metrics; recent windows survive.
	h.Records = append(h.Records[:1], h.Records[2:]...)
	out = execute()
	if out.RSI14 != nil || out.SMA20Cents == nil {
		t.Fatal("gap handling should be per window")
	}
	h.Calendar.Coverage[0].IntegrityValidated = false
	out = execute()
	if out.SMA20Cents != nil {
		t.Fatal("invalid import accepted")
	}
}
