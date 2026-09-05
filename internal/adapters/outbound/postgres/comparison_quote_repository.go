package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

var _ outbound.ComparisonQuoteRepository = (*QuoteRepository)(nil)

func (r *QuoteRepository) FindByTickersAndPeriod(ctx context.Context, tickers []string, from, to time.Time, marketType int) (map[string][]domain.QuoteRecord, error) {
	// A comparação usa os fechamentos do próprio período, sem precisar do fechamento anterior.
	query := `SELECT ` + quoteProjection + `,
 NULL::BIGINT, i.published_at, i.source_url, i.parser_version, i.layout_version
 FROM public.published_historical_quotes q
 JOIN public.historical_imports i ON i.id = q.import_id
 WHERE q.ticker = ANY($1::text[])
 AND q.market_type = $2::bigint AND q.trading_date BETWEEN $3 AND $4
 ORDER BY q.ticker, q.trading_date`
	rows, err := r.db.Query(ctx, query, tickers, marketType, from, to)
	if err != nil {
		return nil, fmt.Errorf("query comparison quotes: %w", err)
	}
	defer rows.Close()
	records := make(map[string][]domain.QuoteRecord, len(tickers))
	for _, ticker := range tickers {
		records[ticker] = []domain.QuoteRecord{}
	}
	for rows.Next() {
		record, err := scanQuoteRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan comparison quote: %w", err)
		}
		records[record.Quote.Ticker] = append(records[record.Quote.Ticker], record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comparison quotes: %w", err)
	}
	return records, nil
}
