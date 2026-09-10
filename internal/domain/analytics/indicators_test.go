package analytics_test

import (
	"math"
	"slices"
	"testing"

	"github.com/jamersom/market-data-api/internal/domain/analytics"
)

func TestWindowIndicators(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fn     func([]int64, int) (float64, bool)
		closes []int64
		period int
		want   float64
	}{
		{"SMA trailing window", analytics.SMA, []int64{900, 100, 101}, 2, 100.5},
		{"SMA avoids sum overflow", analytics.SMA, []int64{math.MaxInt64, math.MaxInt64}, 2, float64(math.MaxInt64)},
		{"distance positive", analytics.DistanceFromSMA, []int64{100, 150}, 2, 20},
		{"distance negative", analytics.DistanceFromSMA, []int64{150, 100}, 2, -20},
		{"distance fractional mean", analytics.DistanceFromSMA, []int64{100, 101}, 2, 100.0 / 201},
		{"current excludes old peak", analytics.CurrentDrawdown, []int64{500, 100, 120, 90, 110}, 4, -100.0 / 12},
		{"current recovered", analytics.CurrentDrawdown, []int64{100, 80, 120}, 3, 0},
		{"current falling", analytics.CurrentDrawdown, []int64{100, 80, 60}, 3, -40},
		{"current flat", analytics.CurrentDrawdown, []int64{100, 100}, 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := slices.Clone(tc.closes)
			got, ok := tc.fn(tc.closes, tc.period)
			if !ok || math.IsNaN(got) || math.Abs(got-tc.want) > 1e-10 {
				t.Fatalf("got %g, %v; want %g", got, ok, tc.want)
			}
			if !slices.Equal(before, tc.closes) {
				t.Fatal("input mutated")
			}
		})
	}
}

func TestIndicatorsUnavailable(t *testing.T) {
	for name, fn := range map[string]func([]int64, int) (float64, bool){
		"SMA": analytics.SMA, "distance": analytics.DistanceFromSMA,
		"RSI": analytics.RSI, "drawdown": analytics.CurrentDrawdown,
	} {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				closes []int64
				period int
			}{
				{nil, 1}, {[]int64{100}, 0}, {[]int64{100}, -1},
				{[]int64{100}, math.MaxInt}, {[]int64{100, 0, 110}, 2},
				{[]int64{100, -1, 110}, 2},
			} {
				if _, ok := fn(tc.closes, tc.period); ok {
					t.Fatalf("accepted %+v", tc)
				}
			}
		})
	}
}

func TestRSIWilder(t *testing.T) {
	for _, tc := range []struct {
		name   string
		closes []int64
		period int
		want   float64
	}{
		// Initial gains 10+0+20 and losses 0+10+0 give RSI=75.
		{"seed", []int64{100, 110, 100, 120}, 3, 75},
		// Next loss=10: smoothed gain=20/3, loss=50/9; RSI=600/11.
		{"smoothing", []int64{100, 110, 100, 120, 110}, 3, 600.0 / 11},
		{"gains", []int64{100, 110, 120, 130, 140}, 3, 100},
		{"losses", []int64{140, 130, 120, 110, 100}, 3, 0},
		{"flat", []int64{100, 100, 100, 100, 100}, 3, 50},
		{"large prices small gains", []int64{math.MaxInt64 - 2, math.MaxInt64 - 1, math.MaxInt64}, 2, 100},
		{"period one", []int64{100, 110, 100}, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := slices.Clone(tc.closes)
			got, ok := analytics.RSI(tc.closes, tc.period)
			if !ok || math.IsNaN(got) || math.Abs(got-tc.want) > 1e-10 {
				t.Fatalf("got %g, %v; want %g", got, ok, tc.want)
			}
			if !slices.Equal(before, tc.closes) {
				t.Fatal("input mutated")
			}
			history, ok := analytics.RSIHistory(tc.closes, tc.period)
			if !ok || len(history) != len(tc.closes)-tc.period || math.Abs(history[len(history)-1]-got) > 1e-10 {
				t.Fatalf("history does not preserve RSI: %v, %v", history, ok)
			}
		})
	}
}

func TestPercentileMidrank(t *testing.T) {
	got, ok := analytics.PercentileMidrank(20, []float64{10, 20, 20, 30})
	if !ok || math.Abs(got-50) > 1e-10 {
		t.Fatalf("got %g, %v; want 50, true", got, ok)
	}
	for _, observations := range [][]float64{nil, {10, math.NaN()}} {
		if _, ok := analytics.PercentileMidrank(20, observations); ok {
			t.Fatalf("accepted invalid observations: %v", observations)
		}
	}
}

func TestIndicatorHistoryBoundaries(t *testing.T) {
	closes := make([]int64, 252)
	for i := range closes {
		closes[i] = 100
	}
	for _, tc := range []struct {
		name             string
		fn               func([]int64, int) (float64, bool)
		period, required int
	}{
		{"SMA20", analytics.SMA, 20, 20},
		{"distance SMA20", analytics.DistanceFromSMA, 20, 20},
		{"RSI14", analytics.RSI, 14, 15},
		{"drawdown252", analytics.CurrentDrawdown, 252, 252},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := tc.fn(closes[:tc.required-1], tc.period); ok {
				t.Fatal("accepted insufficient history")
			}
			if _, ok := tc.fn(closes[:tc.required], tc.period); !ok {
				t.Fatal("rejected full history")
			}
		})
	}
}
