package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

// Prices and calendar use one snapshot, including during concurrent publication.
type ObservedIntelligenceQuoteRepository struct{ db *pgxpool.Pool }

func NewObservedIntelligenceQuoteRepository(db *pgxpool.Pool) *ObservedIntelligenceQuoteRepository {
	return &ObservedIntelligenceQuoteRepository{db: db}
}

var _ outbound.IntelligenceQuoteRepository = (*ObservedIntelligenceQuoteRepository)(nil)

func (r *ObservedIntelligenceQuoteRepository) FindIntelligenceHistory(ctx context.Context, ticker string, market int, asOf time.Time) (outbound.IntelligenceHistory, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return outbound.IntelligenceHistory{}, fmt.Errorf("begin intelligence snapshot: %w", err)
	}
	defer tx.Rollback(context.Background())
	history, err := NewIntelligenceQuoteRepository(tx, nil).FindIntelligenceHistory(ctx, ticker, market, asOf)
	if err != nil {
		return outbound.IntelligenceHistory{}, err
	}
	if len(history.Records) > 0 {
		from := history.Records[0].Quote.TradingDate
		to := history.Records[len(history.Records)-1].Quote.TradingDate
		history.Sessions, history.Calendar, err = loadObservedCalendar(ctx, tx, market, from, to)
		if err != nil {
			return outbound.IntelligenceHistory{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return outbound.IntelligenceHistory{}, fmt.Errorf("finish intelligence snapshot: %w", err)
	}
	return history, nil
}

func loadObservedCalendar(ctx context.Context, db intelligenceQueryer, market int, from, to time.Time) ([]time.Time, domain.IntelligenceCalendarMetadata, error) {
	meta := domain.IntelligenceCalendarMetadata{Source: "cotahist_observed", Policy: "observed_import_integrity_v1", Coverage: []domain.CalendarCoverage{}}
	rows, err := db.Query(ctx, `SELECT reference_year, observed_from, observed_to,
 import_integrity_validated, calendar_version
 FROM public.observed_calendar_coverage
 WHERE market_type=$1 AND reference_year BETWEEN $2 AND $3
 ORDER BY reference_year, import_id`, market, from.Year(), to.Year())
	if err != nil {
		return nil, meta, fmt.Errorf("query observed calendar coverage (b3-data-hub migrations 003/004 required): %w", err)
	}
	for rows.Next() {
		var c domain.CalendarCoverage
		if err := rows.Scan(&c.Year, &c.From, &c.To, &c.IntegrityValidated, &c.Version); err != nil {
			rows.Close()
			return nil, meta, fmt.Errorf("scan calendar coverage: %w", err)
		}
		meta.Coverage = append(meta.Coverage, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, meta, err
	}
	rows.Close()
	rows, err = db.Query(ctx, `SELECT trading_date FROM public.observed_trading_sessions
 WHERE market_type=$1 AND trading_date BETWEEN $2::date AND $3::date
 ORDER BY trading_date`, market, from, to)
	if err != nil {
		return nil, meta, fmt.Errorf("query observed sessions: %w", err)
	}
	defer rows.Close()
	sessions := make([]time.Time, 0)
	for rows.Next() {
		var date time.Time
		if err := rows.Scan(&date); err != nil {
			return nil, meta, err
		}
		sessions = append(sessions, date)
	}
	if err := rows.Err(); err != nil {
		return nil, meta, err
	}
	hash := sha256.New()
	if err := json.NewEncoder(hash).Encode(struct {
		Market   int
		From, To time.Time
		Metadata domain.IntelligenceCalendarMetadata
		Sessions []time.Time
	}{market, from, to, meta, sessions}); err != nil {
		return nil, meta, err
	}
	meta.Version = "sha256-v1:" + hex.EncodeToString(hash.Sum(nil))
	return sessions, meta, nil
}
