package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jamersom/market-data-api/internal/adapters/inbound/http/handlers"
	"github.com/jamersom/market-data-api/internal/application/ports/inbound"
)

type routeComparisonStub struct{ calls int }

func (s *routeComparisonStub) Execute(context.Context, inbound.CompareQuotesInput) (inbound.CompareQuotesOutput, error) {
	s.calls++
	return inbound.CompareQuotesOutput{}, nil
}

func TestComparisonRoutes(t *testing.T) {
	stub := &routeComparisonStub{}
	mux := http.NewServeMux()
	RegisterRoutes(mux, handlers.NewQuoteHandler(nil), handlers.NewQuoteHistoryHandler(nil), handlers.NewComparisonsHandler(stub))
	for _, method := range []string{"GET", "POST", "DELETE"} {
		request := httptest.NewRequest(method, "/comparisons?tickers=PETR4,VALE3&from=2025-01-01&to=2025-01-31", strings.NewReader(`{"tickers":["PETR4","VALE3"],"from":"2025-01-01","to":"2025-01-31"}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		want := 200
		if method == "DELETE" {
			want = 405
		}
		if recorder.Code != want {
			t.Fatalf("%s: %d %s", method, recorder.Code, recorder.Body)
		}
	}
	if stub.calls != 2 {
		t.Fatalf("chamadas: %d", stub.calls)
	}
}

func TestServeOpenAPI(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	response := httptest.NewRecorder()

	serveOpenAPI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/yaml; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", contentType)
	}
	if !strings.HasPrefix(response.Body.String(), "openapi: 3.1.0") {
		t.Fatal("response does not contain the embedded OpenAPI document")
	}
}

func TestServeSwaggerUI(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/docs/", nil)
	response := httptest.NewRecorder()

	serveSwaggerUI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if !strings.Contains(response.Body.String(), `url: "/openapi.yaml"`) {
		t.Fatal("Swagger UI does not reference the embedded OpenAPI document")
	}
}

func TestRedirectToDocs(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/docs", nil)
	response := httptest.NewRecorder()

	redirectToDocs(response, request)

	if response.Code != http.StatusPermanentRedirect {
		t.Fatalf("expected status %d, got %d", http.StatusPermanentRedirect, response.Code)
	}
	if location := response.Header().Get("Location"); location != "/docs/" {
		t.Fatalf("unexpected redirect location: %s", location)
	}
}
