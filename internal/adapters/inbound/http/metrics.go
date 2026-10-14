package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/jamersom/market-data-api/internal/observability"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func MetricsMiddleware(next http.Handler, registry *observability.Registry) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: writer}
		next.ServeHTTP(wrapped, request)
		status := wrapped.status
		if status == 0 {
			status = http.StatusOK
		}
		route := request.Pattern
		if route == "" {
			route = "unmatched"
		} else {
			route = strings.TrimPrefix(route, request.Method+" ")
		}
		registry.RecordHTTP(request.Method, route, status, time.Since(started))
	})
}
