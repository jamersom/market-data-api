package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/response"
	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
	"github.com/jamersom/market-data-api/internal/application/services"
	"github.com/jamersom/market-data-api/internal/domain"
)

type compareQuotesStub struct {
	input    inbound.CompareQuotesInput
	output   inbound.CompareQuotesOutput
	err      error
	calls    int
	deadline bool
}

func (s *compareQuotesStub) Execute(ctx context.Context, input inbound.CompareQuotesInput) (inbound.CompareQuotesOutput, error) {
	s.calls++
	s.input = input
	_, s.deadline = ctx.Deadline()
	return s.output, s.err
}

func TestComparisonsEquivalentRequests(t *testing.T) {
	stub := &compareQuotesStub{}
	handler := NewComparisonsHandler(stub)
	get := httptest.NewRequest("GET", "/comparisons?tickers=PETR4,VALE3&from=2025-01-01&to=2025-01-31&marketType=10&metrics=return,correlation&benchmark=IBOV&includeSeries=true", nil)
	getResponse := httptest.NewRecorder()
	handler.Get(getResponse, get)
	getInput := stub.input
	post := httptest.NewRequest("POST", "/comparisons", strings.NewReader(`{"tickers":["PETR4","VALE3"],"from":"2025-01-01","to":"2025-01-31","marketType":10,"metrics":["return","correlation"],"benchmark":"IBOV","includeSeries":true}`))
	post.Header.Set("Content-Type", "application/json; charset=utf-8")
	postResponse := httptest.NewRecorder()
	handler.Post(postResponse, post)
	if getResponse.Code != 200 || postResponse.Code != 200 || !reflect.DeepEqual(getInput, stub.input) || getResponse.Body.String() != postResponse.Body.String() || !stub.deadline {
		t.Fatalf("GET/POST divergentes: %s / %s", getResponse.Body, postResponse.Body)
	}
}

func TestComparisonsInvalidTransportInput(t *testing.T) {
	valid := `{"tickers":["PETR4","VALE3"],"from":"2025-01-01","to":"2025-01-31"}`
	tests := []struct {
		name, method, url, body, content string
		status                           int
	}{
		{"data ausente", "GET", "/comparisons?tickers=PETR4,VALE3", "", "", 400},
		{"data inválida", "GET", "/comparisons?from=2025-02-30&to=2025-03-01", "", "", 400},
		{"mercado texto", "GET", "/comparisons?marketType=abc", "", "", 400},
		{"mercado zero", "GET", "/comparisons?from=2025-01-01&to=2025-01-31&marketType=0", "", "", 400},
		{"booleano", "GET", "/comparisons?includeSeries=1", "", "", 400},
		{"repetido", "GET", "/comparisons?tickers=PETR4&tickers=VALE3", "", "", 400},
		{"desconhecido", "GET", "/comparisons?other=1", "", "", 400},
		{"mídia", "POST", "/comparisons", valid, "text/plain", 415},
		{"vazio", "POST", "/comparisons", "", "application/json", 400},
		{"sintaxe", "POST", "/comparisons", "{", "application/json", 400},
		{"tipo", "POST", "/comparisons", `{"tickers":"PETR4,VALE3"}`, "application/json", 400},
		{"campo extra", "POST", "/comparisons", `{"other":1}`, "application/json", 400},
		{"objetos múltiplos", "POST", "/comparisons", valid + valid, "application/json", 400},
		{"lixo após JSON", "POST", "/comparisons", valid + "!", "application/json", 400},
		{"nulo", "POST", "/comparisons", "null", "application/json", 400},
		{"grande", "POST", "/comparisons", `{"benchmark":"` + strings.Repeat("A", comparisonBodyLimit) + `"}`, "application/json", 413},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &compareQuotesStub{}
			handler := NewComparisonsHandler(stub)
			req := httptest.NewRequest(tt.method, tt.url, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.content)
			rec := httptest.NewRecorder()
			if tt.method == "GET" {
				handler.Get(rec, req)
			} else {
				handler.Post(rec, req)
			}
			if rec.Code != tt.status || stub.calls != 0 {
				t.Fatalf("status %d, chamadas %d: %s", rec.Code, stub.calls, rec.Body)
			}
		})
	}
}

func TestComparisonsApplicationErrors(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ValidationError{Err: domain.ErrInvalidTickers}, 400, "invalid_tickers"},
		{domain.ValidationError{Err: domain.ErrInvalidMetric}, 400, "invalid_metric"},
		{domain.ErrInsufficientComparisonData, 422, "insufficient_comparison_data"},
		{context.DeadlineExceeded, 504, "request_timeout"},
		{errors.New("database secret"), 500, "internal_error"},
	}
	for _, tt := range tests {
		handler := NewComparisonsHandler(&compareQuotesStub{err: tt.err})
		rec := httptest.NewRecorder()
		handler.Get(rec, httptest.NewRequest("GET", "/comparisons?tickers=PETR4,VALE3&from=2025-01-01&to=2025-01-31", nil))
		var body response.Error
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if rec.Code != tt.status || body.Error.Code != tt.code || strings.Contains(rec.Body.String(), "secret") {
			t.Fatalf("resposta inesperada: %d %s", rec.Code, rec.Body)
		}
	}
}

type comparisonHTTPRepository struct{}

func (comparisonHTTPRepository) FindByTickersAndPeriod(_ context.Context, tickers []string, from, to time.Time, market int) (map[string][]domain.QuoteRecord, error) {
	result := make(map[string][]domain.QuoteRecord)
	for _, ticker := range tickers {
		if ticker != "PETR4" && ticker != "VALE3" {
			continue
		}
		for i, price := range []int64{100, 120, 90} {
			result[ticker] = append(result[ticker], domain.QuoteRecord{Quote: domain.Quote{Ticker: ticker, TradingDate: from.AddDate(0, 0, i), ClosePriceCents: price, HighPriceCents: price, LowPriceCents: price, Currency: "BRL"}})
		}
	}
	return result, nil
}

func TestComparisonsHTTPWithApplication(t *testing.T) {
	handler := NewComparisonsHandler(services.NewCompareQuotesService(comparisonHTTPRepository{}, nil))
	rec := httptest.NewRecorder()
	handler.Get(rec, httptest.NewRequest("GET", "/comparisons?tickers=petr4,vale3,ABCD3&from=2025-01-01&to=2025-01-31&includeSeries=true&benchmark=IBOV&metrics=return,drawdown,correlation", nil))
	var body response.ComparisonEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(body.Data.Assets) != 3 {
		t.Fatalf("resposta: %d %s", rec.Code, rec.Body)
	}
	asset := body.Data.Assets[0]
	if *asset.AbsoluteReturn != "-0.10" || *asset.InitialPrice != "1.00" || *asset.MaximumDrawdown != -25 || asset.Series[2].ClosePrice != "0.90" || body.Meta.MarketType != 10 || body.Meta.PriceAdjustment != "unadjusted" || body.Data.Benchmark.Status != "insufficient_data" || body.Data.Assets[2].AvailableFrom != nil || len(body.Data.Correlations) != 1 {
		t.Fatalf("mapeamento inesperado: %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), "annualizedVolatility") || strings.Contains(rec.Body.String(), "PriceCents") {
		t.Fatalf("campos indevidos: %s", rec.Body)
	}
}

func TestComparisonMapperPreservesZero(t *testing.T) {
	zero := 0.0
	cents := int64(0)
	asset := assetComparisonResponse(domain.AssetComparison{PercentageReturn: &zero, AbsoluteReturnCents: &cents})
	data, err := json.Marshal(asset)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"percentageReturn":0`) || !strings.Contains(string(data), `"absoluteReturn":"0.00"`) || strings.Contains(string(data), `"series"`) {
		t.Fatal(string(data))
	}
}
