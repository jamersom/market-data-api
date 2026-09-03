package http

import (
	"net/http"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/handlers"
)

func RegisterRoutes(
	mux *http.ServeMux,
	quoteHandler *handlers.QuoteHandler,
	quoteHistoryHandler *handlers.QuoteHistoryHandler,
) {
	mux.HandleFunc("GET /quotes/{ticker}", quoteHandler.Get)
	mux.HandleFunc("GET /quotes/{ticker}/history", quoteHistoryHandler.Get)
}
