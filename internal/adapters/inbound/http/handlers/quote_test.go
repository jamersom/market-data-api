package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/response"
	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type getQuoteStub struct {
	output inbound.GetQuoteOutput
	err    error
}

func (s getQuoteStub) Execute(context.Context, inbound.GetQuoteInput) (inbound.GetQuoteOutput, error) {
	return s.output, s.err
}

func TestQuoteHandlerGet(t *testing.T) {
	handler := NewQuoteHandler(getQuoteStub{output: inbound.GetQuoteOutput{Record: domain.QuoteRecord{Quote: domain.Quote{
		Ticker:            "PETR4",
		TradingDate:       time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC),
		Currency:          "BRL",
		ClosePriceCents:   3245,
		BestBidPriceCents: 3244,
		BestAskPriceCents: 3246,
	}}}})

	recorder := serveRequest(handler)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var body response.QuoteEnvelope
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Ticker != "PETR4" || body.Data.ClosePrice != "32.45" || body.Data.BestBidPrice != "32.44" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestFormatCents(t *testing.T) {
	tests := map[int64]string{
		4820: "48.20",
		4500: "45.00",
		1:    "0.01",
		0:    "0.00",
		-133: "-1.33",
	}
	for input, expected := range tests {
		if actual := formatCents(input); actual != expected {
			t.Errorf("formatCents(%d) = %q, want %q", input, actual, expected)
		}
	}
}

func TestQuoteHandlerValidationError(t *testing.T) {
	handler := NewQuoteHandler(getQuoteStub{err: domain.ValidationError{
		Field: "ticker", Value: "?", Message: "invalid ticker", Err: domain.ErrInvalidTicker,
	}})

	recorder := serveRequest(handler)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestQuoteHandlerNotFound(t *testing.T) {
	handler := NewQuoteHandler(getQuoteStub{err: domain.ResourceNotFoundError{
		Resource: "quote", Ticker: "PETR4",
	}})

	recorder := serveRequest(handler)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}

	var body response.Error
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "quote_not_found" {
		t.Fatalf("unexpected error response: %+v", body)
	}
}

func serveRequest(handler *QuoteHandler) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/quotes/PETR4", nil)
	request.SetPathValue("ticker", "PETR4")
	recorder := httptest.NewRecorder()
	handler.Get(recorder, request)
	return recorder
}
