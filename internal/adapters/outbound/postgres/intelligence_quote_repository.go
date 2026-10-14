package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
)

// IntelligenceCalendar supplies independently verified exchange sessions.
// A provider must return verified=false if any portion of [from,to] is outside
// its coverage. Sessions derived from quote dates are not a verified calendar.
type IntelligenceCalendar interface {
	Sessions(ctx context.Context, from, to time.Time) (sessions []time.Time, verified bool, err error)
}

type intelligenceQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

type IntelligenceQuoteRepository struct {
	db       intelligenceQueryer
	calendar IntelligenceCalendar
}

var _ outbound.IntelligenceQuoteRepository = (*IntelligenceQuoteRepository)(nil)

// NewIntelligenceQuoteRepository accepts a pgx pool or transaction. A nil
// calendar deliberately leaves calendar coverage unverified; it never guesses
// sessions from weekdays or historical quotes.
func NewIntelligenceQuoteRepository(db intelligenceQueryer, calendar IntelligenceCalendar) *IntelligenceQuoteRepository {
	return &IntelligenceQuoteRepository{db: db, calendar: calendar}
}

const intelligenceHistoryQuery = `SELECT ` + quoteProjection + `,
 NULL::BIGINT, i.published_at, i.source_url, i.parser_version, i.layout_version
 FROM public.published_historical_quotes q
 JOIN public.historical_imports i ON i.id = q.import_id
 WHERE q.ticker = ANY($1::text[]) AND q.market_type = $2::bigint
 AND q.trading_date <= $3::date
 ORDER BY q.ticker, q.trading_date, q.import_id`

func (r *IntelligenceQuoteRepository) FindIntelligenceHistory(ctx context.Context, ticker string, marketType int, asOf time.Time) (outbound.IntelligenceHistory, error) {
	histories, err := r.FindIntelligenceHistories(ctx, []string{ticker}, marketType, asOf)
	if err != nil {
		return outbound.IntelligenceHistory{}, err
	}
	return histories[ticker], nil
}

func (r *IntelligenceQuoteRepository) FindIntelligenceHistories(ctx context.Context, tickers []string, marketType int, asOf time.Time) (map[string]outbound.IntelligenceHistory, error) {
	histories := make(map[string]outbound.IntelligenceHistory, len(tickers))
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if asOf.IsZero() {
		return nil, fmt.Errorf("intelligence history requires an asOf date")
	}
	if len(tickers) == 0 {
		return histories, nil
	}
	for _, ticker := range tickers {
		histories[ticker] = outbound.IntelligenceHistory{}
	}
	cutoff := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	rows, err := r.db.Query(ctx, intelligenceHistoryQuery, tickers, marketType, cutoff)
	if err != nil {
		return nil, fmt.Errorf("query intelligence history: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		record, err := scanQuoteRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("scan intelligence quote: %w", err)
		}
		history, requested := histories[record.Quote.Ticker]
		if !requested {
			return nil, fmt.Errorf("query intelligence history returned unexpected ticker %s", record.Quote.Ticker)
		}
		// Normalize timestamps so session timezone settings do not change the hash.
		record.PublishedAt = record.PublishedAt.UTC()
		record.Quote.TradingDate = record.Quote.TradingDate.UTC()
		if record.Quote.ExpirationDate != nil {
			date := record.Quote.ExpirationDate.UTC()
			record.Quote.ExpirationDate = &date
		}
		history.Records = append(history.Records, record)
		histories[record.Quote.Ticker] = history
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate intelligence history: %w", err)
	}
	rows.Close() // Release the connection before a calendar provider may use it.
	var earliest time.Time
	for _, ticker := range tickers {
		history := histories[ticker]
		hash := sha256.New()
		encoder := json.NewEncoder(hash)
		// Include query identity, but not cutoff: equal returned data has the same
		// version even when requested on different non-trading days.
		if err := encoder.Encode(struct {
			Ticker     string
			MarketType int
		}{ticker, marketType}); err != nil {
			return nil, err
		}
		for _, record := range history.Records {
			if err := encoder.Encode(record); err != nil {
				return nil, fmt.Errorf("version intelligence quote: %w", err)
			}
		}
		history.DataVersion = "sha256-v1:" + hex.EncodeToString(hash.Sum(nil))
		histories[ticker] = history
		if len(history.Records) > 0 && (earliest.IsZero() || history.Records[0].Quote.TradingDate.Before(earliest)) {
			earliest = history.Records[0].Quote.TradingDate
		}
	}
	if !earliest.IsZero() && r.calendar != nil {
		sessions, verified, err := r.calendar.Sessions(ctx, earliest, cutoff)
		if err != nil {
			return nil, fmt.Errorf("load intelligence calendar: %w", err)
		}
		if verified {
			for _, ticker := range tickers {
				history := histories[ticker]
				history.Sessions = append([]time.Time(nil), sessions...)
				history.CalendarVerified = true
				histories[ticker] = history
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return histories, nil
}
