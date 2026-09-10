package postgres

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type intelligenceRows struct {
	pgx.Rows
	count, index          int
	closePrice            int64
	closed                bool
	scanErr, iterationErr error
}

func (r *intelligenceRows) Next() bool { r.index++; return r.index <= r.count }
func (r *intelligenceRows) Close()     { r.closed = true }
func (r *intelligenceRows) Err() error { return r.iterationErr }
func (r *intelligenceRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	for _, ptr := range dest {
		v := reflect.ValueOf(ptr).Elem()
		v.Set(reflect.Zero(v.Type()))
	}
	*dest[0].(*string) = "PETR4"
	*dest[1].(*time.Time) = time.Date(2024, 1, r.index, 0, 0, 0, 0, time.UTC)
	*dest[3].(*int) = 10
	*dest[7].(*string) = "BRL"
	*dest[12].(*int64) = r.closePrice
	*dest[26].(*time.Time) = time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	return nil
}

type intelligenceDB struct {
	rows  *intelligenceRows
	err   error
	args  []any
	query string
	calls int
}

func (d *intelligenceDB) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	d.calls++
	d.query, d.args = query, args
	return d.rows, d.err
}

type intelligenceCalendarStub struct {
	verified bool
	err      error
	from, to time.Time
	rows     *intelligenceRows
}

func (c *intelligenceCalendarStub) Sessions(ctx context.Context, from, to time.Time) ([]time.Time, bool, error) {
	c.from, c.to = from, to
	if !c.rows.closed {
		return nil, false, errors.New("rows still open")
	}
	return []time.Time{from}, c.verified, c.err
}

func TestIntelligenceRepositoryHistoryAndVersion(t *testing.T) {
	ctx := context.Background()
	date := time.Date(2024, 1, 5, 13, 0, 0, 0, time.FixedZone("test", -3*3600))
	load := func(price int64) (string, *intelligenceDB) {
		t.Helper()
		db := &intelligenceDB{rows: &intelligenceRows{count: 3, closePrice: price}}
		history, err := NewIntelligenceQuoteRepository(db, nil).FindIntelligenceHistory(ctx, "PETR4", 10, date)
		if err != nil {
			t.Fatal(err)
		}
		if len(history.Records) != 3 || history.CalendarVerified || len(history.Sessions) != 0 || !db.rows.closed {
			t.Fatalf("history: %+v", history)
		}
		if db.calls != 1 || db.args[0] != "PETR4" || db.args[1] != 10 || db.args[2].(time.Time).Format(time.RFC3339) != "2024-01-05T00:00:00Z" {
			t.Fatalf("query arguments: %v", db.args)
		}
		return history.DataVersion, db
	}
	a, _ := load(100)
	b, _ := load(100)
	c, _ := load(101)
	if a != b || a == c {
		t.Fatal("version must be stable and detect quote corrections")
	}
}

func TestIntelligenceRepositoryCalendar(t *testing.T) {
	for _, verified := range []bool{false, true} {
		db := &intelligenceDB{rows: &intelligenceRows{count: 1, closePrice: 100}}
		calendar := &intelligenceCalendarStub{verified: verified, rows: db.rows}
		cutoff := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)
		history, err := NewIntelligenceQuoteRepository(db, calendar).FindIntelligenceHistory(context.Background(), "PETR4", 10, cutoff)
		if err != nil {
			t.Fatal(err)
		}
		if history.CalendarVerified != verified || (len(history.Sessions) > 0) != verified || calendar.to != cutoff || calendar.from != history.Records[0].Quote.TradingDate {
			t.Fatalf("calendar: %+v", history)
		}
	}
}

func TestIntelligenceRepositoryErrors(t *testing.T) {
	want := errors.New("failure")
	for _, kind := range []string{"query", "scan", "iteration", "calendar"} {
		t.Run(kind, func(t *testing.T) {
			db := &intelligenceDB{rows: &intelligenceRows{count: 1}}
			calendar := &intelligenceCalendarStub{rows: db.rows}
			switch kind {
			case "query":
				db.err = want
			case "scan":
				db.rows.scanErr = want
			case "iteration":
				db.rows.iterationErr = want
			case "calendar":
				calendar.err = want
			}
			_, err := NewIntelligenceQuoteRepository(db, calendar).FindIntelligenceHistory(context.Background(), "PETR4", 10, time.Now())
			if !errors.Is(err, want) {
				t.Fatalf("error: %v", err)
			}
			if kind != "query" && !db.rows.closed {
				t.Fatal("rows not closed")
			}
		})
	}
	db := &intelligenceDB{rows: &intelligenceRows{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewIntelligenceQuoteRepository(db, nil).FindIntelligenceHistory(ctx, "PETR4", 10, time.Now()); !errors.Is(err, context.Canceled) || db.calls != 0 {
		t.Fatalf("cancel: %v", err)
	}
	history, err := NewIntelligenceQuoteRepository(db, nil).FindIntelligenceHistory(context.Background(), "PETR4", 10, time.Now())
	if err != nil || len(history.Records) != 0 || history.CalendarVerified {
		t.Fatalf("empty: %+v, %v", history, err)
	}
}
