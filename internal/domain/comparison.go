package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	MinComparisonTickers = 2
	MaxComparisonTickers = 10
)

// ComparisonMetric identifica um cálculo solicitado para uma comparação de ativos.
type ComparisonMetric string

const (
	ComparisonMetricReturn        ComparisonMetric = "return"
	ComparisonMetricVolatility    ComparisonMetric = "volatility"
	ComparisonMetricDrawdown      ComparisonMetric = "drawdown"
	ComparisonMetricAverageVolume ComparisonMetric = "averageVolume"
	ComparisonMetricHighLow       ComparisonMetric = "highLow"
	ComparisonMetricBestWorstDay  ComparisonMetric = "bestWorstDay"
	ComparisonMetricCorrelation   ComparisonMetric = "correlation"
)

var DefaultComparisonMetrics = []ComparisonMetric{
	ComparisonMetricReturn,
	ComparisonMetricVolatility,
	ComparisonMetricDrawdown,
	ComparisonMetricAverageVolume,
	ComparisonMetricHighLow,
	ComparisonMetricBestWorstDay,
}

var supportedComparisonMetrics = map[ComparisonMetric]struct{}{
	ComparisonMetricReturn:        {},
	ComparisonMetricVolatility:    {},
	ComparisonMetricDrawdown:      {},
	ComparisonMetricAverageVolume: {},
	ComparisonMetricHighLow:       {},
	ComparisonMetricBestWorstDay:  {},
	ComparisonMetricCorrelation:   {},
}

// ComparisonAssetStatus indica se um ativo pôde participar da comparação no
// período solicitado.
type ComparisonAssetStatus string

const (
	ComparisonAssetAvailable        ComparisonAssetStatus = "available"
	ComparisonAssetInsufficientData ComparisonAssetStatus = "insufficient_data"
)

// DailyPerformance identifica o melhor ou o pior pregão de um ativo.
// ReturnPercentage é expresso como percentual; por exemplo, 2.5 significa 2,5%.
type DailyPerformance struct {
	Date             time.Time
	ReturnPercentage float64
}

// ComparisonPoint representa um ponto opcional da série temporal retornada por
// uma comparação. NormalizedPerformance utiliza 100 como valor inicial de referência.
type ComparisonPoint struct {
	Date                  time.Time
	ClosePriceCents       int64
	NormalizedPerformance float64
}

// AssetComparison contém as métricas calculadas para um ticker solicitado.
// Campos ponteiros diferenciam um zero legítimo de uma métrica que não pôde ser
// calculada ou que não foi solicitada.
type AssetComparison struct {
	Ticker                      string
	Status                      ComparisonAssetStatus
	AvailableFrom               *time.Time
	AvailableTo                 *time.Time
	InitialPriceCents           *int64
	FinalPriceCents             *int64
	AbsoluteReturnCents         *int64
	PercentageReturn            *float64
	AnnualizedVolatility        *float64
	MaximumDrawdown             *float64
	AverageDailyVolumeCents     *int64
	LowestPriceCents            *int64
	HighestPriceCents           *int64
	BestDay                     *DailyPerformance
	WorstDay                    *DailyPerformance
	Series                      []ComparisonPoint
	InsufficientDataExplanation string
}

type AssetCorrelation struct {
	TickerA string
	TickerB string
	Value   float64
}

type ComparisonRanking struct {
	HighestReturnTicker    string
	LowestVolatilityTicker string
	LowestDrawdownTicker   string
}

type QuoteComparison struct {
	Assets       []AssetComparison
	Ranking      ComparisonRanking
	Correlations []AssetCorrelation
}

// NormalizeComparisonTickers valida, normaliza e preserva a ordem solicitada dos
// tickers. Tickers duplicados são rejeitados, em vez de removidos silenciosamente.
func NormalizeComparisonTickers(values []string) ([]string, error) {
	if len(values) < MinComparisonTickers || len(values) > MaxComparisonTickers {
		return nil, ValidationError{
			Field:   "tickers",
			Value:   strings.Join(values, ","),
			Message: fmt.Sprintf("tickers must contain between %d and %d unique values", MinComparisonTickers, MaxComparisonTickers),
			Err:     ErrInvalidTickers,
		}
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		ticker, err := NormalizeTicker(value)
		if err != nil {
			return nil, ValidationError{
				Field:   "tickers",
				Value:   value,
				Message: "each ticker must contain between 4 and 12 letters or numbers",
				Err:     ErrInvalidTickers,
			}
		}
		if _, exists := seen[ticker]; exists {
			return nil, ValidationError{
				Field:   "tickers",
				Value:   ticker,
				Message: "tickers must contain unique values",
				Err:     ErrInvalidTickers,
			}
		}

		seen[ticker] = struct{}{}
		normalized = append(normalized, ticker)
	}

	return normalized, nil
}

// NormalizeComparisonMetrics aplica o conjunto padrão quando values está vazio,
// valida cada métrica e remove duplicidades preservando a ordem.
func NormalizeComparisonMetrics(values []ComparisonMetric) ([]ComparisonMetric, error) {
	if len(values) == 0 {
		return append([]ComparisonMetric(nil), DefaultComparisonMetrics...), nil
	}

	normalized := make([]ComparisonMetric, 0, len(values))
	seen := make(map[ComparisonMetric]struct{}, len(values))
	for _, value := range values {
		metric := ComparisonMetric(strings.TrimSpace(string(value)))
		if _, supported := supportedComparisonMetrics[metric]; !supported {
			return nil, ValidationError{
				Field:   "metrics",
				Value:   string(value),
				Message: "unsupported comparison metric",
				Err:     ErrInvalidMetric,
			}
		}
		if _, exists := seen[metric]; exists {
			continue
		}

		seen[metric] = struct{}{}
		normalized = append(normalized, metric)
	}

	return normalized, nil
}
