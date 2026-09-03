package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jamersom/market-data-api/internal/domain"
)

type QuoteRepository struct {
	db *pgxpool.Pool
}

func NewQuoteRepository(db *pgxpool.Pool) *QuoteRepository {
	return &QuoteRepository{db: db}
}

const quoteProjection = `
	q.ticker,
	q.trading_date,
	q.bdi_code,
	q.market_type,
	q.short_name,
	q.specification,
	COALESCE(q.term, ''),
	q.currency,
	ROUND(q.open_price * 100)::BIGINT,
	ROUND(q.high_price * 100)::BIGINT,
	ROUND(q.low_price * 100)::BIGINT,
	ROUND(q.average_price * 100)::BIGINT,
	ROUND(q.close_price * 100)::BIGINT,
	ROUND(q.best_bid_price * 100)::BIGINT,
	ROUND(q.best_ask_price * 100)::BIGINT,
	q.trade_count,
	q.traded_quantity,
	ROUND(q.traded_volume * 100)::BIGINT,
	ROUND(q.strike_price * 100)::BIGINT,
	COALESCE(q.option_indicator, ''),
	q.expiration_date,
	q.quote_factor,
	ROUND(q.strike_points * 1000000)::BIGINT,
	q.isin,
	q.distribution_number`

func (r *QuoteRepository) FindLatestByTicker(ctx context.Context, ticker string, marketType int) (domain.QuoteRecord, error) {
	query := `
	WITH ranked AS (
		SELECT
			q.*,
			LAG(q.close_price) OVER (
				ORDER BY q.trading_date
			) AS previous_close
		FROM public.published_historical_quotes q
		WHERE q.ticker = $1
		  AND q.market_type = $2
	)
	SELECT
		` + quoteProjection + `,
		CASE
			WHEN q.previous_close IS NULL THEN NULL
			ELSE ROUND(q.previous_close * 100)::BIGINT
		END,
		i.published_at,
		i.source_url,
		i.parser_version,
		i.layout_version
	FROM ranked q
	JOIN public.historical_imports i
	  ON i.id = q.import_id
	ORDER BY q.trading_date DESC
	LIMIT 1`
	record, err := scanQuoteRecord(r.db.QueryRow(ctx, query, ticker, marketType))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.QuoteRecord{}, domain.ResourceNotFoundError{Resource: "quote", Ticker: ticker}
	}
	if err != nil {
		return domain.QuoteRecord{}, fmt.Errorf("query latest quote for ticker %s: %w", ticker, err)
	}
	return record, nil
}

func (r *QuoteRepository) FindByTickerAndPeriod(
	ctx context.Context,
	ticker string,
	from time.Time,
	to time.Time,
	marketType int,
	limit int,
	offset int,
	order domain.SortOrder,
) (domain.QuotePage, error) {
	var total int64

	const countQuery = `
		SELECT COUNT(*)
		FROM public.published_historical_quotes q
		WHERE q.ticker = $1
		  AND q.market_type = $2
		  AND q.trading_date BETWEEN $3 AND $4
	`

	err := r.db.QueryRow(
		ctx,
		countQuery,
		ticker,
		marketType,
		from,
		to,
	).Scan(&total)
	if err != nil {
		return domain.QuotePage{}, fmt.Errorf("count quotes for ticker %s: %w", ticker, err)
	}
	direction := "ASC"
	if order == domain.SortDescending {
		direction = "DESC"
	}
	query := `
	WITH ranked AS (
		SELECT
			q.*,
			LAG(q.close_price) OVER (
				ORDER BY q.trading_date
			) AS previous_close
		FROM public.published_historical_quotes q
		WHERE q.ticker = $1
		  AND q.market_type = $2
	)
	SELECT
		` + quoteProjection + `,
		CASE
			WHEN q.previous_close IS NULL THEN NULL
			ELSE ROUND(q.previous_close * 100)::BIGINT
		END,
		i.published_at,
		i.source_url,
		i.parser_version,
		i.layout_version
	FROM ranked q
	JOIN public.historical_imports i
	  ON i.id = q.import_id
	WHERE q.trading_date BETWEEN $3 AND $4
	ORDER BY q.trading_date ` + direction + `
	LIMIT $5
	OFFSET $6`
	rows, err := r.db.Query(ctx, query, ticker, marketType, from, to, limit, offset)
	if err != nil {
		return domain.QuotePage{}, fmt.Errorf("query quotes by period for ticker %s: %w", ticker, err)
	}
	defer rows.Close()
	records := make([]domain.QuoteRecord, 0)
	for rows.Next() {
		record, err := scanQuoteRecord(rows)
		if err != nil {
			return domain.QuotePage{}, fmt.Errorf("scan quote for ticker %s: %w", ticker, err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return domain.QuotePage{}, fmt.Errorf("iterate quotes for ticker %s: %w", ticker, err)
	}
	return domain.QuotePage{Records: records, Total: total}, nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanQuoteRecord(row rowScanner) (domain.QuoteRecord, error) {
	var record domain.QuoteRecord
	q := &record.Quote
	err := row.Scan(
		&q.Ticker,
		&q.TradingDate,
		&q.BDICode,
		&q.MarketType,
		&q.ShortName,
		&q.Specification,
		&q.Term,
		&q.Currency,
		&q.OpenPriceCents,
		&q.HighPriceCents,
		&q.LowPriceCents,
		&q.AveragePriceCents,
		&q.ClosePriceCents,
		&q.BestBidPriceCents,
		&q.BestAskPriceCents,
		&q.TradeCount,
		&q.TradedQuantity,
		&q.TradedVolumeCents,
		&q.StrikePriceCents,
		&q.OptionIndicator,
		&q.ExpirationDate,
		&q.QuoteFactor,
		&q.StrikePointsScaled,
		&q.ISIN,
		&q.DistributionNumber,
		&record.PreviousCloseCents,
		&record.PublishedAt,
		&record.SourceURL,
		&record.ParserVersion,
		&record.LayoutVersion,
	)
	return record, err
}
