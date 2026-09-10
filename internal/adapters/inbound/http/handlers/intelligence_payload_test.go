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

type payloadStub struct {
	calls int
	input inbound.GetAssetIntelligenceInput
}

func (s *payloadStub) Execute(_ context.Context, input inbound.GetAssetIntelligenceInput) (inbound.GetAssetIntelligenceOutput, error) {
	s.calls++
	s.input = input
	zero := 0.0
	out := inbound.GetAssetIntelligenceOutput{Ticker: "PETR4", Status: "partial", Return7D: &zero,
		AsOf: time.Date(2023, 12, 28, 0, 0, 0, 0, time.UTC),
		Calendar: domain.IntelligenceCalendarMetadata{Source: "cotahist_observed", Version: "calendar-v1", Policy: "observed_import_integrity_v1",
			Coverage: []domain.CalendarCoverage{{Year: 2023, From: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC), To: time.Date(2023, 12, 28, 0, 0, 0, 0, time.UTC)}}},
		CalculationVersion: "1.1", DataVersion: "data-v1", RSISeedFrom: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
		RSIPercentile: &inbound.RSIPercentile{Value: 91.4, WindowYears: 5, Observations: 1034,
			CoverageFrom: time.Date(2022, 7, 18, 0, 0, 0, 0, time.UTC), CoverageTo: time.Date(2023, 12, 27, 0, 0, 0, 0, time.UTC)},
		Unavailable: []inbound.IntelligenceUnavailable{{Field: "momentum.rsi14", Reason: "insufficient_history"}}}
	if !input.AsOf.IsZero() {
		requested := input.AsOf
		out.RequestedAsOf = &requested
	}
	return out, nil
}

func TestIntelligenceCompactPayload(t *testing.T) {
	for _, tc := range []struct {
		name, query            string
		details, requestedAsOf bool
	}{
		{name: "default"},
		{name: "details false", query: "?includeDetails=false"},
		{name: "details true", query: "?includeDetails=true", details: true},
		{name: "asOf compact", query: "?asOf=2023-12-28", requestedAsOf: true},
		{name: "asOf details", query: "?asOf=2023-12-28&includeDetails=true", details: true, requestedAsOf: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &payloadStub{}
			mux := http.NewServeMux()
			mux.HandleFunc("GET /assets/{ticker}/intelligence", NewIntelligenceHandler(stub).Get)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/assets/PETR4/intelligence"+tc.query, nil))
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
			percentile, ok := momentum["rsi14_percentile"].(map[string]any)
			if !ok || percentile["window"] != "5y" || percentile["observations"] != float64(1034) {
				t.Fatalf("percentile missing or invalid: %v", momentum["rsi14_percentile"])
			}
			coverage, ok := percentile["coverage"].(map[string]any)
			if !ok || coverage["from"] != "2022-07-18" || coverage["to"] != "2023-12-27" || coverage["complete"] != false {
				t.Fatalf("coverage missing or invalid: %v", percentile["coverage"])
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
			for _, key := range []string{"calendar", "calculation_version", "data_version", "rsi_seed_from"} {
				if _, ok := meta[key]; ok != tc.details {
					t.Fatalf("field %s presence does not match includeDetails", key)
				}
			}
			if tc.details {
				calendar := meta["calendar"].(map[string]any)
				if _, ok := calendar["coverage"]; !ok {
					t.Fatal("calendar coverage missing from detailed response")
				}
			}
			requested, ok := meta["requested_as_of"]
			if ok != tc.requestedAsOf {
				t.Fatal("requested_as_of presence does not match asOf")
			}
			if tc.requestedAsOf && requested != "2023-12-28" {
				t.Fatalf("unexpected requested_as_of: %v", requested)
			}
		})
	}
}

func TestIntelligenceRSIWindow(t *testing.T) {
	for _, tc := range []struct {
		query string
		years int
		code  int
	}{
		{"", 0, 200},
		{"?rsiWindow=1y", 1, 200},
		{"?rsiWindow=3y", 3, 200},
		{"?rsiWindow=5y", 5, 200},
		{"?rsiWindow=0y", 0, 400},
		{"?rsiWindow=", 0, 400},
		{"?rsiWindow=5", 0, 400},
		{"?rsiWindow=05y", 0, 400},
		{"?rsiWindow=1y&rsiWindow=3y", 0, 400},
	} {
		stub := &payloadStub{}
		rec := httptest.NewRecorder()
		NewIntelligenceHandler(stub).Get(rec, httptest.NewRequest("GET", "/assets/PETR4/intelligence"+tc.query, nil))
		if rec.Code != tc.code || stub.input.WindowYears != tc.years {
			t.Fatalf("query %q: HTTP %d, window %d", tc.query, rec.Code, stub.input.WindowYears)
		}
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
