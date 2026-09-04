package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type GetQuoteService struct {
	quotes outbound.QuoteRepository
	logger *slog.Logger
}

var _ inbound.GetQuoteUseCase = (*GetQuoteService)(nil)

func NewGetQuoteService(quotes outbound.QuoteRepository, logger *slog.Logger) *GetQuoteService {
	return &GetQuoteService{
		quotes: quotes,
		logger: logger,
	}
}

func (s *GetQuoteService) Execute(ctx context.Context, input inbound.GetQuoteInput) (inbound.GetQuoteOutput, error) {
	ticker, err := domain.NormalizeTicker(input.Ticker)
	if err != nil {
		return inbound.GetQuoteOutput{}, err
	}

	marketType := input.MarketType
	if marketType == 0 {
		marketType = domain.DefaultMarketType
	}
	if marketType < 0 {
		return inbound.GetQuoteOutput{}, domain.ValidationError{
			Field:   "marketType",
			Message: "marketType must be positive",
			Err:     domain.ErrInvalidMarketType,
		}
	}

	s.logger.DebugContext(
		ctx,
		"getting latest quote",
		"ticker", ticker,
		"market_type", marketType,
	)

	record, err := s.quotes.FindLatestByTicker(ctx, ticker, marketType)
	if err != nil {
		return inbound.GetQuoteOutput{}, fmt.Errorf("get quote %s: %w", ticker, err)
	}

	s.logger.DebugContext(
		ctx,
		"latest quote retrieved",
		"ticker", record.Quote.Ticker,
		"market_type", record.Quote.MarketType,
		"trading_date", record.Quote.TradingDate,
	)

	return inbound.GetQuoteOutput{
		Record: record,
	}, nil
}
