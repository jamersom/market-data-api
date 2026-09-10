// Package analytics provides pure quantitative calculations, independent of
// transport, persistence and comparison response types.
package analytics

import (
	"math"
	"math/big"
)

// Return calculates the absolute change in cents and the percentage change.
// The caller must supply positive, validated closing prices.
func Return(initial, final int64) (absolute int64, percentage float64) {
	return final - initial, (float64(final)/float64(initial) - 1) * 100
}

// AnnualizedVolatility uses sample variance and 252 trading sessions per year.
// Returns must be finite simple returns (0.01 means 1%), in chronological order.
// The result is a percentage. Fewer than two returns produces ok=false.
func AnnualizedVolatility(returns []float64) (value float64, ok bool) {
	if len(returns) < 2 {
		return 0, false
	}
	var mean, sumSquares float64
	for i, value := range returns {
		delta := value - mean
		mean += delta / float64(i+1)
		sumSquares += delta * (value - mean)
	}
	return math.Sqrt(sumSquares/float64(len(returns)-1)) * math.Sqrt(252) * 100, true
}

// MaximumDrawdown returns the lowest percentage change from the running peak.
// Closes must be positive, validated prices in chronological order.
// An empty series produces ok=false; a single close has zero drawdown.
func MaximumDrawdown(closes []int64) (value float64, ok bool) {
	if len(closes) == 0 {
		return 0, false
	}
	peak := closes[0]
	for _, close := range closes {
		if close > peak {
			peak = close
		}
		drawdown := (float64(close)/float64(peak) - 1) * 100
		if drawdown < value {
			value = drawdown
		}
	}
	return value, true
}

// AverageVolume calculates the mean monetary volume in cents without sum
// overflow. Fractional cents are truncated toward zero. Empty input is absent.
func AverageVolume(volumes []int64) (value int64, ok bool) {
	if len(volumes) == 0 {
		return 0, false
	}
	sum := new(big.Int)
	for _, volume := range volumes {
		sum.Add(sum, big.NewInt(volume))
	}
	return sum.Quo(sum, big.NewInt(int64(len(volumes)))).Int64(), true
}
