package handlers

import (
	"net/http"
	"time"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/response"
	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type QuoteHistoryHandler struct {
	getQuotes inbound.GetQuotesByPeriodUseCase
}

func NewQuoteHistoryHandler(
	getQuotes inbound.GetQuotesByPeriodUseCase,
) *QuoteHistoryHandler {
	return &QuoteHistoryHandler{
		getQuotes: getQuotes,
	}
}

func (h *QuoteHistoryHandler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	ticker := r.PathValue("ticker")

	from, err := parseDateParameter(r, "from")
	if err != nil {
		writeError(w, err)
		return
	}

	to, err := parseDateParameter(r, "to")
	if err != nil {
		writeError(w, err)
		return
	}
	marketType, err := optionalIntParameter(r, "marketType", 0)
	if err != nil {
		writeError(w, err)
		return
	}
	limit, err := optionalIntParameter(r, "limit", 0)
	if err != nil {
		writeError(w, err)
		return
	}
	offset, err := optionalIntParameter(r, "offset", 0)
	if err != nil {
		writeError(w, err)
		return
	}
	order := domain.SortOrder(r.URL.Query().Get("order"))

	output, err := h.getQuotes.Execute(
		r.Context(),
		inbound.GetQuotesByPeriodInput{
			Ticker:     ticker,
			From:       from,
			To:         to,
			MarketType: marketType, Limit: limit, Offset: offset, Order: order,
		},
	)
	if err != nil {
		writeError(w, err)
		return
	}

	meta := response.Metadata{Ticker: output.Ticker, MarketType: output.MarketType,
		From: output.From.Format(time.DateOnly), To: output.To.Format(time.DateOnly),
		Records: len(output.Page.Records), TotalRecords: output.Page.Total,
		Limit: output.Limit, Offset: output.Offset,
		HasMore: int64(output.Offset+len(output.Page.Records)) < output.Page.Total,
		Order:   string(output.Order), Source: "B3 COTAHIST", PriceAdjustment: "unadjusted"}
	if len(output.Page.Records) > 0 {
		enrichMetadata(&meta, output.Page.Records[0])
	}
	writeJSON(w, http.StatusOK, response.QuoteHistoryEnvelope{Data: QuotesResponse(output.Page.Records), Meta: meta})
}

func parseDateParameter(
	r *http.Request,
	name string,
) (time.Time, error) {
	value := r.URL.Query().Get(name)

	if value == "" {
		return time.Time{}, domain.ValidationError{
			Field:   name,
			Value:   value,
			Message: name + " is required",
			Err:     domain.ErrInvalidDateRange,
		}
	}

	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, domain.ValidationError{
			Field:   name,
			Value:   value,
			Message: name + " must use YYYY-MM-DD",
			Err:     domain.ErrInvalidDateRange,
		}
	}

	return date, nil
}
