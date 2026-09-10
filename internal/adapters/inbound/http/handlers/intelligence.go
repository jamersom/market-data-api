package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type IntelligenceHandler struct {
	intelligence inbound.GetAssetIntelligenceUseCase
}

func NewIntelligenceHandler(service inbound.GetAssetIntelligenceUseCase) *IntelligenceHandler {
	return &IntelligenceHandler{intelligence: service}
}

func (h *IntelligenceHandler) Get(w http.ResponseWriter, r *http.Request) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, domain.ValidationError{Message: "malformed query string"})
		return
	}
	for name, values := range query {
		if name != "asOf" && name != "marketType" && name != "includeDetails" && name != "rsiWindow" {
			writeError(w, domain.ValidationError{Field: name, Message: "unknown query parameter"})
			return
		}
		if len(values) != 1 {
			writeError(w, domain.ValidationError{Field: name, Message: "query parameter must not be repeated"})
			return
		}
	}
	input := inbound.GetAssetIntelligenceInput{Ticker: r.PathValue("ticker")}
	if query.Has("rsiWindow") {
		var years int
		if _, err := fmt.Sscanf(query.Get("rsiWindow"), "%dy", &years); err != nil || years <= 0 || query.Get("rsiWindow") != strconv.Itoa(years)+"y" {
			writeError(w, domain.ValidationError{Field: "rsiWindow", Message: "rsiWindow must use a positive number followed by y"})
			return
		}
		input.WindowYears = years
	}
	includeDetails := false
	if query.Has("includeDetails") {
		switch query.Get("includeDetails") {
		case "true":
			includeDetails = true
		case "false":
		default:
			writeError(w, domain.ValidationError{Field: "includeDetails", Message: "includeDetails must be true or false"})
			return
		}
	}
	if query.Has("asOf") {
		input.AsOf, err = time.Parse(time.DateOnly, query.Get("asOf"))
		if err != nil || input.AsOf.IsZero() {
			writeError(w, domain.ValidationError{Field: "asOf", Message: "asOf must use YYYY-MM-DD and be nonzero", Err: domain.ErrInvalidDateRange})
			return
		}
	}
	if query.Has("marketType") {
		input.MarketType, err = strconv.Atoi(query.Get("marketType"))
		if err != nil || input.MarketType <= 0 {
			writeError(w, domain.ValidationError{Field: "marketType", Message: "marketType must be a positive integer", Err: domain.ErrInvalidMarketType})
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	output, err := h.intelligence.Execute(ctx, input)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := ctx.Err(); err != nil {
		writeError(w, err)
		return
	}
	result := IntelligenceResponse(output)
	if !includeDetails {
		result.Meta.Calendar = nil
		result.Meta.CalculationVersion = ""
		result.Meta.DataVersion = ""
		result.Meta.RSISeedFrom = ""
	}
	writeJSON(w, http.StatusOK, result)
}
