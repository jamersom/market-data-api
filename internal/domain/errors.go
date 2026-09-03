package domain

import (
	"errors"
	"fmt"
)

var (
	ErrQuoteNotFound  = errors.New("quote not found")
	ErrTradeNotFound  = errors.New("trade not found")
	ErrCandleNotFound = errors.New("candle not found")

	ErrInvalidTicker     = errors.New("invalid ticker")
	ErrInvalidDateRange  = errors.New("invalid date range")
	ErrInvalidTimeframe  = errors.New("invalid timeframe")
	ErrInvalidLimit      = errors.New("invalid limit")
	ErrInvalidOffset     = errors.New("invalid offset")
	ErrInvalidOrder      = errors.New("invalid order")
	ErrInvalidMarketType = errors.New("invalid market type")
)

type ValidationError struct {
	Field   string
	Value   string
	Message string
	Err     error
}

func (e ValidationError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf(
			"validation failed for field %q with value %q",
			e.Field,
			e.Value,
		)
	}

	return fmt.Sprintf(
		"validation failed for field %q with value %q: %s",
		e.Field,
		e.Value,
		e.Message,
	)
}

func (e ValidationError) Unwrap() error {
	return e.Err
}

type ResourceNotFoundError struct {
	Resource string
	Ticker   string
}

func (e ResourceNotFoundError) Error() string {
	return fmt.Sprintf("%s not found for ticker %s", e.Resource, e.Ticker)
}

func (e ResourceNotFoundError) Unwrap() error {
	switch e.Resource {
	case "quote":
		return ErrQuoteNotFound
	case "trade":
		return ErrTradeNotFound
	case "candle":
		return ErrCandleNotFound
	default:
		return nil
	}
}
