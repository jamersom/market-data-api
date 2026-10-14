package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/domain"
	"github.com/jamersom/market-data-api/internal/observability"
)

type quoteRepositoryStub struct {
	err error
}

func (s quoteRepositoryStub) FindLatestByTicker(context.Context, string, int) (domain.QuoteRecord, error) {
	return domain.QuoteRecord{}, s.err
}

func (s quoteRepositoryStub) FindByTickerAndPeriod(context.Context, string, time.Time, time.Time, int, int, int, domain.SortOrder) (domain.QuotePage, error) {
	return domain.QuotePage{Records: make([]domain.QuoteRecord, 3)}, s.err
}

func (s quoteRepositoryStub) FindByTickersAndPeriod(context.Context, []string, time.Time, time.Time, int) (map[string][]domain.QuoteRecord, error) {
	return map[string][]domain.QuoteRecord{"PETR4": make([]domain.QuoteRecord, 2), "VALE3": make([]domain.QuoteRecord, 4)}, s.err
}

type intelligenceRepositoryStub struct {
	err error
}

func (s intelligenceRepositoryStub) FindIntelligenceHistories(context.Context, []string, int, time.Time) (map[string]outbound.IntelligenceHistory, error) {
	return map[string]outbound.IntelligenceHistory{
		"PETR4": {Records: make([]domain.QuoteRecord, 10)},
		"IBOV":  {Records: make([]domain.QuoteRecord, 8)},
	}, s.err
}

func TestRepositoryMetricsRecordRowsAndErrors(t *testing.T) {
	registry := observability.NewRegistry()
	quotes := NewQuoteRepository(quoteRepositoryStub{}, registry)
	_, _ = quotes.FindLatestByTicker(context.Background(), "PETR4", 10)
	_, _ = quotes.FindByTickerAndPeriod(context.Background(), "PETR4", time.Time{}, time.Time{}, 10, 10, 0, domain.SortAscending)
	_, _ = quotes.FindByTickersAndPeriod(context.Background(), []string{"PETR4", "VALE3"}, time.Time{}, time.Time{}, 10)
	intelligence := NewIntelligenceQuoteRepository(intelligenceRepositoryStub{err: errors.New("query failed")}, registry)
	_, _ = intelligence.FindIntelligenceHistories(context.Background(), []string{"PETR4", "IBOV"}, 10, time.Now())

	metrics := registry.Snapshot().Database
	got := make(map[string]observability.DatabaseMetric, len(metrics))
	for _, metric := range metrics {
		got[metric.Operation] = metric
	}
	if got["quote_latest"].Rows != 1 || got["quote_history"].Rows != 3 || got["comparison_history"].Rows != 6 {
		t.Fatalf("unexpected quote rows: %+v", got)
	}
	if got["intelligence_history"].Rows != 18 || got["intelligence_history"].Errors != 1 {
		t.Fatalf("unexpected intelligence metric: %+v", got["intelligence_history"])
	}
}
