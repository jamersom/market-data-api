package services

import (
	"context"
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
	market := input.MarketType
	if market == 0 {
		market = domain.DefaultMarketType
	}
	if market < 0 {
		return output, domain.ValidationError{Field: "marketType", Message: "marketType must be positive", Err: domain.ErrInvalidMarketType}
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
	history, err := s.quotes.FindIntelligenceHistory(ctx, ticker, market, cutoff)
	if err != nil {
		return output, fmt.Errorf("get intelligence history %s: %w", ticker, err)
	}
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
	output.CalculationVersion, output.DataVersion = "1.0", history.DataVersion
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
	if len(quotes) < 15 {
		missing("momentum.rsi14", "insufficient_history")
	} else {
		output.RSI14 = calculate("momentum.rsi14", len(quotes), func(v []int64) float64 { p, _ := analytics.RSI(v, 14); return p })
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
	if len(output.Unavailable) > 0 {
		output.Status = "partial"
	}
	if err := ctx.Err(); err != nil {
		return inbound.GetAssetIntelligenceOutput{}, err
	}
	s.logger.DebugContext(ctx, "asset intelligence calculated", "ticker", ticker, "records", len(quotes), "as_of", output.AsOf, "calculation_version", output.CalculationVersion, "duration", time.Since(started))
	return output, nil
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
