package analytics_test

import (
	"math"
	"testing"

	"github.com/jamersom/market-data-api/internal/domain/analytics"
)

func TestReturn(t *testing.T) {
	for _, tc := range []struct {
		initial, final, absolute int64
		percentage               float64
	}{{10000, 12000, 2000, 20}, {12000, 9000, -3000, -25}, {10000, 10000, 0, 0}} {
		absolute, percentage := analytics.Return(tc.initial, tc.final)
		if absolute != tc.absolute || math.Abs(percentage-tc.percentage) > 1e-12 {
			t.Fatalf("Return(%d, %d) = %d, %g", tc.initial, tc.final, absolute, percentage)
		}
	}
}

func TestAnnualizedVolatility(t *testing.T) {
	// Returns -10%, 0%, +10% have sample variance 0.01.
	value, ok := analytics.AnnualizedVolatility([]float64{-0.1, 0, 0.1})
	if !ok || math.Abs(value-158.74507866387543) > 1e-10 {
		t.Fatalf("volatility = %g, %v", value, ok)
	}
	value, ok = analytics.AnnualizedVolatility([]float64{0.1, 0.1})
	if !ok || value != 0 {
		t.Fatalf("constant returns = %g, %v", value, ok)
	}
	for _, values := range [][]float64{nil, {0.1}} {
		if _, ok := analytics.AnnualizedVolatility(values); ok {
			t.Fatal("insufficient returns must be absent")
		}
	}
}

func TestMaximumDrawdown(t *testing.T) {
	for _, tc := range []struct {
		closes []int64
		want   float64
	}{{[]int64{100, 120, 90, 130}, -25}, {[]int64{100, 80, 60}, -40}, {[]int64{100, 110, 120}, 0}, {[]int64{100, 100}, 0}, {[]int64{100}, 0}} {
		value, ok := analytics.MaximumDrawdown(tc.closes)
		if !ok || math.Abs(value-tc.want) > 1e-12 {
			t.Fatalf("drawdown(%v) = %g, %v; want %g", tc.closes, value, ok, tc.want)
		}
	}
	if _, ok := analytics.MaximumDrawdown(nil); ok {
		t.Fatal("empty series must be absent")
	}
}

func TestAverageVolume(t *testing.T) {
	for _, tc := range []struct {
		volumes []int64
		want    int64
	}{{[]int64{100, 101}, 100}, {[]int64{0, 0}, 0}, {[]int64{math.MaxInt64, math.MaxInt64}, math.MaxInt64}} {
		value, ok := analytics.AverageVolume(tc.volumes)
		if !ok || value != tc.want {
			t.Fatalf("average(%v) = %d, %v; want %d", tc.volumes, value, ok, tc.want)
		}
	}
	if _, ok := analytics.AverageVolume(nil); ok {
		t.Fatal("empty volumes must be absent")
	}
}
