package handlers

import (
	"net/http"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/response"
	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
)

type QuoteHandler struct {
	getQuote inbound.GetQuoteUseCase
}

func NewQuoteHandler(getQuote inbound.GetQuoteUseCase) *QuoteHandler {
	return &QuoteHandler{
		getQuote: getQuote,
	}
}

func (h *QuoteHandler) Get(w http.ResponseWriter, r *http.Request) {
	ticker := r.PathValue("ticker")
	marketType, err := optionalIntParameter(r, "marketType", 0)
	if err != nil {
		writeError(w, err)
		return
	}

	output, err := h.getQuote.Execute(
		r.Context(),
		inbound.GetQuoteInput{
			Ticker: ticker, MarketType: marketType,
		},
	)

	if err != nil {
		writeError(w, err)
		return
	}

	meta := metadataFromRecord(output.Record)
	meta.Ticker, meta.MarketType = output.Record.Quote.Ticker, output.Record.Quote.MarketType
	meta.Records, meta.TotalRecords = 1, 1
	writeJSON(w, http.StatusOK, response.QuoteEnvelope{Data: QuoteResponse(output.Record), Meta: meta})
}
