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
	"github.com/jamersom/market-data-api/internal/domain/signals"
)

type intelligenceRepositoryStub struct {
	history    outbound.IntelligenceHistory
	benchmarks map[string]outbound.IntelligenceHistory
	err        error
	calls      int
	tickers    []string
	market     int
	cutoff     time.Time
	cancel     context.CancelFunc
}

func (r *intelligenceRepositoryStub) FindIntelligenceHistories(ctx context.Context, tickers []string, market int, cutoff time.Time) (map[string]outbound.IntelligenceHistory, error) {
	r.calls++
	r.tickers, r.market, r.cutoff = append([]string(nil), tickers...), market, cutoff
	if r.cancel != nil {
		r.cancel()
	}
	if r.err != nil {
		return nil, r.err
	}
	histories := make(map[string]outbound.IntelligenceHistory, len(tickers))
	for _, ticker := range tickers {
		if ticker == "PETR4" {
			histories[ticker] = r.history
		} else {
			histories[ticker] = r.benchmarks[ticker]
		}
	}
	return histories, nil
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

func benchmarkFixture(r *intelligenceRepositoryStub, ticker string, indexes []int) outbound.IntelligenceHistory {
	history := outbound.IntelligenceHistory{DataVersion: "benchmark-fixture-v1"}
	for _, index := range indexes {
		quote := r.history.Records[index].Quote
		quote.Ticker = ticker
		quote.ClosePriceCents = 10000
		history.Records = append(history.Records, domain.QuoteRecord{Quote: quote})
	}
	return history
}

func hasUnavailable(out inbound.GetAssetIntelligenceOutput, field, reason string) bool {
	for _, entry := range out.Unavailable {
		if entry.Field == field && entry.Reason == reason {
			return true
		}
	}
	return false
}

func signalByID(out inbound.GetAssetIntelligenceOutput, id string) signals.Evaluation {
	for _, evaluation := range out.Signals {
		if evaluation.ID == id {
			return evaluation
		}
	}
	return signals.Evaluation{}
}

func TestIntelligenceCompleteCore(t *testing.T) {
	r := intelligenceFixture(rsiPeriod + MinimumRSIPercentileObservations + 1)
	out, err := intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: " petr4 "})
	if err != nil {
		t.Fatal(err)
	}
	if r.calls != 1 || !reflect.DeepEqual(r.tickers, []string{"PETR4"}) || r.market != 10 || r.cutoff.Format(time.DateOnly) != "2026-09-06" {
		t.Fatalf("query: %+v", r)
	}
	for name, value := range map[string]*float64{"return": out.Return7D, "distance": out.DistanceSMA20, "volatility": out.Volatility30D, "current": out.DrawdownCurrent, "maximum": out.MaximumDrawdown252D} {
		if value == nil || *value != 0 {
			t.Fatalf("%s = %v, want available zero", name, value)
		}
	}
	if out.SMA20Cents == nil || *out.SMA20Cents != 10000 || out.SMA50Cents == nil || *out.SMA50Cents != 10000 || out.RSI14 == nil || *out.RSI14 != 50 || out.RSIPercentile == nil || out.RSIPercentile.Value != 50 || out.AverageDailyVolume20DCents == nil || *out.AverageDailyVolume20DCents != 100000 {
		t.Fatalf("metrics: %+v", out)
	}
	if out.RSIPercentile.WindowYears != DefaultRSIPercentileWindowYears {
		t.Fatalf("default RSI window = %d", out.RSIPercentile.WindowYears)
	}
	if out.RequestedAsOf != nil || out.Status != "complete" || out.DataVersion != "fixture-v1" || out.CalculationVersion != "1.2" || out.RulesetVersion != signals.RulesetV1Version || out.PriceAdjustment != "unadjusted" || len(out.Unavailable) != 0 {
		t.Fatalf("metadata: %+v", out)
	}
	if len(out.Signals) != 6 || signalByID(out, "rsi_overbought").Status != signals.StatusNotTriggered || signalByID(out, "sma20_above_sma50").Status != signals.StatusNotTriggered {
		t.Fatalf("signals: %+v", out.Signals)
	}
}

func TestIntelligenceBenchmarkRelativeStrength(t *testing.T) {
	r := intelligenceFixture(rsiPeriod + MinimumRSIPercentileObservations + 1)
	last := len(r.history.Records) - 1
	r.history.Records[last].Quote.ClosePriceCents = 11250
	indexes := make([]int, return7DPeriods+1)
	for i := range indexes {
		indexes[i] = last - return7DPeriods + i
	}
	benchmark := benchmarkFixture(r, "IBOV", indexes)
	benchmark.Records[len(benchmark.Records)-1].Quote.ClosePriceCents = 10720
	r.benchmarks = map[string]outbound.IntelligenceHistory{"IBOV": benchmark}

	service := intelligenceService(r)
	baseline, err := service.Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := service.Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4", Benchmark: " ibov "})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.tickers, []string{"PETR4", "IBOV"}) || out.Benchmark == nil || out.Benchmark.Ticker != "IBOV" {
		t.Fatalf("benchmark request: %+v, %+v", r.tickers, out.Benchmark)
	}
	if out.Return7D == nil || math.Abs(*out.Return7D-12.5) > 1e-12 || out.Benchmark.Return7D == nil || math.Abs(*out.Benchmark.Return7D-7.2) > 1e-12 || out.Benchmark.RelativeStrengthReturn7DPP == nil || math.Abs(*out.Benchmark.RelativeStrengthReturn7DPP-5.3) > 1e-12 {
		t.Fatalf("relative strength: asset=%v benchmark=%+v", out.Return7D, out.Benchmark)
	}
	if baseline.Return7D == nil || *baseline.Return7D != *out.Return7D || baseline.RSI14 == nil || out.RSI14 == nil || *baseline.RSI14 != *out.RSI14 || baseline.RSIPercentile == nil || out.RSIPercentile == nil || baseline.RSIPercentile.Value != out.RSIPercentile.Value || baseline.Volatility30D == nil || out.Volatility30D == nil || *baseline.Volatility30D != *out.Volatility30D {
		t.Fatalf("benchmark changed independent indicators: baseline=%+v output=%+v", baseline, out)
	}
	if out.Status != "complete" || len(out.Unavailable) != 0 {
		t.Fatalf("metadata: %+v", out)
	}
	if out.DataVersion == baseline.DataVersion || out.DataVersion != combinedIntelligenceDataVersion(baseline.DataVersion, benchmark.DataVersion) {
		t.Fatalf("combined data version: %q", out.DataVersion)
	}
}

func TestIntelligenceBenchmarkUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		benchmark    func(*intelligenceRepositoryStub) outbound.IntelligenceHistory
	}{
		{"not found", "benchmark_not_found", func(*intelligenceRepositoryStub) outbound.IntelligenceHistory { return outbound.IntelligenceHistory{} }},
		{"insufficient history", "insufficient_history", func(r *intelligenceRepositoryStub) outbound.IntelligenceHistory {
			return benchmarkFixture(r, "IBOV", []int{260, 261, 262, 263, 264, 265, 266})
		}},
		{"no comparable dates", "no_comparable_dates", func(r *intelligenceRepositoryStub) outbound.IntelligenceHistory {
			return benchmarkFixture(r, "IBOV", []int{250, 252, 254, 256, 258, 260, 262, 264})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := intelligenceFixture(rsiPeriod + MinimumRSIPercentileObservations + 1)
			r.benchmarks = map[string]outbound.IntelligenceHistory{"IBOV": tc.benchmark(r)}
			out, err := intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4", Benchmark: "IBOV"})
			if err != nil {
				t.Fatal(err)
			}
			if out.Benchmark == nil || out.Benchmark.Return7D != nil || out.Benchmark.RelativeStrengthReturn7DPP != nil || out.Return7D == nil || out.RSI14 == nil || out.RSIPercentile == nil {
				t.Fatalf("independent indicators or unavailable benchmark: %+v", out)
			}
			if out.Status != "partial" || !hasUnavailable(out, "benchmark.returns.return_7d", tc.reason) || !hasUnavailable(out, "benchmark.relative_strength.return_7d_pp", tc.reason) {
				t.Fatalf("unavailable: %+v", out.Unavailable)
			}
		})
	}
}

func TestIntelligenceBenchmarkHistoricalAsOf(t *testing.T) {
	r := intelligenceFixture(rsiPeriod + MinimumRSIPercentileObservations + 5)
	indexes := make([]int, len(r.history.Records))
	for i := range indexes {
		indexes[i] = i
	}
	benchmark := benchmarkFixture(r, "IBOV", indexes)
	asOfIndex := len(r.history.Records) - 4
	for i := asOfIndex + 1; i < len(benchmark.Records); i++ {
		benchmark.Records[i].Quote.ClosePriceCents = 999999
	}
	r.benchmarks = map[string]outbound.IntelligenceHistory{"IBOV": benchmark}

	out, err := intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4", Benchmark: "IBOV", AsOf: r.history.Sessions[asOfIndex]})
	if err != nil {
		t.Fatal(err)
	}
	if out.AsOf != r.history.Sessions[asOfIndex] || out.Benchmark == nil || out.Benchmark.Return7D == nil || *out.Benchmark.Return7D != 0 || out.Benchmark.RelativeStrengthReturn7DPP == nil || *out.Benchmark.RelativeStrengthReturn7DPP != 0 {
		t.Fatalf("future benchmark data leaked: %+v", out)
	}
}

func TestIntelligenceRSIPercentileWindowsAndCoverage(t *testing.T) {
	start := time.Date(2020, 8, 1, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	days := int(asOf.Sub(start).Hours()/24) + 1
	r := intelligenceFixture(days)
	for i := range r.history.Records {
		date := start.AddDate(0, 0, i)
		r.history.Records[i].Quote.TradingDate = date
		r.history.Sessions[i] = date
	}
	service := intelligenceService(r)
	service.now = func() time.Time { return time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC) }

	observations := map[int]int{}
	for _, years := range []int{1, 3, 5} {
		out, err := service.Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4", AsOf: asOf, WindowYears: years})
		if err != nil {
			t.Fatal(err)
		}
		if out.RSIPercentile == nil || out.RSIPercentile.WindowYears != years || !out.RSIPercentile.CoverageComplete {
			t.Fatalf("window %dy: %+v", years, out.RSIPercentile)
		}
		if out.RSIPercentile.CoverageTo != asOf.AddDate(0, 0, -1) {
			t.Fatalf("window %dy included asOf: %+v", years, out.RSIPercentile)
		}
		observations[years] = out.RSIPercentile.Observations
	}
	if !(observations[1] < observations[3] && observations[3] < observations[5]) {
		t.Fatalf("windows are not configurable: %+v", observations)
	}
}

func TestIntelligenceRSIPercentilePartialCoverage(t *testing.T) {
	r := intelligenceFixture(400)
	out, err := intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4", WindowYears: 5})
	if err != nil {
		t.Fatal(err)
	}
	if out.RSIPercentile == nil || out.RSIPercentile.CoverageComplete || out.RSIPercentile.Observations != 385 {
		t.Fatalf("partial coverage: %+v", out.RSIPercentile)
	}
}

func TestIntelligenceRSIPercentileMinimumObservations(t *testing.T) {
	for _, tc := range []struct {
		name      string
		records   int
		available bool
	}{
		{"exact minimum", rsiPeriod + MinimumRSIPercentileObservations + 1, true},
		{"below minimum", rsiPeriod + MinimumRSIPercentileObservations, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := intelligenceService(intelligenceFixture(tc.records)).Execute(context.Background(), inbound.GetAssetIntelligenceInput{Ticker: "PETR4", WindowYears: 1})
			if err != nil {
				t.Fatal(err)
			}
			if (out.RSIPercentile != nil) != tc.available {
				t.Fatalf("percentile availability: %+v", out.RSIPercentile)
			}
			if tc.available && out.RSIPercentile.Observations != MinimumRSIPercentileObservations {
				t.Fatalf("observations = %d", out.RSIPercentile.Observations)
			}
			if !tc.available && !hasUnavailable(out, "momentum.rsi14_percentile", "insufficient_history") {
				t.Fatalf("unavailable: %+v", out.Unavailable)
			}
		})
	}
}

func TestIntelligenceRSIPercentileHistoricalAsOfExcludesCurrent(t *testing.T) {
	r := intelligenceFixture(500)
	asOfIndex := 399
	r.history.Records[asOfIndex].Quote.ClosePriceCents = 10100
	out, err := intelligenceService(r).Execute(context.Background(), inbound.GetAssetIntelligenceInput{
		Ticker: "PETR4", AsOf: r.history.Sessions[asOfIndex], WindowYears: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.AsOf != r.history.Sessions[asOfIndex] || out.RSIPercentile == nil {
		t.Fatalf("historical percentile: %+v", out)
	}
	if out.RSIPercentile.Observations != asOfIndex-rsiPeriod || out.RSIPercentile.CoverageTo != r.history.Sessions[asOfIndex-1] {
		t.Fatalf("current or future observation included: %+v", out.RSIPercentile)
	}
	if out.RSIPercentile.Value != 100 {
		t.Fatalf("percentile = %v, want 100", out.RSIPercentile.Value)
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
	if signalByID(out, "price_above_sma20").Status != signals.StatusNotTriggered || signalByID(out, "price_below_sma20").Status != signals.StatusNotTriggered {
		t.Fatalf("signals used data after asOf: %+v", out.Signals)
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
		}{{out.Return7D, 8}, {out.SMA20Cents, 20}, {out.SMA50Cents, 50}, {out.DistanceSMA20, 20}, {out.RSI14, 15}, {out.Volatility30D, 31}, {out.DrawdownCurrent, 252}, {out.MaximumDrawdown252D, 252}} {
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
		{inbound.GetAssetIntelligenceInput{Ticker: "PETR4", Benchmark: "!"}, domain.ErrInvalidTicker},
		{inbound.GetAssetIntelligenceInput{Ticker: "PETR4", Benchmark: " petr4 "}, domain.ErrInvalidTicker},
		{inbound.GetAssetIntelligenceInput{Ticker: "PETR4", MarketType: -1}, domain.ErrInvalidMarketType},
		{inbound.GetAssetIntelligenceInput{Ticker: "PETR4", WindowYears: -1}, nil},
		{inbound.GetAssetIntelligenceInput{Ticker: "PETR4", AsOf: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)}, domain.ErrInvalidDateRange},
	} {
		r := intelligenceFixture(31)
		_, err := intelligenceService(r).Execute(context.Background(), tc.input)
		if (tc.want != nil && !errors.Is(err, tc.want)) || (tc.want == nil && err == nil) || r.calls != 0 {
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
