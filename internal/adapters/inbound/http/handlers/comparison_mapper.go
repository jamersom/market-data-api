package handlers

import (
	"time"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/response"
	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

func ComparisonResponse(output inbound.CompareQuotesOutput) response.ComparisonEnvelope {
	result := response.ComparisonEnvelope{
		Data: response.Comparison{Assets: make([]response.AssetComparison, 0, len(output.Assets)), Ranking: response.ComparisonRanking{HighestReturn: output.Ranking.HighestReturnTicker, LowestVolatility: output.Ranking.LowestVolatilityTicker, LowestDrawdown: output.Ranking.LowestDrawdownTicker}},
		Meta: response.ComparisonMetadata{RequestedTickers: output.RequestedTickers, From: output.From.Format(time.DateOnly), To: output.To.Format(time.DateOnly), MarketType: output.MarketType, Metrics: make([]string, 0, len(output.Metrics)), PriceAdjustment: output.PriceAdjustment, Currency: output.Currency, Source: output.Source, CalculationVersion: output.CalculationVersion},
	}
	for _, metric := range output.Metrics {
		result.Meta.Metrics = append(result.Meta.Metrics, string(metric))
	}
	for _, asset := range output.Assets {
		result.Data.Assets = append(result.Data.Assets, assetComparisonResponse(asset))
	}
	for _, pair := range output.Correlations {
		result.Data.Correlations = append(result.Data.Correlations, response.AssetCorrelation{TickerA: pair.TickerA, TickerB: pair.TickerB, Value: pair.Value})
	}
	if output.Benchmark != nil {
		benchmark := assetComparisonResponse(*output.Benchmark)
		result.Data.Benchmark = &benchmark
	}
	return result
}

func assetComparisonResponse(asset domain.AssetComparison) response.AssetComparison {
	result := response.AssetComparison{
		Ticker:           asset.Ticker,
		Status:           string(asset.Status),
		AvailableFrom:    comparisonDateResponse(asset.AvailableFrom),
		AvailableTo:      comparisonDateResponse(asset.AvailableTo),
		InitialPrice:     comparisonMoney(asset.InitialPriceCents),
		FinalPrice:       comparisonMoney(asset.FinalPriceCents),
		AbsoluteReturn:   comparisonMoney(asset.AbsoluteReturnCents),
		PercentageReturn: comparisonPercentage(asset.PercentageReturn), AnnualizedVolatility: comparisonPercentage(asset.AnnualizedVolatility), MaximumDrawdown: comparisonPercentage(asset.MaximumDrawdown),
		AverageDailyVolume: comparisonMoney(asset.AverageDailyVolumeCents), LowestPrice: comparisonMoney(asset.LowestPriceCents), HighestPrice: comparisonMoney(asset.HighestPriceCents),
		BestDay: comparisonDay(asset.BestDay), WorstDay: comparisonDay(asset.WorstDay), Message: asset.InsufficientDataExplanation,
	}
	for _, point := range asset.Series {
		result.Series = append(result.Series, response.ComparisonPoint{Date: point.Date.Format(time.DateOnly), ClosePrice: formatCents(point.ClosePriceCents), NormalizedPerformance: round(point.NormalizedPerformance)})
	}
	return result
}

func comparisonMoney(value *int64) *string {
	if value == nil {
		return nil
	}
	formatted := formatCents(*value)
	return &formatted
}
func comparisonPercentage(value *float64) *float64 {
	if value == nil {
		return nil
	}
	rounded := round(*value)
	return &rounded
}
func comparisonDateResponse(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.DateOnly)
	return &formatted
}
func comparisonDay(value *domain.DailyPerformance) *response.DailyPerformance {
	if value == nil {
		return nil
	}
	return &response.DailyPerformance{Date: value.Date.Format(time.DateOnly), Return: round(value.ReturnPercentage)}
}
