package analytics

import (
	"math"
	"math/big"
)

// SMA returns the mean of the last period closes in cents, retaining fractional
// cents. Input is chronological. Invalid periods, insufficient history or
// nonpositive prices in the selected window produce ok=false.
func SMA(closes []int64, period int) (value float64, ok bool) {
	window, ok := closingWindow(closes, period)
	if !ok {
		return 0, false
	}
	sum := new(big.Int)
	for _, close := range window {
		sum.Add(sum, big.NewInt(close))
	}
	mean := new(big.Rat).SetFrac(sum, big.NewInt(int64(period)))
	value, _ = mean.Float64()
	return value, true
}

// DistanceFromSMA returns the last close's percentage distance from its SMA.
// It uses the unrounded mean and the same window validation as SMA.
func DistanceFromSMA(closes []int64, period int) (value float64, ok bool) {
	mean, ok := SMA(closes, period)
	if !ok {
		return 0, false
	}
	return (float64(closes[len(closes)-1])/mean - 1) * 100, true
}

// RSI returns the final Wilder RSI (0..100). It seeds average gain and loss
// from the first period changes, then smooths every remaining change.
// All supplied closes must be positive and chronological. At least period+1
// closes are required. The caller must choose a stable history start: changing
// the seed history can change the result even with identical recent closes.
// Flat series return 50, only gains 100, and only losses 0.
func RSI(closes []int64, period int) (value float64, ok bool) {
	values, ok := RSIHistory(closes, period)
	if !ok {
		return 0, false
	}
	return values[len(values)-1], true
}

// RSIHistory returns every valid Wilder RSI using one stable seed. The first
// result corresponds to closes[period], and the final result is equal to RSI.
func RSIHistory(closes []int64, period int) (values []float64, ok bool) {
	if period <= 0 || period >= len(closes) {
		return nil, false
	}
	for _, close := range closes {
		if close <= 0 {
			return nil, false
		}
	}
	values = make([]float64, 0, len(closes)-period)
	var gain, loss float64
	for i := 1; i < len(closes); i++ {
		// Positive int64 prices have a representable int64 difference. Subtract
		// before conversion so small changes in large prices are preserved.
		change := float64(closes[i] - closes[i-1])
		up, down := 0.0, 0.0
		if change > 0 {
			up = change
		} else {
			down = -change
		}
		if i <= period {
			gain += up
			loss += down
			if i == period {
				gain /= float64(period)
				loss /= float64(period)
			}
		} else {
			gain = (gain*float64(period-1) + up) / float64(period)
			loss = (loss*float64(period-1) + down) / float64(period)
		}
		if i >= period {
			values = append(values, rsiFromAverages(gain, loss))
		}
	}
	return values, true
}

func rsiFromAverages(gain, loss float64) float64 {
	if gain == 0 && loss == 0 {
		return 50
	}
	if loss == 0 {
		return 100
	}
	return 100 - 100/(1+gain/loss)
}

// PercentileMidrank returns the percentile rank of value within observations.
// Equal observations receive half weight. NaN and infinite values are invalid.
func PercentileMidrank(value float64, observations []float64) (percentile float64, ok bool) {
	if len(observations) == 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	lower, equal := 0, 0
	for _, observation := range observations {
		if math.IsNaN(observation) || math.IsInf(observation, 0) {
			return 0, false
		}
		switch {
		case observation < value:
			lower++
		case observation == value:
			equal++
		}
	}
	return (float64(lower) + 0.5*float64(equal)) / float64(len(observations)) * 100, true
}

// CurrentDrawdown returns the last close's percentage change from the maximum
// close in the last period observations, including the last close itself.
// A full window of positive chronological closes is required.
func CurrentDrawdown(closes []int64, period int) (value float64, ok bool) {
	window, ok := closingWindow(closes, period)
	if !ok {
		return 0, false
	}
	peak := window[0]
	for _, close := range window[1:] {
		if close > peak {
			peak = close
		}
	}
	return (float64(window[len(window)-1])/float64(peak) - 1) * 100, true
}

// Calendar continuity and as-of filtering belong to the caller; these helpers
// have no dates and cannot detect missing sessions or future observations.
func closingWindow(closes []int64, period int) ([]int64, bool) {
	if period <= 0 || period > len(closes) {
		return nil, false
	}
	window := closes[len(closes)-period:]
	for _, close := range window {
		if close <= 0 {
			return nil, false
		}
	}
	return window, true
}
