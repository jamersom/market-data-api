package services

import (
	"math"
	"math/big"
	"sort"
	"time"

	"github.com/jamersom/market-data-api/internal/domain"
)

// A correlação alinha as datas inicial e final de cada intervalo de retorno,
// evitando que lacunas no histórico associem um retorno de vários pregões
// a um retorno de um único pregão.
type returnInterval struct{ from, to time.Time }

func compareAsset(ticker string, records []domain.QuoteRecord, metrics map[domain.ComparisonMetric]bool, includeSeries bool) (domain.AssetComparison, map[returnInterval]float64) {
	asset := domain.AssetComparison{Ticker: ticker, Status: domain.ComparisonAssetInsufficientData}
	if len(records) == 0 {
		asset.InsufficientDataExplanation = "No quotes were found for the requested period"
		return asset, nil
	}
	// Copia antes de ordenar: os dados do repositório podem ser compartilhados com outros consumidores.
	quotes := make([]domain.Quote, len(records))
	for i, record := range records {
		quotes[i] = record.Quote
		quotes[i].TradingDate = comparisonDate(record.Quote.TradingDate)
	}
	sort.Slice(quotes, func(i, j int) bool { return quotes[i].TradingDate.Before(quotes[j].TradingDate) })
	first, last := quotes[0], quotes[len(quotes)-1]
	asset.AvailableFrom, asset.AvailableTo = &first.TradingDate, &last.TradingDate
	if len(quotes) < 2 {
		asset.InsufficientDataExplanation = "At least two daily quotes are required"
		return asset, nil
	}
	for i, quote := range quotes {
		if quote.ClosePriceCents <= 0 || quote.TradingDate.IsZero() || (quote.Currency != "" && quote.Currency != "BRL") || (i > 0 && quote.TradingDate.Equal(quotes[i-1].TradingDate)) {
			asset.InsufficientDataExplanation = "Quotes must have unique dates, positive closing prices and BRL currency"
			return asset, nil
		}
	}
	asset.Status = domain.ComparisonAssetAvailable
	asset.InitialPriceCents, asset.FinalPriceCents = &first.ClosePriceCents, &last.ClosePriceCents
	daily := make(map[returnInterval]float64, len(quotes)-1)
	values := make([]float64, 0, len(quotes)-1)
	peak, drawdown := first.ClosePriceCents, 0.0
	low, high := first.LowPriceCents, first.HighPriceCents
	volume := new(big.Int)
	var best, worst *domain.DailyPerformance
	if includeSeries {
		asset.Series = make([]domain.ComparisonPoint, 0, len(quotes))
	}
	for i, quote := range quotes {
		volume.Add(volume, big.NewInt(quote.TradedVolumeCents))
		if quote.LowPriceCents < low {
			low = quote.LowPriceCents
		}
		if quote.HighPriceCents > high {
			high = quote.HighPriceCents
		}
		if quote.ClosePriceCents > peak {
			peak = quote.ClosePriceCents
		}
		dd := (float64(quote.ClosePriceCents)/float64(peak) - 1) * 100
		if dd < drawdown {
			drawdown = dd
		}
		if includeSeries {
			asset.Series = append(asset.Series, domain.ComparisonPoint{Date: quote.TradingDate, ClosePriceCents: quote.ClosePriceCents, NormalizedPerformance: float64(quote.ClosePriceCents) / float64(first.ClosePriceCents) * 100})
		}
		if i == 0 {
			continue
		}
		value := float64(quote.ClosePriceCents)/float64(quotes[i-1].ClosePriceCents) - 1
		values = append(values, value)
		daily[returnInterval{quotes[i-1].TradingDate, quote.TradingDate}] = value
		day := &domain.DailyPerformance{Date: quote.TradingDate, ReturnPercentage: value * 100}
		if best == nil || day.ReturnPercentage > best.ReturnPercentage {
			best = day
		}
		if worst == nil || day.ReturnPercentage < worst.ReturnPercentage {
			worst = day
		}
	}
	if metrics[domain.ComparisonMetricReturn] {
		absolute := last.ClosePriceCents - first.ClosePriceCents
		percentage := (float64(last.ClosePriceCents)/float64(first.ClosePriceCents) - 1) * 100
		asset.AbsoluteReturnCents, asset.PercentageReturn = &absolute, &percentage
	}
	if metrics[domain.ComparisonMetricVolatility] && len(values) >= 2 {
		_, sumSquares := moments(values)
		volatility := math.Sqrt(sumSquares/float64(len(values)-1)) * math.Sqrt(252) * 100
		asset.AnnualizedVolatility = &volatility
	}
	if metrics[domain.ComparisonMetricDrawdown] {
		asset.MaximumDrawdown = &drawdown
	}
	if metrics[domain.ComparisonMetricAverageVolume] {
		average := volume.Quo(volume, big.NewInt(int64(len(quotes)))).Int64()
		asset.AverageDailyVolumeCents = &average
	}
	if metrics[domain.ComparisonMetricHighLow] {
		asset.LowestPriceCents, asset.HighestPriceCents = &low, &high
	}
	if metrics[domain.ComparisonMetricBestWorstDay] {
		asset.BestDay, asset.WorstDay = best, worst
	}
	return asset, daily
}

func moments(values []float64) (mean, sumSquares float64) {
	for i, value := range values {
		delta := value - mean
		mean += delta / float64(i+1)
		sumSquares += delta * (value - mean)
	}
	return mean, sumSquares
}

func comparisonRanking(assets []domain.AssetComparison) domain.ComparisonRanking {
	var ranking domain.ComparisonRanking
	var highestReturn, lowestVolatility, lowestDrawdown *float64
	for _, asset := range assets {
		if asset.Status != domain.ComparisonAssetAvailable {
			continue
		}
		if value := asset.PercentageReturn; value != nil && (highestReturn == nil || *value > *highestReturn) {
			highestReturn = value
			ranking.HighestReturnTicker = asset.Ticker
		}
		if value := asset.AnnualizedVolatility; value != nil && (lowestVolatility == nil || *value < *lowestVolatility) {
			lowestVolatility = value
			ranking.LowestVolatilityTicker = asset.Ticker
		}
		// Drawdowns são negativos: o valor mais próximo de zero representa a menor perda.
		if value := asset.MaximumDrawdown; value != nil && (lowestDrawdown == nil || *value > *lowestDrawdown) {
			lowestDrawdown = value
			ranking.LowestDrawdownTicker = asset.Ticker
		}
	}
	return ranking
}

func compareCorrelations(tickers []string, returns map[string]map[returnInterval]float64) []domain.AssetCorrelation {
	result := make([]domain.AssetCorrelation, 0)
	for i, tickerA := range tickers {
		for _, tickerB := range tickers[i+1:] {
			intervals := make([]returnInterval, 0)
			for interval := range returns[tickerA] {
				if _, ok := returns[tickerB][interval]; ok {
					intervals = append(intervals, interval)
				}
			}
			sort.Slice(intervals, func(i, j int) bool { return intervals[i].to.Before(intervals[j].to) })
			if len(intervals) < 2 {
				continue
			}
			x, y := make([]float64, len(intervals)), make([]float64, len(intervals))
			for i, interval := range intervals {
				x[i], y[i] = returns[tickerA][interval], returns[tickerB][interval]
			}
			meanX, squaresX := moments(x)
			meanY, squaresY := moments(y)
			if squaresX <= 0 || squaresY <= 0 {
				continue
			}
			covariance := 0.0
			for i := range x {
				covariance += (x[i] - meanX) * (y[i] - meanY)
			}
			value := math.Max(-1, math.Min(1, covariance/math.Sqrt(squaresX*squaresY)))
			result = append(result, domain.AssetCorrelation{TickerA: tickerA, TickerB: tickerB, Value: value})
		}
	}
	return result
}
