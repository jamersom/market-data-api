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

type getQuotesByPeriodStub struct {
	input  inbound.GetQuotesByPeriodInput
	output inbound.GetQuotesByPeriodOutput
	err    error
}

func (s *getQuotesByPeriodStub) Execute(
	_ context.Context,
	input inbound.GetQuotesByPeriodInput,
) (inbound.GetQuotesByPeriodOutput, error) {
	s.input = input
	return s.output, s.err
}

func TestQuoteHistoryHandlerGet(t *testing.T) {
	stub := &getQuotesByPeriodStub{output: inbound.GetQuotesByPeriodOutput{
		Page: domain.QuotePage{Records: []domain.QuoteRecord{{Quote: domain.Quote{
			Ticker:          "PETR4",
			TradingDate:     time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC),
			Currency:        "BRL",
			ClosePriceCents: 3245,
		}}}, Total: 1}, Ticker: "PETR4", MarketType: 10,
		From: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		Limit: 100, Order: domain.SortAscending,
	}}
	handler := NewQuoteHistoryHandler(stub)
	request := httptest.NewRequest(
		http.MethodGet,
		"/quotes/PETR4/history?from=2026-08-01&to=2026-08-31",
		nil,
	)
	request.SetPathValue("ticker", "PETR4")
	recorder := httptest.NewRecorder()

	handler.Get(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	if stub.input.Ticker != "PETR4" ||
		stub.input.From.Format(time.DateOnly) != "2026-08-01" ||
		stub.input.To.Format(time.DateOnly) != "2026-08-31" {
		t.Fatalf("unexpected input: %+v", stub.input)
	}

	var body response.QuoteHistoryEnvelope
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].Ticker != "PETR4" || body.Data[0].ClosePrice != "32.45" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestQuoteHistoryHandlerMissingTo(t *testing.T) {
	handler := NewQuoteHistoryHandler(&getQuotesByPeriodStub{})
	request := httptest.NewRequest(
		http.MethodGet,
		"/quotes/PETR4/history?from=2026-08-01",
		nil,
	)
	request.SetPathValue("ticker", "PETR4")
	recorder := httptest.NewRecorder()

	handler.Get(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestQuoteHistoryHandlerInvalidDate(t *testing.T) {
	handler := NewQuoteHistoryHandler(&getQuotesByPeriodStub{})
	request := httptest.NewRequest(
		http.MethodGet,
		"/quotes/PETR4/history?from=01-08-2026&to=2026-08-31",
		nil,
	)
	request.SetPathValue("ticker", "PETR4")
	recorder := httptest.NewRecorder()

	handler.Get(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}
