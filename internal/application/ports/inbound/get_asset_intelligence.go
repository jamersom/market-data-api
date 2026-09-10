package inbound

import (
	"context"
	"time"

	"github.com/jamersom/market-data-api/internal/domain"
)

type GetAssetIntelligenceInput struct {
	Ticker      string
	Benchmark   string
	AsOf        time.Time // Zero means the latest available quote, bounded by today.
	MarketType  int
	WindowYears int
}

type IntelligenceUnavailable struct {
	Field  string
	Reason string
}

type RSIPercentile struct {
	Value            float64
	WindowYears      int
	Observations     int
	CoverageFrom     time.Time
	CoverageTo       time.Time
	CoverageComplete bool
}

type IntelligenceBenchmark struct {
	Ticker                     string
	Return7D                   *float64
	RelativeStrengthReturn7DPP *float64
}

// GetAssetIntelligenceOutput is an application result, not the HTTP DTO.
// Optional metrics distinguish legitimate zeros from unavailable calculations.
type GetAssetIntelligenceOutput struct {
	Calendar                   domain.IntelligenceCalendarMetadata
	Ticker                     string
	Benchmark                  *IntelligenceBenchmark
	MarketType                 int
	RequestedAsOf              *time.Time
	AsOf                       time.Time
	Price                      domain.Quote
	Return7D                   *float64
	SMA20Cents                 *float64
	DistanceSMA20              *float64
	RSI14                      *float64
	RSIPercentile              *RSIPercentile
	Volatility30D              *float64
	DrawdownCurrent            *float64
	MaximumDrawdown252D        *float64
	AverageDailyVolume20DCents *int64
	Status                     string
	Source                     string
	PriceAdjustment            string
	WindowUnit                 string
	PercentageUnit             string
	CalculationVersion         string
	DataVersion                string
	RSISeedFrom                time.Time
	Unavailable                []IntelligenceUnavailable
}

type GetAssetIntelligenceUseCase interface {
	Execute(ctx context.Context, input GetAssetIntelligenceInput) (GetAssetIntelligenceOutput, error)
}
