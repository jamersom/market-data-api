package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type CompareQuotesService struct {
	quotes outbound.ComparisonQuoteRepository
	logger *slog.Logger
}

var _ inbound.CompareQuotesUseCase = (*CompareQuotesService)(nil)

func NewCompareQuotesService(quotes outbound.ComparisonQuoteRepository, logger *slog.Logger) *CompareQuotesService {
	if logger == nil {
		logger = slog.Default()
	}
	return &CompareQuotesService{quotes: quotes, logger: logger}
}

func (s *CompareQuotesService) Execute(ctx context.Context, input inbound.CompareQuotesInput) (inbound.CompareQuotesOutput, error) {
	if err := ctx.Err(); err != nil {
		return inbound.CompareQuotesOutput{}, err
	}
	input, err := normalizeComparisonInput(input)
	if err != nil {
		return inbound.CompareQuotesOutput{}, err
	}
	tickers := append([]string(nil), input.Tickers...)
	if input.Benchmark != "" {
		found := false
		for _, ticker := range tickers {
			if ticker == input.Benchmark {
				found = true
			}
		}
		if !found {
			tickers = append(tickers, input.Benchmark)
		}
	}
	started := time.Now()
	records, err := s.quotes.FindByTickersAndPeriod(ctx, tickers, input.From, input.To, input.MarketType)
	if err != nil {
		return inbound.CompareQuotesOutput{}, fmt.Errorf("get quotes for comparison: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return inbound.CompareQuotesOutput{}, err
	}
	metrics := make(map[domain.ComparisonMetric]bool, len(input.Metrics))
	for _, metric := range input.Metrics {
		metrics[metric] = true
	}
	output := inbound.CompareQuotesOutput{
		QuoteComparison:  domain.QuoteComparison{Assets: make([]domain.AssetComparison, 0, len(input.Tickers))},
		RequestedTickers: input.Tickers, From: input.From, To: input.To,
		MarketType: input.MarketType, Metrics: input.Metrics,
		PriceAdjustment: "unadjusted", Currency: "BRL", Source: "B3 COTAHIST", CalculationVersion: "1.0",
	}
	returns := make(map[string]map[returnInterval]float64, len(input.Tickers))
	available := 0
	for _, ticker := range input.Tickers {
		if err := ctx.Err(); err != nil {
			return inbound.CompareQuotesOutput{}, err
		}
		asset, daily := compareAsset(ticker, records[ticker], metrics, input.IncludeSeries)
		output.Assets = append(output.Assets, asset)
		if asset.Status == domain.ComparisonAssetAvailable {
			available++
			returns[ticker] = daily
		}
	}
	if available < domain.MinComparisonTickers {
		return inbound.CompareQuotesOutput{}, domain.ErrInsufficientComparisonData
	}
	output.Ranking = comparisonRanking(output.Assets)
	if metrics[domain.ComparisonMetricCorrelation] {
		output.Correlations = compareCorrelations(input.Tickers, returns)
	}
	if input.Benchmark != "" {
		benchmark, _ := compareAsset(input.Benchmark, records[input.Benchmark], metrics, input.IncludeSeries)
		output.Benchmark = &benchmark
	}
	s.logger.DebugContext(ctx, "quotes compared", "asset_count", len(input.Tickers), "available_assets", available, "metric_count", len(input.Metrics), "duration", time.Since(started))
	return output, nil
}

func normalizeComparisonInput(input inbound.CompareQuotesInput) (inbound.CompareQuotesInput, error) {
	tickers, err := domain.NormalizeComparisonTickers(input.Tickers)
	if err != nil {
		return input, err
	}
	metrics, err := domain.NormalizeComparisonMetrics(input.Metrics)
	if err != nil {
		return input, err
	}
	for _, field := range []struct {
		name string
		date time.Time
	}{{"from", input.From}, {"to", input.To}} {
		if field.date.IsZero() {
			return input, domain.ValidationError{Field: field.name, Message: field.name + " is required", Err: domain.ErrInvalidDateRange}
		}
	}
	input.From, input.To = comparisonDate(input.From), comparisonDate(input.To)
	if input.From.After(input.To) || input.To.After(input.From.AddDate(5, 0, 0)) {
		return input, domain.ValidationError{Field: "dateRange", Value: formatRange(input.From, input.To), Message: "from must not exceed to and date range cannot exceed five years", Err: domain.ErrInvalidDateRange}
	}
	if input.MarketType < 0 {
		return input, domain.ValidationError{Field: "marketType", Message: "marketType must be positive", Err: domain.ErrInvalidMarketType}
	}
	if input.MarketType == 0 {
		input.MarketType = domain.DefaultMarketType
	}
	if strings.TrimSpace(input.Benchmark) != "" {
		benchmark, err := domain.NormalizeTicker(input.Benchmark)
		if err != nil {
			return input, domain.ValidationError{Field: "benchmark", Value: input.Benchmark, Message: "benchmark must be a valid ticker", Err: domain.ErrInvalidTicker}
		}
		input.Benchmark = benchmark
	} else {
		input.Benchmark = ""
	}
	input.Tickers, input.Metrics = tickers, metrics
	return input, nil
}

func comparisonDate(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
}
