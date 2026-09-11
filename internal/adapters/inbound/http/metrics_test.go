package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jamersom/market-data-api/internal/observability"
)

func TestMetricsMiddlewareUsesNormalizedRouteAndStatus(t *testing.T) {
	registry := observability.NewRegistry()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /assets/{ticker}/intelligence", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusPartialContent)
	})
	recorder := httptest.NewRecorder()
	MetricsMiddleware(mux, registry).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/assets/PETR4/intelligence?asOf=2024-01-01", nil))
	metrics := registry.Snapshot().HTTP
	if recorder.Code != http.StatusPartialContent || len(metrics) != 1 {
		t.Fatalf("unexpected response or metrics: status=%d metrics=%+v", recorder.Code, metrics)
	}
	metric := metrics[0]
	if metric.Method != http.MethodGet || metric.Route != "/assets/{ticker}/intelligence" || metric.Status != http.StatusPartialContent || metric.Requests != 1 {
		t.Fatalf("unexpected metric: %+v", metric)
	}
}
