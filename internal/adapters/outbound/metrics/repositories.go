package metrics

import (
	"context"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/domain"
	"github.com/jamersom/market-data-api/internal/observability"
)

type quoteRepository interface {
	outbound.QuoteRepository
	outbound.ComparisonQuoteRepository
}

type QuoteRepository struct {
	next     quoteRepository
	registry *observability.Registry
}

func NewQuoteRepository(next quoteRepository, registry *observability.Registry) *QuoteRepository {
	return &QuoteRepository{next: next, registry: registry}
}

var _ outbound.QuoteRepository = (*QuoteRepository)(nil)
var _ outbound.ComparisonQuoteRepository = (*QuoteRepository)(nil)

func (r *QuoteRepository) FindLatestByTicker(ctx context.Context, ticker string, marketType int) (record domain.QuoteRecord, err error) {
	started := time.Now()
	defer func() { r.registry.RecordDatabase("quote_latest", time.Since(started), boolCount(err == nil), err) }()
	return r.next.FindLatestByTicker(ctx, ticker, marketType)
}

func (r *QuoteRepository) FindByTickerAndPeriod(ctx context.Context, ticker string, from, to time.Time, marketType, limit, offset int, order domain.SortOrder) (page domain.QuotePage, err error) {
	started := time.Now()
	defer func() { r.registry.RecordDatabase("quote_history", time.Since(started), len(page.Records), err) }()
	return r.next.FindByTickerAndPeriod(ctx, ticker, from, to, marketType, limit, offset, order)
}

func (r *QuoteRepository) FindByTickersAndPeriod(ctx context.Context, tickers []string, from, to time.Time, marketType int) (records map[string][]domain.QuoteRecord, err error) {
	started := time.Now()
	defer func() {
		rows := 0
		for _, values := range records {
			rows += len(values)
		}
		r.registry.RecordDatabase("comparison_history", time.Since(started), rows, err)
	}()
	return r.next.FindByTickersAndPeriod(ctx, tickers, from, to, marketType)
}

type IntelligenceQuoteRepository struct {
	next     outbound.IntelligenceQuoteRepository
	registry *observability.Registry
}

func NewIntelligenceQuoteRepository(next outbound.IntelligenceQuoteRepository, registry *observability.Registry) *IntelligenceQuoteRepository {
	return &IntelligenceQuoteRepository{next: next, registry: registry}
}

var _ outbound.IntelligenceQuoteRepository = (*IntelligenceQuoteRepository)(nil)

func (r *IntelligenceQuoteRepository) FindIntelligenceHistories(ctx context.Context, tickers []string, marketType int, asOf time.Time) (histories map[string]outbound.IntelligenceHistory, err error) {
	started := time.Now()
	defer func() {
		rows := 0
		for _, history := range histories {
			rows += len(history.Records)
		}
		r.registry.RecordDatabase("intelligence_history", time.Since(started), rows, err)
	}()
	return r.next.FindIntelligenceHistories(ctx, tickers, marketType, asOf)
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
