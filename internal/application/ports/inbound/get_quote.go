package inbound

import (
	"context"

	"github.com/jamersom/market-data-api/internal/domain"
)

type GetQuoteInput struct {
	Ticker     string
	MarketType int
}

type GetQuoteOutput struct {
	Record domain.QuoteRecord
}

type GetQuoteUseCase interface {
	Execute(
		ctx context.Context,
		input GetQuoteInput,
	) (GetQuoteOutput, error)
}
