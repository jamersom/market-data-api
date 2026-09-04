package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type quoteRepositoryStub struct {
	page domain.QuotePage
	err  error

	ticker     string
	from       time.Time
	to         time.Time
	marketType int
	limit      int
	offset     int
	order      domain.SortOrder
}

func (s *quoteRepositoryStub) FindLatestByTicker(
	context.Context,
	string,
	int,
) (domain.QuoteRecord, error) {
	return domain.QuoteRecord{}, nil
}

func (s *quoteRepositoryStub) FindByTickerAndPeriod(
	_ context.Context,
	ticker string,
	from time.Time,
	to time.Time,
	marketType int,
	limit int,
	offset int,
	order domain.SortOrder,
) (domain.QuotePage, error) {
	s.ticker = ticker
	s.from = from
	s.to = to
	s.marketType = marketType
	s.limit = limit
	s.offset = offset
	s.order = order

	return s.page, s.err
}

func TestGetQuotesByPeriodServiceUsesDefaultsAndNormalizesTicker(t *testing.T) {
	from := date(2026, time.August, 1)
	to := date(2026, time.August, 31)
	wantedPage := domain.QuotePage{
		Records: []domain.QuoteRecord{{
			Quote: domain.Quote{
				Ticker:      "PETR4",
				TradingDate: date(2026, time.August, 10),
			},
		}},
		Total: 1,
	}
	repository := &quoteRepositoryStub{page: wantedPage}
	service := NewGetQuotesByPeriodService(repository, discardLogger())

	output, err := service.Execute(context.Background(), inbound.GetQuotesByPeriodInput{
		Ticker: " petr4 ",
		From:   from,
		To:     to,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if repository.ticker != "PETR4" {
		t.Errorf("repository ticker = %q, want PETR4", repository.ticker)
	}
	if repository.marketType != domain.DefaultMarketType {
		t.Errorf("repository marketType = %d, want %d", repository.marketType, domain.DefaultMarketType)
	}
	if repository.limit != domain.DefaultLimit {
		t.Errorf("repository limit = %d, want %d", repository.limit, domain.DefaultLimit)
	}
	if repository.offset != 0 {
		t.Errorf("repository offset = %d, want 0", repository.offset)
	}
	if repository.order != domain.SortAscending {
		t.Errorf("repository order = %q, want %q", repository.order, domain.SortAscending)
	}
	if !repository.from.Equal(from) || !repository.to.Equal(to) {
		t.Errorf("repository period = %s to %s, want %s to %s", repository.from, repository.to, from, to)
	}

	if output.Ticker != "PETR4" || output.MarketType != domain.DefaultMarketType {
		t.Errorf("unexpected output metadata: %+v", output)
	}
	if output.Limit != domain.DefaultLimit || output.Offset != 0 || output.Order != domain.SortAscending {
		t.Errorf("unexpected output pagination: %+v", output)
	}
	if output.Page.Total != 1 || len(output.Page.Records) != 1 {
		t.Errorf("unexpected output page: %+v", output.Page)
	}
}

func TestGetQuotesByPeriodServiceForwardsCustomQuery(t *testing.T) {
	repository := &quoteRepositoryStub{page: domain.QuotePage{Total: 25}}
	service := NewGetQuotesByPeriodService(repository, discardLogger())

	output, err := service.Execute(context.Background(), inbound.GetQuotesByPeriodInput{
		Ticker:     "VALE3",
		From:       date(2026, time.January, 1),
		To:         date(2026, time.December, 31),
		MarketType: 20,
		Limit:      25,
		Offset:     50,
		Order:      domain.SortDescending,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if repository.marketType != 20 || repository.limit != 25 || repository.offset != 50 {
		t.Errorf("unexpected repository query: %+v", repository)
	}
	if repository.order != domain.SortDescending {
		t.Errorf("repository order = %q, want desc", repository.order)
	}
	if output.Page.Total != 25 || output.Order != domain.SortDescending {
		t.Errorf("unexpected output: %+v", output)
	}
}

func TestGetQuotesByPeriodServiceValidations(t *testing.T) {
	validFrom := date(2026, time.January, 1)
	validTo := date(2026, time.December, 31)

	tests := []struct {
		name    string
		input   inbound.GetQuotesByPeriodInput
		wantErr error
	}{
		{
			name:    "invalid ticker",
			input:   inbound.GetQuotesByPeriodInput{Ticker: "?", From: validFrom, To: validTo},
			wantErr: domain.ErrInvalidTicker,
		},
		{
			name:    "missing from",
			input:   inbound.GetQuotesByPeriodInput{Ticker: "PETR4", To: validTo},
			wantErr: domain.ErrInvalidDateRange,
		},
		{
			name:    "missing to",
			input:   inbound.GetQuotesByPeriodInput{Ticker: "PETR4", From: validFrom},
			wantErr: domain.ErrInvalidDateRange,
		},
		{
			name:    "inverted period",
			input:   inbound.GetQuotesByPeriodInput{Ticker: "PETR4", From: validTo, To: validFrom},
			wantErr: domain.ErrInvalidDateRange,
		},
		{
			name:    "period over five years",
			input:   inbound.GetQuotesByPeriodInput{Ticker: "PETR4", From: validFrom, To: validFrom.AddDate(5, 0, 1)},
			wantErr: domain.ErrInvalidDateRange,
		},
		{
			name:    "negative market type",
			input:   inbound.GetQuotesByPeriodInput{Ticker: "PETR4", From: validFrom, To: validTo, MarketType: -1},
			wantErr: domain.ErrInvalidMarketType,
		},
		{
			name:    "negative limit",
			input:   inbound.GetQuotesByPeriodInput{Ticker: "PETR4", From: validFrom, To: validTo, Limit: -1},
			wantErr: domain.ErrInvalidLimit,
		},
		{
			name:    "limit above maximum",
			input:   inbound.GetQuotesByPeriodInput{Ticker: "PETR4", From: validFrom, To: validTo, Limit: domain.MaxLimit + 1},
			wantErr: domain.ErrInvalidLimit,
		},
		{
			name:    "negative offset",
			input:   inbound.GetQuotesByPeriodInput{Ticker: "PETR4", From: validFrom, To: validTo, Offset: -1},
			wantErr: domain.ErrInvalidOffset,
		},
		{
			name:    "invalid order",
			input:   inbound.GetQuotesByPeriodInput{Ticker: "PETR4", From: validFrom, To: validTo, Order: "newest"},
			wantErr: domain.ErrInvalidOrder,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &quoteRepositoryStub{}
			service := NewGetQuotesByPeriodService(repository, discardLogger())

			_, err := service.Execute(context.Background(), test.input)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Execute() error = %v, want error wrapping %v", err, test.wantErr)
			}
			if repository.ticker != "" {
				t.Errorf("repository was called for invalid input: %+v", repository)
			}
		})
	}
}

func TestGetQuotesByPeriodServiceWrapsRepositoryError(t *testing.T) {
	repositoryError := errors.New("database unavailable")
	repository := &quoteRepositoryStub{err: repositoryError}
	service := NewGetQuotesByPeriodService(repository, discardLogger())

	_, err := service.Execute(context.Background(), inbound.GetQuotesByPeriodInput{
		Ticker: "PETR4",
		From:   date(2026, time.January, 1),
		To:     date(2026, time.January, 31),
	})
	if !errors.Is(err, repositoryError) {
		t.Fatalf("Execute() error = %v, want wrapped repository error", err)
	}
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
