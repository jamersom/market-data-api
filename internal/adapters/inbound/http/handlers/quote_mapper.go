package handlers

import (
	"fmt"
	"math"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/response"
	"github.com/jamersom/market-data-api/internal/domain"
)

func QuoteResponse(record domain.QuoteRecord) response.Quote {
	quote := record.Quote
	result := response.Quote{
		Ticker: quote.Ticker, TradingDate: quote.TradingDate.Format("2006-01-02"),
		BDICode: quote.BDICode, MarketType: quote.MarketType,
		MarketTypeDescription: marketTypeDescription(quote.MarketType), ShortName: quote.ShortName,
		Specification: quote.Specification, Term: quote.Term, Currency: quote.Currency,
		OpenPrice: formatCents(quote.OpenPriceCents), HighPrice: formatCents(quote.HighPriceCents),
		LowPrice: formatCents(quote.LowPriceCents), AveragePrice: formatCents(quote.AveragePriceCents),
		ClosePrice:    formatCents(quote.ClosePriceCents),
		IntradayRange: formatCents(quote.HighPriceCents - quote.LowPriceCents),
		BestBidPrice:  formatCents(quote.BestBidPriceCents), BestAskPrice: formatCents(quote.BestAskPriceCents),
		BidAskSpread: formatCents(quote.BestAskPriceCents - quote.BestBidPriceCents),
		TradeCount:   quote.TradeCount, TradedQuantity: quote.TradedQuantity,
		TradedVolume: formatCents(quote.TradedVolumeCents), QuoteFactor: quote.QuoteFactor,
		ISIN: quote.ISIN, DistributionNumber: quote.DistributionNumber,
	}
	if record.PreviousCloseCents != nil {
		previous := formatCents(*record.PreviousCloseCents)
		absolute := formatCents(quote.ClosePriceCents - *record.PreviousCloseCents)
		result.PreviousClose = &previous
		result.AbsoluteChange = &absolute
		if *record.PreviousCloseCents != 0 {
			percentage := round(float64(quote.ClosePriceCents-*record.PreviousCloseCents) / float64(*record.PreviousCloseCents) * 100)
			result.PercentageChange = &percentage
		}
	}
	return result
}

func QuotesResponse(records []domain.QuoteRecord) []response.Quote {
	result := make([]response.Quote, 0, len(records))
	for _, record := range records {
		result = append(result, QuoteResponse(record))
	}
	return result
}

func formatCents(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	return fmt.Sprintf("%s%d.%02d", sign, value/100, value%100)
}
func round(value float64) float64 { return math.Round(value*100) / 100 }
func marketTypeDescription(value int) string {
	switch value {
	case 10:
		return "spot"
	case 12:
		return "call_option_exercise"
	case 13:
		return "put_option_exercise"
	case 17:
		return "auction"
	case 20:
		return "fractional"
	case 30:
		return "forward"
	case 50:
		return "futures_with_gains_retention"
	case 60:
		return "futures_continuous"
	case 70:
		return "call_option"
	case 80:
		return "put_option"
	default:
		return "unknown"
	}
}
