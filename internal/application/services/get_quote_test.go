package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type getQuoteRepositoryStub struct {
	record domain.QuoteRecord
	err    error

	callCount  int
	ticker     string
	marketType int
}

func (stub *getQuoteRepositoryStub) FindLatestByTicker(
	_ context.Context,
	ticker string,
	marketType int,
) (domain.QuoteRecord, error) {
	stub.callCount++
	stub.ticker = ticker
	stub.marketType = marketType

	return stub.record, stub.err
}

func (stub *getQuoteRepositoryStub) FindByTickerAndPeriod(
	context.Context,
	string,
	time.Time,
	time.Time,
	int,
	int,
	int,
	domain.SortOrder,
) (domain.QuotePage, error) {
	return domain.QuotePage{}, nil
}

func TestGetQuoteServiceUsesDefaultMarketTypeAndNormalizesTicker(t *testing.T) {
	wantedRecord := domain.QuoteRecord{
		Quote: domain.Quote{
			Ticker:      "PETR4",
			MarketType:  domain.DefaultMarketType,
			TradingDate: time.Date(2026, time.September, 3, 0, 0, 0, 0, time.UTC),
		},
	}
	repository := &getQuoteRepositoryStub{record: wantedRecord}
	service := NewGetQuoteService(repository, discardLogger())

	output, err := service.Execute(context.Background(), inbound.GetQuoteInput{
		Ticker: " petr4 ",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if repository.callCount != 1 {
		t.Fatalf("repository call count = %d, want 1", repository.callCount)
	}
	if repository.ticker != "PETR4" {
		t.Errorf("repository ticker = %q, want PETR4", repository.ticker)
	}
	if repository.marketType != domain.DefaultMarketType {
		t.Errorf(
			"repository marketType = %d, want %d",
			repository.marketType,
			domain.DefaultMarketType,
		)
	}
	if output.Record != wantedRecord {
		t.Errorf("output record = %+v, want %+v", output.Record, wantedRecord)
	}
}

func TestGetQuoteServiceForwardsCustomMarketType(t *testing.T) {
	repository := &getQuoteRepositoryStub{}
	service := NewGetQuoteService(repository, discardLogger())

	_, err := service.Execute(context.Background(), inbound.GetQuoteInput{
		Ticker:     "VALE3",
		MarketType: 20,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if repository.ticker != "VALE3" || repository.marketType != 20 {
		t.Errorf(
			"repository query = ticker %q, marketType %d; want VALE3 and 20",
			repository.ticker,
			repository.marketType,
		)
	}
}

func TestGetQuoteServiceValidations(t *testing.T) {
	tests := []struct {
		name    string
		input   inbound.GetQuoteInput
		wantErr error
	}{
		{
			name:    "invalid ticker",
			input:   inbound.GetQuoteInput{Ticker: "?"},
			wantErr: domain.ErrInvalidTicker,
		},
		{
			name: "negative market type",
			input: inbound.GetQuoteInput{
				Ticker:     "PETR4",
				MarketType: -1,
			},
			wantErr: domain.ErrInvalidMarketType,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &getQuoteRepositoryStub{}
			service := NewGetQuoteService(repository, discardLogger())

			_, err := service.Execute(context.Background(), test.input)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"Execute() error = %v, want error wrapping %v",
					err,
					test.wantErr,
				)
			}
			if repository.callCount != 0 {
				t.Fatalf(
					"repository call count = %d, want 0",
					repository.callCount,
				)
			}
		})
	}
}

func TestGetQuoteServiceWrapsRepositoryError(t *testing.T) {
	repositoryError := errors.New("database unavailable")
	repository := &getQuoteRepositoryStub{err: repositoryError}
	service := NewGetQuoteService(repository, discardLogger())

	_, err := service.Execute(context.Background(), inbound.GetQuoteInput{
		Ticker: "PETR4",
	})
	if !errors.Is(err, repositoryError) {
		t.Fatalf(
			"Execute() error = %v, want wrapped repository error",
			err,
		)
	}
}

func TestGetQuoteServiceWritesStructuredDebugLogs(t *testing.T) {
	tradingDate := time.Date(2026, time.September, 3, 0, 0, 0, 0, time.UTC)
	repository := &getQuoteRepositoryStub{
		record: domain.QuoteRecord{
			Quote: domain.Quote{
				Ticker:      "PETR4",
				MarketType:  domain.DefaultMarketType,
				TradingDate: tradingDate,
			},
		},
	}

	var output bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	service := NewGetQuoteService(repository, log)

	_, err := service.Execute(context.Background(), inbound.GetQuoteInput{
		Ticker: "PETR4",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	entries := decodeLogEntries(t, output.Bytes())
	if len(entries) != 2 {
		t.Fatalf("log entry count = %d, want 2", len(entries))
	}

	assertLogField(t, entries[0], "level", "DEBUG")
	assertLogField(t, entries[0], "msg", "getting latest quote")
	assertLogField(t, entries[0], "ticker", "PETR4")
	assertLogField(t, entries[0], "market_type", float64(domain.DefaultMarketType))

	assertLogField(t, entries[1], "level", "DEBUG")
	assertLogField(t, entries[1], "msg", "latest quote retrieved")
	assertLogField(t, entries[1], "ticker", "PETR4")
	assertLogField(t, entries[1], "market_type", float64(domain.DefaultMarketType))
	assertLogField(t, entries[1], "trading_date", tradingDate.Format(time.RFC3339))
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}

func decodeLogEntries(t *testing.T, data []byte) []map[string]any {
	t.Helper()

	decoder := json.NewDecoder(bytes.NewReader(data))
	entries := make([]map[string]any, 0)

	for decoder.More() {
		var entry map[string]any
		if err := decoder.Decode(&entry); err != nil {
			t.Fatalf("decode log entry: %v", err)
		}
		entries = append(entries, entry)
	}

	return entries
}

func assertLogField(
	t *testing.T,
	entry map[string]any,
	field string,
	want any,
) {
	t.Helper()

	if entry[field] != want {
		t.Fatalf(
			"log field %q = %v, want %v",
			field,
			entry[field],
			want,
		)
	}
}
