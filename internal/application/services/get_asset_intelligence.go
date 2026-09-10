package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"time"
	_ "time/tzdata"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/domain"
	"github.com/jamersom/market-data-api/internal/domain/analytics"
)

type GetAssetIntelligenceService struct {
	quotes outbound.IntelligenceQuoteRepository
	logger *slog.Logger
	now    func() time.Time
}

const (
	DefaultRSIPercentileWindowYears  = 3
	MinimumRSIPercentileObservations = 252
	rsiPeriod                        = 14
	return7DPeriods                  = 7
)

var _ inbound.GetAssetIntelligenceUseCase = (*GetAssetIntelligenceService)(nil)

func NewGetAssetIntelligenceService(quotes outbound.IntelligenceQuoteRepository, logger *slog.Logger) *GetAssetIntelligenceService {
	if logger == nil {
		logger = slog.Default()
	}
	return &GetAssetIntelligenceService{quotes: quotes, logger: logger, now: time.Now}
}

func (s *GetAssetIntelligenceService) Execute(ctx context.Context, input inbound.GetAssetIntelligenceInput) (inbound.GetAssetIntelligenceOutput, error) {
	var output inbound.GetAssetIntelligenceOutput
	if err := ctx.Err(); err != nil {
		return output, err
	}
	ticker, err := domain.NormalizeTicker(input.Ticker)
	if err != nil {
		return output, err
	}
	benchmark := ""
	if input.Benchmark != "" {
		benchmark, err = domain.NormalizeTicker(input.Benchmark)
		if err != nil {
			return output, domain.ValidationError{Field: "benchmark", Value: input.Benchmark, Message: "benchmark must be a valid ticker", Err: domain.ErrInvalidTicker}
		}
		if benchmark == ticker {
			return output, domain.ValidationError{Field: "benchmark", Value: benchmark, Message: "benchmark must differ from ticker", Err: domain.ErrInvalidTicker}
		}
	}
	market := input.MarketType
	if market == 0 {
		market = domain.DefaultMarketType
	}
	if market < 0 {
		return output, domain.ValidationError{Field: "marketType", Message: "marketType must be positive", Err: domain.ErrInvalidMarketType}
	}
	windowYears := input.WindowYears
	if windowYears == 0 {
		windowYears = DefaultRSIPercentileWindowYears
	}
	if windowYears < 0 {
		return output, domain.ValidationError{Field: "rsiWindow", Message: "rsiWindow must be a positive number of years"}
	}
	location, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return output, fmt.Errorf("load market timezone: %w", err)
	}
	today := comparisonDate(s.now().In(location))
	cutoff := today
	if !input.AsOf.IsZero() {
		cutoff = comparisonDate(input.AsOf)
		if cutoff.After(today) {
			return output, domain.ValidationError{Field: "asOf", Message: "asOf must not be in the future", Err: domain.ErrInvalidDateRange}
		}
		output.RequestedAsOf = &cutoff
	}
	started := time.Now()
	tickers := []string{ticker}
	if benchmark != "" {
		tickers = append(tickers, benchmark)
	}
	histories, err := s.quotes.FindIntelligenceHistories(ctx, tickers, market, cutoff)
	if err != nil {
		return output, fmt.Errorf("get intelligence history %s: %w", ticker, err)
	}
	history := histories[ticker]
	if err := ctx.Err(); err != nil {
		return output, err
	}
	quotes := make([]domain.Quote, 0, len(history.Records))
	for _, record := range history.Records {
		quote := record.Quote
		if quote.TradingDate.IsZero() {
			return output, fmt.Errorf("invalid intelligence history: missing trading date")
		}
		quote.TradingDate = comparisonDate(quote.TradingDate)
		if quote.TradingDate.After(cutoff) {
			continue
		}
		if quote.Ticker != ticker || quote.MarketType != market || (quote.Currency != "" && quote.Currency != "BRL") {
			return output, fmt.Errorf("invalid intelligence history: ticker, market or currency mismatch")
		}
		quotes = append(quotes, quote)
	}
	if len(quotes) == 0 {
		return output, domain.ResourceNotFoundError{Resource: "quote", Ticker: ticker}
	}
	sort.Slice(quotes, func(i, j int) bool { return quotes[i].TradingDate.Before(quotes[j].TradingDate) })
	for i := 1; i < len(quotes); i++ {
		if quotes[i].TradingDate.Equal(quotes[i-1].TradingDate) {
			return output, fmt.Errorf("invalid intelligence history: duplicate trading date")
		}
	}
	output.Ticker, output.MarketType = ticker, market
	output.Price = quotes[len(quotes)-1]
	output.AsOf = output.Price.TradingDate
	output.Source, output.PriceAdjustment = "B3 COTAHIST", "unadjusted"
	output.WindowUnit, output.PercentageUnit = "trading_sessions", "percent"
	output.CalculationVersion, output.DataVersion = "1.2", history.DataVersion
	output.RSISeedFrom = quotes[0].TradingDate
	output.Calendar = history.Calendar
	output.Calendar.OfficialVerified = history.CalendarVerified
	output.Status = "complete"
	missing := func(field, reason string) {
		output.Unavailable = append(output.Unavailable, inbound.IntelligenceUnavailable{Field: field, Reason: reason})
	}
	calendar := make(map[time.Time]int, len(history.Sessions))
	sessions := append([]time.Time(nil), history.Sessions...)
	for i := range sessions {
		sessions[i] = comparisonDate(sessions[i])
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].Before(sessions[j]) })
	observed := history.Calendar.Source == "cotahist_observed" && history.Calendar.Policy == "observed_import_integrity_v1"
	calendarOK := (history.CalendarVerified || observed) && len(sessions) > 0
	for i, date := range sessions {
		if date.IsZero() {
			calendarOK = false
		}
		if _, exists := calendar[date]; exists {
			calendarOK = false
		}
		calendar[date] = i
	}
	window := func(count int, volume bool) ([]int64, string) {
		if len(quotes) < count {
			return nil, "insufficient_history"
		}
		if !calendarOK {
			return nil, "calendar_unavailable"
		}
		selected := quotes[len(quotes)-count:]
		if observed {
			if reason := observedCoverageReason(history.Calendar.Coverage, selected[0].TradingDate, selected[len(selected)-1].TradingDate); reason != "" {
				return nil, reason
			}
		}
		values := make([]int64, count)
		previous := -1
		for i, quote := range selected {
			index, exists := calendar[quote.TradingDate]
			if !exists || (i > 0 && index != previous+1) {
				return nil, "missing_sessions"
			}
			previous = index
			if volume {
				if quote.TradedVolumeCents < 0 {
					return nil, "invalid_data"
				}
				values[i] = quote.TradedVolumeCents
			} else {
				if quote.ClosePriceCents <= 0 {
					return nil, "invalid_data"
				}
				values[i] = quote.ClosePriceCents
			}
		}
		return values, ""
	}
	calculate := func(field string, count int, fn func([]int64) float64) *float64 {
		values, reason := window(count, false)
		if reason != "" {
			missing(field, reason)
			return nil
		}
		value := fn(values)
		return &value
	}
	output.Return7D = calculate("returns.return_7d", 8, func(v []int64) float64 { _, p := analytics.Return(v[0], v[7]); return p })
	output.SMA20Cents = calculate("trend.sma20", 20, func(v []int64) float64 { p, _ := analytics.SMA(v, 20); return p })
	output.DistanceSMA20 = calculate("trend.distance_sma20", 20, func(v []int64) float64 { p, _ := analytics.DistanceFromSMA(v, 20); return p })
	if len(quotes) < rsiPeriod+1 {
		missing("momentum.rsi14", "insufficient_history")
		missing("momentum.rsi14_percentile", "insufficient_history")
	} else if closes, reason := window(len(quotes), false); reason != "" {
		missing("momentum.rsi14", reason)
		missing("momentum.rsi14_percentile", reason)
	} else {
		rsiHistory, _ := analytics.RSIHistory(closes, rsiPeriod)
		currentRSI := rsiHistory[len(rsiHistory)-1]
		output.RSI14 = &currentRSI
		windowStart := output.AsOf.AddDate(-windowYears, 0, 0)
		values := make([]float64, 0, len(rsiHistory)-1)
		var coverageFrom, coverageTo time.Time
		for i, value := range rsiHistory[:len(rsiHistory)-1] {
			date := quotes[rsiPeriod+i].TradingDate
			if date.Before(windowStart) {
				continue
			}
			if coverageFrom.IsZero() {
				coverageFrom = date
			}
			coverageTo = date
			values = append(values, value)
		}
		if len(values) < MinimumRSIPercentileObservations {
			missing("momentum.rsi14_percentile", "insufficient_history")
		} else if value, ok := analytics.PercentileMidrank(*output.RSI14, values); ok {
			firstValidRSIDate := quotes[rsiPeriod].TradingDate
			output.RSIPercentile = &inbound.RSIPercentile{
				Value:            value,
				WindowYears:      windowYears,
				Observations:     len(values),
				CoverageFrom:     coverageFrom,
				CoverageTo:       coverageTo,
				CoverageComplete: !firstValidRSIDate.After(windowStart),
			}
		} else {
			missing("momentum.rsi14_percentile", "invalid_data")
		}
	}
	output.Volatility30D = calculate("risk.volatility_30d", 31, func(v []int64) float64 {
		returns := make([]float64, 30)
		for i := range returns {
			returns[i] = float64(v[i+1])/float64(v[i]) - 1
		}
		p, _ := analytics.AnnualizedVolatility(returns)
		return p
	})
	output.DrawdownCurrent = calculate("risk.drawdown_current", 252, func(v []int64) float64 { p, _ := analytics.CurrentDrawdown(v, 252); return p })
	output.MaximumDrawdown252D = calculate("risk.maximum_drawdown_252d", 252, func(v []int64) float64 { p, _ := analytics.MaximumDrawdown(v); return p })
	if volumes, reason := window(20, true); reason != "" {
		missing("liquidity.average_daily_volume_20d", reason)
	} else {
		value, _ := analytics.AverageVolume(volumes)
		output.AverageDailyVolume20DCents = &value
	}
	if output.Price.ClosePriceCents <= 0 {
		missing("price.close", "invalid_data")
	}
	if benchmark != "" {
		output.Benchmark = &inbound.IntelligenceBenchmark{Ticker: benchmark}
		benchmarkHistory := histories[benchmark]
		output.DataVersion = combinedIntelligenceDataVersion(history.DataVersion, benchmarkHistory.DataVersion)
		reason := ""
		if len(benchmarkHistory.Records) == 0 {
			reason = "benchmark_not_found"
		} else if !calendarOK {
			reason = "calendar_unavailable"
		} else {
			benchmarkQuotes, normalizeErr := normalizeIntelligenceQuotes(benchmarkHistory.Records, benchmark, market, cutoff)
			if normalizeErr != nil {
				reason = "invalid_data"
			} else {
				assetReturn, benchmarkReturn, from, to, comparableReason := comparableReturns(quotes, benchmarkQuotes, sessions, return7DPeriods)
				reason = comparableReason
				if reason == "" && observed {
					reason = observedCoverageReason(history.Calendar.Coverage, from, to)
				}
				if reason == "" {
					output.Benchmark.Return7D = &benchmarkReturn
					relativeStrength := assetReturn - benchmarkReturn
					output.Benchmark.RelativeStrengthReturn7DPP = &relativeStrength
				}
			}
		}
		if reason != "" {
			missing("benchmark.returns.return_7d", reason)
			missing("benchmark.relative_strength.return_7d_pp", reason)
		}
	}
	if len(output.Unavailable) > 0 {
		output.Status = "partial"
	}
	if err := ctx.Err(); err != nil {
		return inbound.GetAssetIntelligenceOutput{}, err
	}
	s.logger.DebugContext(ctx, "asset intelligence calculated", "ticker", ticker, "records", len(quotes), "as_of", output.AsOf, "calculation_version", output.CalculationVersion, "duration", time.Since(started))
	return output, nil
}

func combinedIntelligenceDataVersion(assetVersion, benchmarkVersion string) string {
	hash := sha256.New()
	hash.Write([]byte(assetVersion))
	hash.Write([]byte{0})
	hash.Write([]byte(benchmarkVersion))
	return "sha256-v1:" + hex.EncodeToString(hash.Sum(nil))
}

func normalizeIntelligenceQuotes(records []domain.QuoteRecord, ticker string, market int, cutoff time.Time) ([]domain.Quote, error) {
	quotes := make([]domain.Quote, 0, len(records))
	for _, record := range records {
		quote := record.Quote
		if quote.TradingDate.IsZero() {
			return nil, fmt.Errorf("missing trading date")
		}
		quote.TradingDate = comparisonDate(quote.TradingDate)
		if quote.TradingDate.After(cutoff) {
			continue
		}
		if quote.Ticker != ticker || quote.MarketType != market || (quote.Currency != "" && quote.Currency != "BRL") {
			return nil, fmt.Errorf("ticker, market or currency mismatch")
		}
		quotes = append(quotes, quote)
	}
	sort.Slice(quotes, func(i, j int) bool { return quotes[i].TradingDate.Before(quotes[j].TradingDate) })
	for i := range quotes {
		if quotes[i].ClosePriceCents <= 0 || (i > 0 && quotes[i].TradingDate.Equal(quotes[i-1].TradingDate)) {
			return nil, fmt.Errorf("duplicate trading date or invalid closing price")
		}
	}
	return quotes, nil
}

func comparableReturns(asset, benchmark []domain.Quote, sessions []time.Time, periods int) (float64, float64, time.Time, time.Time, string) {
	if len(asset) < periods+1 || len(benchmark) < periods+1 {
		return 0, 0, time.Time{}, time.Time{}, "insufficient_history"
	}
	assetWindow := asset[len(asset)-(periods+1):]
	sessionIndexes := make(map[time.Time]int, len(sessions))
	for i, session := range sessions {
		sessionIndexes[comparisonDate(session)] = i
	}
	benchmarkPrices := make(map[time.Time]int64, len(benchmark))
	for _, quote := range benchmark {
		benchmarkPrices[quote.TradingDate] = quote.ClosePriceCents
	}
	previous := -1
	for _, quote := range assetWindow {
		index, sessionExists := sessionIndexes[quote.TradingDate]
		benchmarkPrice, benchmarkExists := benchmarkPrices[quote.TradingDate]
		if !sessionExists || !benchmarkExists || benchmarkPrice <= 0 || (previous >= 0 && index != previous+1) {
			return 0, 0, time.Time{}, time.Time{}, "no_comparable_dates"
		}
		previous = index
	}
	from, to := assetWindow[0].TradingDate, assetWindow[len(assetWindow)-1].TradingDate
	_, assetReturn := analytics.Return(assetWindow[0].ClosePriceCents, assetWindow[len(assetWindow)-1].ClosePriceCents)
	_, benchmarkReturn := analytics.Return(benchmarkPrices[from], benchmarkPrices[to])
	return assetReturn, benchmarkReturn, from, to, ""
}

// Each involved year must have a valid published import. Intermediate years
// use observed sessions, not presumed holidays. Whole-market omissions in a
// source file cannot be independently detected by this policy.
func observedCoverageReason(coverage []domain.CalendarCoverage, from, to time.Time) string {
	for year := from.Year(); year <= to.Year(); year++ {
		var found *domain.CalendarCoverage
		for i := range coverage {
			if coverage[i].Year == year {
				if found != nil {
					return "calendar_invalid_coverage"
				}
				found = &coverage[i]
			}
		}
		if found == nil {
			return "calendar_missing_year"
		}
		if !found.IntegrityValidated || found.From.IsZero() || found.To.IsZero() || found.From.Year() != year || found.To.Year() != year || found.From.After(found.To) {
			return "calendar_invalid_coverage"
		}
		if (year == from.Year() && from.Before(found.From)) || (year == to.Year() && to.After(found.To)) {
			return "calendar_outside_coverage"
		}
	}
	return ""
}
