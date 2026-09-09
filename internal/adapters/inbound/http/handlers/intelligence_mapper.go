package handlers

import (
	"fmt"
	"time"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/response"
	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
)

func IntelligenceResponse(out inbound.GetAssetIntelligenceOutput) response.IntelligenceEnvelope {
	result := response.IntelligenceEnvelope{}
	d := &result.Data
	d.Ticker = out.Ticker
	d.Price.Currency = out.Price.Currency
	if d.Price.Currency == "" {
		d.Price.Currency = "BRL"
	}
	blocked := make(map[string]bool, len(out.Unavailable))
	result.Meta.Unavailable = make([]response.IntelligenceUnavailable, 0, len(out.Unavailable))
	for _, entry := range out.Unavailable {
		blocked[entry.Field] = true
		result.Meta.Unavailable = append(result.Meta.Unavailable, response.IntelligenceUnavailable{Field: entry.Field, Reason: entry.Reason})
	}
	if !blocked["price.close"] && out.Price.ClosePriceCents > 0 {
		value := formatCents(out.Price.ClosePriceCents)
		d.Price.Close = &value
	}
	metric := func(field string, value *float64) *float64 {
		if value == nil || blocked[field] {
			return nil
		}
		v := round(*value)
		return &v
	}
	d.Returns.Return7D = metric("returns.return_7d", out.Return7D)
	if out.SMA20Cents != nil && !blocked["trend.sma20"] {
		v := fmt.Sprintf("%.2f", *out.SMA20Cents/100)
		d.Trend.SMA20 = &v
	}
	d.Trend.DistanceSMA20 = metric("trend.distance_sma20", out.DistanceSMA20)
	d.Momentum.RSI14 = metric("momentum.rsi14", out.RSI14)
	d.Risk.Volatility30D = metric("risk.volatility_30d", out.Volatility30D)
	d.Risk.DrawdownCurrent = metric("risk.drawdown_current", out.DrawdownCurrent)
	d.Risk.MaximumDrawdown252D = metric("risk.maximum_drawdown_252d", out.MaximumDrawdown252D)
	if out.AverageDailyVolume20DCents != nil && !blocked["liquidity.average_daily_volume_20d"] {
		v := formatCents(*out.AverageDailyVolume20DCents)
		d.Liquidity.AverageDailyVolume20D = &v
	}
	m := &result.Meta
	m.Calendar = &response.IntelligenceCalendar{Source: out.Calendar.Source, Version: out.Calendar.Version, Policy: out.Calendar.Policy, OfficialVerified: out.Calendar.OfficialVerified, Coverage: []response.IntelligenceCalendarCoverage{}}
	for _, c := range out.Calendar.Coverage {
		m.Calendar.Coverage = append(m.Calendar.Coverage, response.IntelligenceCalendarCoverage{Year: c.Year, From: c.From.Format(time.DateOnly), To: c.To.Format(time.DateOnly), IntegrityValidated: c.IntegrityValidated, Version: c.Version})
	}
	m.Status, m.MarketType = out.Status, out.MarketType
	m.AsOf = out.AsOf.Format(time.DateOnly)
	if out.RequestedAsOf != nil {
		v := out.RequestedAsOf.Format(time.DateOnly)
		m.RequestedAsOf = &v
	}
	m.Source, m.PriceAdjustment = out.Source, out.PriceAdjustment
	m.WindowUnit, m.PercentageUnit = out.WindowUnit, out.PercentageUnit
	m.CalculationVersion, m.DataVersion = out.CalculationVersion, out.DataVersion
	m.RSISeedFrom = out.RSISeedFrom.Format(time.DateOnly)
	return result
}
