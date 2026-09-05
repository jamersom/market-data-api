package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/response"
	"github.com/jamersom/market-data-api/internal/domain"
)

func writeError(w http.ResponseWriter, err error) {
	var validationErr domain.ValidationError

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		writeJSON(w, http.StatusGatewayTimeout, response.Error{Error: response.ErrorDetail{
			Code: "request_timeout", Message: "request timed out", Retryable: true,
		}})
	case errors.Is(err, domain.ErrInsufficientComparisonData):
		writeJSON(w, http.StatusUnprocessableEntity, response.Error{Error: response.ErrorDetail{
			Code: "insufficient_comparison_data", Message: "at least two assets must have sufficient data", Retryable: false,
		}})
	case errors.As(err, &validationErr):
		code := validationCode(validationErr.Err)
		expectedFormat := ""
		if errors.Is(validationErr.Err, domain.ErrInvalidDateRange) {
			expectedFormat = "YYYY-MM-DD"
		}
		writeJSON(w, http.StatusBadRequest, response.Error{Error: response.ErrorDetail{
			Code:           code,
			Message:        validationErr.Message,
			Field:          validationErr.Field,
			Value:          validationErr.Value,
			ExpectedFormat: expectedFormat,
			Retryable:      false,
		}})

	case errors.Is(err, domain.ErrQuoteNotFound):
		writeJSON(w, http.StatusNotFound, response.Error{Error: response.ErrorDetail{
			Code:      "quote_not_found",
			Message:   err.Error(),
			Retryable: false,
		}})

	default:
		// TODO: registrar o erro interno em um logger estruturado.
		writeJSON(w, http.StatusInternalServerError, response.Error{Error: response.ErrorDetail{
			Code:      "internal_error",
			Message:   "an unexpected error occurred",
			Retryable: true,
		}})
	}
}

func validationCode(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidTickers):
		return "invalid_tickers"
	case errors.Is(err, domain.ErrInvalidMetric):
		return "invalid_metric"
	case errors.Is(err, domain.ErrInvalidTicker):
		return "invalid_ticker"
	case errors.Is(err, domain.ErrInvalidDateRange):
		return "invalid_date_range"
	case errors.Is(err, domain.ErrInvalidMarketType):
		return "invalid_market_type"
	case errors.Is(err, domain.ErrInvalidLimit):
		return "invalid_limit"
	case errors.Is(err, domain.ErrInvalidOffset):
		return "invalid_offset"
	case errors.Is(err, domain.ErrInvalidOrder):
		return "invalid_order"
	default:
		return "invalid_request"
	}
}
