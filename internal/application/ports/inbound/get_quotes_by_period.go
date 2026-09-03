package inbound

import (
	"context"
	"time"

	"github.com/jamersom/market-data-api/internal/domain"
)

type GetQuotesByPeriodInput struct {
	Ticker     string
	From       time.Time
	To         time.Time
	MarketType int
	Limit      int
	Offset     int
	Order      domain.SortOrder
}

type GetQuotesByPeriodOutput struct {
	Page       domain.QuotePage
	Ticker     string
	From       time.Time
	To         time.Time
	MarketType int
	Limit      int
	Offset     int
	Order      domain.SortOrder
}

type GetQuotesByPeriodUseCase interface {
	Execute(
		ctx context.Context,
		input GetQuotesByPeriodInput,
	) (GetQuotesByPeriodOutput, error)
}
