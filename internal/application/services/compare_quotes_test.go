package services

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type comparisonRepositoryStub struct {
	records  map[string][]domain.QuoteRecord
	err      error
	calls    int
	tickers  []string
	from, to time.Time
	market   int
}

func (r *comparisonRepositoryStub) FindByTickersAndPeriod(ctx context.Context, tickers []string, from, to time.Time, market int) (map[string][]domain.QuoteRecord, error) {
	r.calls++
	r.tickers = tickers
	r.from, r.to, r.market = from, to, market
	return r.records, r.err
}

func comparisonInput() inbound.CompareQuotesInput {
	return inbound.CompareQuotesInput{Tickers: []string{" petr4 ", "vale3"}, From: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)}
}

func comparisonRecords(prices ...int64) []domain.QuoteRecord {
	records := make([]domain.QuoteRecord, len(prices))
	for i, price := range prices {
		records[i].Quote = domain.Quote{TradingDate: comparisonInput().From.AddDate(0, 0, i), ClosePriceCents: price, HighPriceCents: price + 10, LowPriceCents: price - 10, TradedVolumeCents: int64(i+1) * 100, Currency: "BRL"}
	}
	return records
}

func validComparisonRepository() *comparisonRepositoryStub {
	return &comparisonRepositoryStub{records: map[string][]domain.QuoteRecord{"PETR4": comparisonRecords(100, 120, 90), "VALE3": comparisonRecords(100, 110, 121)}}
}

func closeTo(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > 1e-9 {
		t.Fatalf("got %v, want %.12f", got, want)
	}
}

func TestCompareQuotesMetricsAndBatch(t *testing.T) {
	repo := validComparisonRepository()
	// A ordem dos registros do repositório não deve afetar os resultados nem ser alterada pelo cálculo.
	repo.records["PETR4"][0], repo.records["PETR4"][2] = repo.records["PETR4"][2], repo.records["PETR4"][0]
	input := comparisonInput()
	input.IncludeSeries = true
	output, err := NewCompareQuotesService(repo, discardLogger()).Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 || !reflect.DeepEqual(repo.tickers, []string{"PETR4", "VALE3"}) || repo.market != 10 || !repo.from.Equal(input.From) || !repo.to.Equal(input.To) {
		t.Fatalf("unexpected batch: %+v", repo)
	}
	if input.Tickers[0] != " petr4 " || repo.records["PETR4"][0].Quote.ClosePriceCents != 90 {
		t.Fatal("caller data mutated")
	}
	asset := output.Assets[0]
	if asset.Ticker != "PETR4" || asset.Status != domain.ComparisonAssetAvailable {
		t.Fatalf("unexpected asset: %+v", asset)
	}
	closeTo(t, asset.PercentageReturn, -10)
	closeTo(t, asset.MaximumDrawdown, -25)
	closeTo(t, asset.AnnualizedVolatility, math.Sqrt(0.10125)*math.Sqrt(252)*100)
	if *asset.AbsoluteReturnCents != -10 || *asset.AverageDailyVolumeCents != 200 || *asset.LowestPriceCents != 80 || *asset.HighestPriceCents != 130 {
		t.Fatalf("unexpected amounts: %+v", asset)
	}
	closeTo(t, &asset.BestDay.ReturnPercentage, 20)
	closeTo(t, &asset.WorstDay.ReturnPercentage, -25)
	if !asset.BestDay.Date.Equal(input.From.AddDate(0, 0, 1)) || len(asset.Series) != 3 || asset.Series[0].NormalizedPerformance != 100 || asset.Series[2].NormalizedPerformance != 90 {
		t.Fatalf("unexpected dates/series: %+v", asset)
	}
	if output.Ranking.HighestReturnTicker != "VALE3" || output.Ranking.LowestVolatilityTicker != "VALE3" || output.Ranking.LowestDrawdownTicker != "VALE3" {
		t.Fatalf("unexpected ranking: %+v", output.Ranking)
	}
	if output.PriceAdjustment != "unadjusted" || output.CalculationVersion != "1.0" || output.Currency != "BRL" {
		t.Fatalf("unexpected metadata: %+v", output)
	}
}

func TestCompareQuotesValidationBeforeRepository(t *testing.T) {
	tests := []struct {
		name   string
		change func(*inbound.CompareQuotesInput)
		want   error
	}{
		{"tickers", func(i *inbound.CompareQuotesInput) { i.Tickers = []string{"PETR4"} }, domain.ErrInvalidTickers},
		{"duplicate", func(i *inbound.CompareQuotesInput) { i.Tickers = []string{"PETR4", " petr4 "} }, domain.ErrInvalidTickers},
		{"metric", func(i *inbound.CompareQuotesInput) { i.Metrics = []domain.ComparisonMetric{"invalid"} }, domain.ErrInvalidMetric},
		{"missing from", func(i *inbound.CompareQuotesInput) { i.From = time.Time{} }, domain.ErrInvalidDateRange},
		{"missing to", func(i *inbound.CompareQuotesInput) { i.To = time.Time{} }, domain.ErrInvalidDateRange},
		{"inverted", func(i *inbound.CompareQuotesInput) { i.To = i.From.AddDate(0, 0, -1) }, domain.ErrInvalidDateRange},
		{"too long", func(i *inbound.CompareQuotesInput) { i.To = i.From.AddDate(5, 0, 1) }, domain.ErrInvalidDateRange},
		{"market", func(i *inbound.CompareQuotesInput) { i.MarketType = -1 }, domain.ErrInvalidMarketType},
		{"benchmark", func(i *inbound.CompareQuotesInput) { i.Benchmark = "!" }, domain.ErrInvalidTicker},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := validComparisonRepository()
			input := comparisonInput()
			tt.change(&input)
			_, err := NewCompareQuotesService(repo, nil).Execute(context.Background(), input)
			if !errors.Is(err, tt.want) || repo.calls != 0 {
				t.Fatalf("error %v, calls %d", err, repo.calls)
			}
		})
	}
}

func TestCompareQuotesPartialAndBenchmark(t *testing.T) {
	repo := validComparisonRepository()
	input := comparisonInput()
	input.Tickers = append(input.Tickers, "ABCD3")
	input.Benchmark = " ibov "
	output, err := NewCompareQuotesService(repo, nil).Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if output.Assets[2].Status != domain.ComparisonAssetInsufficientData || output.Benchmark.Status != domain.ComparisonAssetInsufficientData || output.Benchmark.Ticker != "IBOV" || len(repo.tickers) != 4 {
		t.Fatalf("unexpected partial output: %+v", output)
	}
	input.Benchmark = "petr4"
	output, err = NewCompareQuotesService(repo, nil).Execute(context.Background(), input)
	if err != nil || len(repo.tickers) != 3 || output.Benchmark.Status != domain.ComparisonAssetAvailable {
		t.Fatalf("benchmark reuse: %+v, %v", output, err)
	}
	delete(repo.records, "VALE3")
	_, err = NewCompareQuotesService(repo, nil).Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrInsufficientComparisonData) {
		t.Fatalf("got %v", err)
	}
}

func TestCompareQuotesSelectionAndShortHistories(t *testing.T) {
	repo := validComparisonRepository()
	repo.records["PETR4"] = comparisonRecords(100, 110)
	input := comparisonInput()
	input.Metrics = []domain.ComparisonMetric{domain.ComparisonMetricVolatility}
	output, err := NewCompareQuotesService(repo, nil).Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	asset := output.Assets[0]
	if asset.Status != domain.ComparisonAssetAvailable || asset.AnnualizedVolatility != nil || asset.PercentageReturn != nil || asset.Series != nil || asset.MaximumDrawdown != nil || asset.AverageDailyVolumeCents != nil || asset.BestDay != nil || asset.HighestPriceCents != nil || output.Ranking.HighestReturnTicker != "" {
		t.Fatalf("unexpected optional metrics: %+v", asset)
	}
	for _, records := range [][]domain.QuoteRecord{nil, comparisonRecords(100), comparisonRecords(0, 100), comparisonRecords(-10, 100)} {
		repo.records["PETR4"] = records
		_, err := NewCompareQuotesService(repo, nil).Execute(context.Background(), input)
		if !errors.Is(err, domain.ErrInsufficientComparisonData) {
			t.Fatalf("got %v for %+v", err, records)
		}
	}
}

func TestCompareQuotesErrorsAndCancellation(t *testing.T) {
	repo := validComparisonRepository()
	repo.err = errors.New("database unavailable")
	_, err := NewCompareQuotesService(repo, nil).Execute(context.Background(), comparisonInput())
	if !errors.Is(err, repo.err) {
		t.Fatalf("error not preserved: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo.calls = 0
	_, err = NewCompareQuotesService(repo, nil).Execute(ctx, comparisonInput())
	if !errors.Is(err, context.Canceled) || repo.calls != 0 {
		t.Fatalf("error %v, calls %d", err, repo.calls)
	}
}

func TestCompareQuotesCorrelation(t *testing.T) {
	input := comparisonInput()
	input.Metrics = []domain.ComparisonMetric{domain.ComparisonMetricCorrelation}
	repo := validComparisonRepository()
	repo.records["VALE3"] = comparisonRecords(200, 240, 180)
	output, err := NewCompareQuotesService(repo, nil).Execute(context.Background(), input)
	if err != nil || len(output.Correlations) != 1 {
		t.Fatalf("correlations %+v, error %v", output.Correlations, err)
	}
	closeTo(t, &output.Correlations[0].Value, 1)
	// Retornos diários em sentidos opostos: -20%, depois +25%.
	repo.records["VALE3"] = comparisonRecords(100, 80, 100)
	output, err = NewCompareQuotesService(repo, nil).Execute(context.Background(), input)
	if err != nil || len(output.Correlations) != 1 {
		t.Fatalf("correlations %+v, error %v", output.Correlations, err)
	}
	closeTo(t, &output.Correlations[0].Value, -1)
	repo.records["VALE3"] = comparisonRecords(100, 100, 100)
	output, err = NewCompareQuotesService(repo, nil).Execute(context.Background(), input)
	if err != nil || len(output.Correlations) != 0 {
		t.Fatalf("constant series: %+v, %v", output.Correlations, err)
	}
	// Históricos com lacunas nas datas não possuem intervalos suficientes em comum.
	repo.records["VALE3"] = comparisonRecords(200, 240, 180)
	repo.records["VALE3"][1].Quote.TradingDate = input.From.Add(12*time.Hour).AddDate(0, 0, 3)
	output, err = NewCompareQuotesService(repo, nil).Execute(context.Background(), input)
	if err != nil || len(output.Correlations) != 0 {
		t.Fatalf("gapped series: %+v, %v", output.Correlations, err)
	}
}

func TestCompareQuotesVolumeOverflowAndTies(t *testing.T) {
	repo := validComparisonRepository()
	for _, ticker := range []string{"PETR4", "VALE3"} {
		repo.records[ticker] = comparisonRecords(100, 100, 100)
		for i := range repo.records[ticker] {
			repo.records[ticker][i].Quote.TradedVolumeCents = math.MaxInt64
		}
	}
	output, err := NewCompareQuotesService(repo, nil).Execute(context.Background(), comparisonInput())
	if err != nil {
		t.Fatal(err)
	}
	if *output.Assets[0].AverageDailyVolumeCents != math.MaxInt64 || output.Ranking.HighestReturnTicker != "PETR4" || output.Ranking.LowestVolatilityTicker != "PETR4" || output.Ranking.LowestDrawdownTicker != "PETR4" {
		t.Fatalf("unexpected output: %+v", output)
	}
}
