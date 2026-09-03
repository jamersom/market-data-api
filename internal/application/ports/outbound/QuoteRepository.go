package outbound

import (
	"context"
	"time"

	"github.com/jamersom/market-data-api/internal/domain"
)

type QuoteRepository interface {
	FindLatestByTicker(
		ctx context.Context,
		ticker string,
		marketType int,
	) (domain.QuoteRecord, error)

	FindByTickerAndPeriod(
		ctx context.Context,
		ticker string,
		from time.Time,
		to time.Time,
		marketType int,
		limit int,
		offset int,
		order domain.SortOrder,
	) (domain.QuotePage, error)
}
