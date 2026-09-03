package services

import (
	"context"
	"fmt"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type GetQuoteService struct {
	quotes outbound.QuoteRepository
}

var _ inbound.GetQuoteUseCase = (*GetQuoteService)(nil)

func NewGetQuoteService(quotes outbound.QuoteRepository) *GetQuoteService {
	return &GetQuoteService{
		quotes: quotes,
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
		return inbound.GetQuoteOutput{}, domain.ValidationError{Field: "marketType", Message: "marketType must be positive", Err: domain.ErrInvalidMarketType}
	}

	record, err := s.quotes.FindLatestByTicker(ctx, ticker, marketType)
	if err != nil {
		return inbound.GetQuoteOutput{}, fmt.Errorf("get quote %s: %w", ticker, err)
	}

	return inbound.GetQuoteOutput{
		Record: record,
	}, nil
}
