package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/application/ports/outbound"
	"github.com/jamersom/market-data-api/internal/application/services"
	"github.com/jamersom/market-data-api/internal/domain"
	"github.com/jamersom/market-data-api/internal/domain/signals"
)

type payloadStub struct {
	calls        int
	input        inbound.GetAssetIntelligenceInput
	signalsInput *signals.Indicators
}

type unusedIntelligenceRepository struct{}

func (unusedIntelligenceRepository) FindIntelligenceHistories(context.Context, []string, int, time.Time) (map[string]outbound.IntelligenceHistory, error) {
	panic("repository must not be called for invalid benchmark")
}

func (s *payloadStub) Execute(_ context.Context, input inbound.GetAssetIntelligenceInput) (inbound.GetAssetIntelligenceOutput, error) {
	s.calls++
	s.input = input
	zero := 0.0
	price, sma20, sma50, rsi := 10000.0, 9000.0, 9500.0, 70.0
	evaluator := signals.NewEvaluator(signals.RulesetV1())
	signalsInput := signals.Indicators{PriceCents: &price, RSI14: &rsi, SMA20Cents: &sma20, SMA50Cents: &sma50}
	if s.signalsInput != nil {
		signalsInput = *s.signalsInput
	}
	out := inbound.GetAssetIntelligenceOutput{Ticker: "PETR4", Status: "partial", Return7D: &zero, SMA20Cents: &sma20, SMA50Cents: &sma50,
		AsOf: time.Date(2023, 12, 28, 0, 0, 0, 0, time.UTC),
		Calendar: domain.IntelligenceCalendarMetadata{Source: "cotahist_observed", Version: "calendar-v1", Policy: "observed_import_integrity_v1",
			Coverage: []domain.CalendarCoverage{{Year: 2023, From: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC), To: time.Date(2023, 12, 28, 0, 0, 0, 0, time.UTC)}}},
		CalculationVersion: "1.2", RulesetVersion: evaluator.Version(), DataVersion: "data-v1", RSISeedFrom: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
		Signals: evaluator.Evaluate(signalsInput),
		RSIPercentile: &inbound.RSIPercentile{Value: 91.4, WindowYears: 5, Observations: 1034,
			CoverageFrom: time.Date(2022, 7, 18, 0, 0, 0, 0, time.UTC), CoverageTo: time.Date(2023, 12, 27, 0, 0, 0, 0, time.UTC)},
		Unavailable: []inbound.IntelligenceUnavailable{{Field: "momentum.rsi14", Reason: "insufficient_history"}}}
	if !input.AsOf.IsZero() {
		requested := input.AsOf
		out.RequestedAsOf = &requested
	}
	if input.Benchmark != "" {
		out.Benchmark = &inbound.IntelligenceBenchmark{Ticker: input.Benchmark}
		if input.Benchmark == "NONE" {
			out.Unavailable = append(out.Unavailable,
				inbound.IntelligenceUnavailable{Field: "benchmark.returns.return_7d", Reason: "benchmark_not_found"},
				inbound.IntelligenceUnavailable{Field: "benchmark.relative_strength.return_7d_pp", Reason: "benchmark_not_found"},
			)
		} else {
			benchmarkReturn, relativeStrength := 7.2, 5.3
			out.Benchmark.Ticker = "IBOV"
			out.Benchmark.Return7D = &benchmarkReturn
			out.Benchmark.RelativeStrengthReturn7DPP = &relativeStrength
		}
	}
	return out, nil
}

func TestIntelligenceUnavailableSignalsPayload(t *testing.T) {
	price, sma20 := 10000.0, 9000.0
	stub := &payloadStub{signalsInput: &signals.Indicators{PriceCents: &price, SMA20Cents: &sma20}}
	rec := httptest.NewRecorder()
	NewIntelligenceHandler(stub).Get(rec, httptest.NewRequest("GET", "/assets/PETR4/intelligence", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	evaluations := body["data"].(map[string]any)["signals"].([]any)
	statuses := make(map[string]map[string]any, len(evaluations))
	for _, value := range evaluations {
		evaluation := value.(map[string]any)
		statuses[evaluation["id"].(string)] = evaluation
	}
	if statuses["rsi_overbought"]["status"] != "unavailable" {
		t.Fatalf("RSI signal should be unavailable: %+v", statuses["rsi_overbought"])
	}
	if _, exists := statuses["rsi_overbought"]["evidence"]; exists {
		t.Fatalf("unavailable signal should omit evidence: %+v", statuses["rsi_overbought"])
	}
	if statuses["price_above_sma20"]["status"] != "triggered" {
		t.Fatalf("independent signal was not evaluated: %+v", statuses["price_above_sma20"])
	}
	evidence := statuses["price_above_sma20"]["evidence"].(map[string]any)
	if evidence["price"] != float64(100) || evidence["sma20"] != float64(90) {
		t.Fatalf("monetary evidence was not converted to reais: %+v", evidence)
	}
}

func TestIntelligenceUnavailableBenchmarkPayload(t *testing.T) {
	stub := &payloadStub{}
	rec := httptest.NewRecorder()
	NewIntelligenceHandler(stub).Get(rec, httptest.NewRequest("GET", "/assets/PETR4/intelligence?benchmark=NONE", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	benchmark := body["data"].(map[string]any)["benchmark"].(map[string]any)
	if benchmark["returns"].(map[string]any)["return_7d"] != nil || benchmark["relative_strength"].(map[string]any)["return_7d_pp"] != nil {
		t.Fatalf("unavailable benchmark values: %+v", benchmark)
	}
	unavailable := body["meta"].(map[string]any)["unavailable"].([]any)
	if len(unavailable) != 3 {
		t.Fatalf("unavailable entries: %+v", unavailable)
	}
}

func TestIntelligenceBenchmarkPayload(t *testing.T) {
	stub := &payloadStub{}
	rec := httptest.NewRecorder()
	NewIntelligenceHandler(stub).Get(rec, httptest.NewRequest("GET", "/assets/PETR4/intelligence?benchmark=IBOV", nil))
	if rec.Code != http.StatusOK || stub.input.Benchmark != "IBOV" {
		t.Fatalf("HTTP %d, input %+v", rec.Code, stub.input)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	benchmark := body["data"].(map[string]any)["benchmark"].(map[string]any)
	if benchmark["ticker"] != "IBOV" || benchmark["returns"].(map[string]any)["return_7d"] != 7.2 || benchmark["relative_strength"].(map[string]any)["return_7d_pp"] != 5.3 {
		t.Fatalf("benchmark payload: %+v", benchmark)
	}
}

func TestIntelligenceBenchmarkValidation(t *testing.T) {
	for _, query := range []string{"?benchmark=", "?benchmark=IBOV&benchmark=BOVA11"} {
		stub := &payloadStub{}
		rec := httptest.NewRecorder()
		NewIntelligenceHandler(stub).Get(rec, httptest.NewRequest("GET", "/assets/PETR4/intelligence"+query, nil))
		if rec.Code != http.StatusBadRequest || stub.calls != 0 {
			t.Fatalf("invalid benchmark accepted: %s", query)
		}
	}
}

func TestIntelligenceRejectsBenchmarkEqualToAsset(t *testing.T) {
	handler := NewIntelligenceHandler(services.NewGetAssetIntelligenceService(unusedIntelligenceRepository{}, nil))
	rec := httptest.NewRecorder()
	handler.Get(rec, httptest.NewRequest("GET", "/assets/PETR4/intelligence?benchmark=petr4", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("HTTP %d, want 400", rec.Code)
	}
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
			for _, key := range []string{"benchmark", "relative_strength", "market_context", "score", "alerts"} {
				if _, ok := data[key]; ok {
					t.Fatalf("placeholder %s still present", key)
				}
			}
			signalValues, ok := data["signals"].([]any)
			if !ok || len(signalValues) != 6 {
				t.Fatalf("signals missing: %+v", data["signals"])
			}
			firstSignal := signalValues[0].(map[string]any)
			if firstSignal["id"] != "rsi_overbought" || firstSignal["status"] != "triggered" || firstSignal["severity"] != "warning" {
				t.Fatalf("unexpected first signal: %+v", firstSignal)
			}
			evidence := firstSignal["evidence"].(map[string]any)
			if evidence["rsi14"] != float64(70) || evidence["threshold"] != float64(70) {
				t.Fatalf("unexpected signal evidence: %+v", evidence)
			}
			momentum := data["momentum"].(map[string]any)
			if data["trend"].(map[string]any)["sma50"] != "95.00" {
				t.Fatalf("SMA50 missing from trend: %+v", data["trend"])
			}
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
			_, hasRulesetVersion := meta["ruleset_version"]
			if hasRulesetVersion != tc.details {
				t.Fatal("ruleset_version presence does not match includeDetails")
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
