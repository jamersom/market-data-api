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
 WHERE q.ticker = $1 AND q.market_type = $2::bigint
 AND q.trading_date <= $3::date
 ORDER BY q.trading_date, q.import_id`

func (r *IntelligenceQuoteRepository) FindIntelligenceHistory(ctx context.Context, ticker string, marketType int, asOf time.Time) (outbound.IntelligenceHistory, error) {
	var history outbound.IntelligenceHistory
	if err := ctx.Err(); err != nil {
		return history, err
	}
	if asOf.IsZero() {
		return history, fmt.Errorf("intelligence history requires an asOf date")
	}
	cutoff := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	rows, err := r.db.Query(ctx, intelligenceHistoryQuery, ticker, marketType, cutoff)
	if err != nil {
		return history, fmt.Errorf("query intelligence history: %w", err)
	}
	defer rows.Close()
	hash := sha256.New()
	encoder := json.NewEncoder(hash)
	// Include query identity, but not cutoff: equal returned data has the same
	// version even when requested on different non-trading days.
	if err := encoder.Encode(struct {
		Ticker     string
		MarketType int
	}{ticker, marketType}); err != nil {
		return history, err
	}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return outbound.IntelligenceHistory{}, err
		}
		record, err := scanQuoteRecord(rows)
		if err != nil {
			return outbound.IntelligenceHistory{}, fmt.Errorf("scan intelligence quote: %w", err)
		}
		// Normalize timestamps so session timezone settings do not change the hash.
		record.PublishedAt = record.PublishedAt.UTC()
		record.Quote.TradingDate = record.Quote.TradingDate.UTC()
		if record.Quote.ExpirationDate != nil {
			date := record.Quote.ExpirationDate.UTC()
			record.Quote.ExpirationDate = &date
		}
		if err := encoder.Encode(record); err != nil {
			return outbound.IntelligenceHistory{}, fmt.Errorf("version intelligence quote: %w", err)
		}
		history.Records = append(history.Records, record)
	}
	if err := rows.Err(); err != nil {
		return outbound.IntelligenceHistory{}, fmt.Errorf("iterate intelligence history: %w", err)
	}
	rows.Close() // Release the connection before a calendar provider may use it.
	history.DataVersion = "sha256-v1:" + hex.EncodeToString(hash.Sum(nil))
	if len(history.Records) > 0 && r.calendar != nil {
		sessions, verified, err := r.calendar.Sessions(ctx, history.Records[0].Quote.TradingDate, cutoff)
		if err != nil {
			return outbound.IntelligenceHistory{}, fmt.Errorf("load intelligence calendar: %w", err)
		}
		if verified {
			history.Sessions = append([]time.Time(nil), sessions...)
			history.CalendarVerified = true
		}
	}
	if err := ctx.Err(); err != nil {
		return outbound.IntelligenceHistory{}, err
	}
	return history, nil
}
