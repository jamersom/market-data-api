package outbound

import (
	"context"
	"time"

	"github.com/jamersom/market-data-api/internal/domain"
)

// ComparisonQuoteRepository consulta históricos diários completos em lote, sem
// paginação. Tickers sem dados não possuem registros. Cada ticker tem no máximo
// um registro por data de pregão, no mercado solicitado e incluindo as datas
// inicial e final do período.
type ComparisonQuoteRepository interface {
	FindByTickersAndPeriod(ctx context.Context, tickers []string, from, to time.Time, marketType int) (map[string][]domain.QuoteRecord, error)
}
