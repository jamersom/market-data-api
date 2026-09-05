package inbound

import (
	"context"
	"time"

	"github.com/jamersom/market-data-api/internal/domain"
)

type CompareQuotesInput struct {
	Tickers       []string
	From          time.Time
	To            time.Time
	MarketType    int
	Metrics       []domain.ComparisonMetric
	Benchmark     string
	IncludeSeries bool
}

type CompareQuotesOutput struct {
	domain.QuoteComparison
	RequestedTickers   []string
	From               time.Time
	To                 time.Time
	MarketType         int
	Metrics            []domain.ComparisonMetric
	Benchmark          *domain.AssetComparison
	PriceAdjustment    string
	Currency           string
	Source             string
	CalculationVersion string
}

type CompareQuotesUseCase interface {
	Execute(ctx context.Context, input CompareQuotesInput) (CompareQuotesOutput, error)
}
