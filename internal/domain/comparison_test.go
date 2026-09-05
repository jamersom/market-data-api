package domain

import (
	"errors"
	"reflect"
	"testing"
)

func TestNormalizeComparisonTickers(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr error
	}{
		{
			name:  "normalizes and preserves order",
			input: []string{" petr4 ", "vale3", "Itub4"},
			want:  []string{"PETR4", "VALE3", "ITUB4"},
		},
		{
			name:    "requires at least two tickers",
			input:   []string{"PETR4"},
			wantErr: ErrInvalidTickers,
		},
		{
			name:    "rejects more than ten tickers",
			input:   []string{"AAAA1", "AAAA2", "AAAA3", "AAAA4", "AAAA5", "AAAA6", "AAAA7", "AAAA8", "AAAA9", "AAA10", "AAA11"},
			wantErr: ErrInvalidTickers,
		},
		{
			name:    "rejects duplicate after normalization",
			input:   []string{"PETR4", " petr4 "},
			wantErr: ErrInvalidTickers,
		},
		{
			name:    "rejects an invalid ticker",
			input:   []string{"PETR4", "VA!"},
			wantErr: ErrInvalidTickers,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeComparisonTickers(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestNormalizeComparisonMetrics(t *testing.T) {
	t.Run("uses an independent copy of defaults", func(t *testing.T) {
		got, err := NormalizeComparisonMetrics(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, DefaultComparisonMetrics) {
			t.Fatalf("expected %v, got %v", DefaultComparisonMetrics, got)
		}

		got[0] = ComparisonMetricCorrelation
		if DefaultComparisonMetrics[0] != ComparisonMetricReturn {
			t.Fatal("default metrics must not be mutated by callers")
		}
	})

	t.Run("preserves order and removes duplicates", func(t *testing.T) {
		input := []ComparisonMetric{
			ComparisonMetricCorrelation,
			ComparisonMetricReturn,
			ComparisonMetricCorrelation,
		}
		want := []ComparisonMetric{ComparisonMetricCorrelation, ComparisonMetricReturn}

		got, err := NormalizeComparisonMetrics(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})

	t.Run("rejects unsupported metric", func(t *testing.T) {
		_, err := NormalizeComparisonMetrics([]ComparisonMetric{"sharpeRatio"})
		if !errors.Is(err, ErrInvalidMetric) {
			t.Fatalf("expected error %v, got %v", ErrInvalidMetric, err)
		}
	})
}
