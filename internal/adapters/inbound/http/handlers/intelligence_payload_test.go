package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/domain"
)

type payloadStub struct{ calls int }

func (s *payloadStub) Execute(context.Context, inbound.GetAssetIntelligenceInput) (inbound.GetAssetIntelligenceOutput, error) {
	s.calls++
	zero := 0.0
	return inbound.GetAssetIntelligenceOutput{Ticker: "PETR4", Status: "partial", Return7D: &zero,
		Calendar:    domain.IntelligenceCalendarMetadata{Source: "cotahist_observed", Coverage: []domain.CalendarCoverage{{Year: 2023, From: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC), To: time.Date(2023, 12, 28, 0, 0, 0, 0, time.UTC)}}},
		Unavailable: []inbound.IntelligenceUnavailable{{Field: "momentum.rsi14", Reason: "insufficient_history"}}}, nil
}

func TestIntelligenceCompactPayload(t *testing.T) {
	for _, query := range []string{"", "?includeDetails=false", "?includeDetails=true"} {
		t.Run(query, func(t *testing.T) {
			stub := &payloadStub{}
			mux := http.NewServeMux()
			mux.HandleFunc("GET /assets/{ticker}/intelligence", NewIntelligenceHandler(stub).Get)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/assets/PETR4/intelligence"+query, nil))
			if rec.Code != 200 {
				t.Fatalf("HTTP %d", rec.Code)
			}
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			data := body["data"].(map[string]any)
			meta := body["meta"].(map[string]any)
			for _, key := range []string{"benchmark", "relative_strength", "market_context", "score", "signals", "alerts"} {
				if _, ok := data[key]; ok {
					t.Fatalf("placeholder %s still present", key)
				}
			}
			momentum := data["momentum"].(map[string]any)
			if _, ok := momentum["rsi14_percentile_3y"]; ok {
				t.Fatal("percentile placeholder present")
			}
			if value, ok := momentum["rsi14"]; !ok || value != nil {
				t.Fatal("unavailable metric must remain null")
			}
			if data["returns"].(map[string]any)["return_7d"] != float64(0) {
				t.Fatal("valid zero lost")
			}
			if _, ok := meta["rules_version"]; ok {
				t.Fatal("rules version placeholder present")
			}
			_, hasCoverage := meta["calendar"].(map[string]any)["coverage"]
			if hasCoverage != (query == "?includeDetails=true") {
				t.Fatal("details option mismatch")
			}
		})
	}
}

func TestIntelligenceDetailsValidation(t *testing.T) {
	for _, query := range []string{"?includeDetails=", "?includeDetails=1", "?includeDetails=yes", "?includeDetails=true&includeDetails=false"} {
		stub := &payloadStub{}
		rec := httptest.NewRecorder()
		NewIntelligenceHandler(stub).Get(rec, httptest.NewRequest("GET", "/assets/PETR4/intelligence"+query, nil))
		if rec.Code != 400 || stub.calls != 0 {
			t.Fatalf("invalid details accepted: %s", query)
		}
	}
}
