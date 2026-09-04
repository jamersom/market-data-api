package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type GetQuotesByPeriodService struct {
	quotes outbound.QuoteRepository
	logger *slog.Logger
}

var _ inbound.GetQuotesByPeriodUseCase = (*GetQuotesByPeriodService)(nil)

func NewGetQuotesByPeriodService(quotes outbound.QuoteRepository, logger *slog.Logger) *GetQuotesByPeriodService {
	return &GetQuotesByPeriodService{
		quotes: quotes,
		logger: logger,
	}
}

func (s *GetQuotesByPeriodService) Execute(
	ctx context.Context,
	input inbound.GetQuotesByPeriodInput,
) (inbound.GetQuotesByPeriodOutput, error) {
	ticker, err := domain.NormalizeTicker(input.Ticker)
	if err != nil {
		return inbound.GetQuotesByPeriodOutput{}, err
	}

	if input.From.IsZero() {
		return inbound.GetQuotesByPeriodOutput{}, domain.ValidationError{
			Field:   "from",
			Message: "from is required and must use YYYY-MM-DD",
			Err:     domain.ErrInvalidDateRange,
		}
	}

	if input.To.IsZero() {
		return inbound.GetQuotesByPeriodOutput{}, domain.ValidationError{
			Field:   "to",
			Message: "to is required and must use YYYY-MM-DD",
			Err:     domain.ErrInvalidDateRange,
		}
	}

	if input.From.After(input.To) {
		return inbound.GetQuotesByPeriodOutput{}, domain.ValidationError{
			Field:   "dateRange",
			Value:   formatRange(input.From, input.To),
			Message: "from must be before or equal to to",
			Err:     domain.ErrInvalidDateRange,
		}
	}
	if input.To.After(input.From.AddDate(5, 0, 0)) {
		return inbound.GetQuotesByPeriodOutput{}, domain.ValidationError{
			Field: "dateRange", Value: formatRange(input.From, input.To),
			Message: "date range cannot exceed five years", Err: domain.ErrInvalidDateRange,
		}
	}
	marketType := input.MarketType
	if marketType == 0 {
		marketType = domain.DefaultMarketType
	}
	if marketType < 0 {
		return inbound.GetQuotesByPeriodOutput{}, domain.ValidationError{
			Field:   "marketType",
			Message: "marketType must be positive",
			Err:     domain.ErrInvalidMarketType,
		}
	}
	limit := input.Limit
	if limit == 0 {
		limit = domain.DefaultLimit
	}
	if limit < 1 || limit > domain.MaxLimit {
		return inbound.GetQuotesByPeriodOutput{}, domain.ValidationError{
			Field:   "limit",
			Message: "limit must be between 1 and 1000",
			Err:     domain.ErrInvalidLimit,
		}
	}
	if input.Offset < 0 {
		return inbound.GetQuotesByPeriodOutput{}, domain.ValidationError{
			Field:   "offset",
			Message: "offset cannot be negative",
			Err:     domain.ErrInvalidOffset,
		}
	}
	order := input.Order
	if order == "" {
		order = domain.SortAscending
	}
	if order != domain.SortAscending && order != domain.SortDescending {
		return inbound.GetQuotesByPeriodOutput{}, domain.ValidationError{
			Field:   "order",
			Value:   string(order),
			Message: "order must be asc or desc",
			Err:     domain.ErrInvalidOrder,
		}
	}

	s.logger.DebugContext(
		ctx,
		"getting quotes by period",
		"ticker", ticker,
		"from", input.From.Format(time.DateOnly),
		"to", input.To.Format(time.DateOnly),
		"market_type", marketType,
		"limit", limit,
		"offset", input.Offset,
		"order", order,
	)

	page, err := s.quotes.FindByTickerAndPeriod(
		ctx,
		ticker,
		input.From,
		input.To,
		marketType,
		limit,
		input.Offset,
		order,
	)
	if err != nil {
		return inbound.GetQuotesByPeriodOutput{}, fmt.Errorf(
			"get quotes for %s between %s and %s: %w",
			ticker,
			input.From.Format(time.DateOnly),
			input.To.Format(time.DateOnly),
			err,
		)
	}

	s.logger.DebugContext(
		ctx,
		"quotes by period retrieved",
		"ticker", ticker,
		"returned_records", len(page.Records),
		"total_records", page.Total,
	)

	return inbound.GetQuotesByPeriodOutput{
		Page:       page,
		Ticker:     ticker,
		From:       input.From,
		To:         input.To,
		MarketType: marketType,
		Limit:      limit,
		Offset:     input.Offset,
		Order:      order,
	}, nil
}

func formatRange(from, to time.Time) string {
	return from.Format(time.DateOnly) +
		"," +
		to.Format(time.DateOnly)
}
