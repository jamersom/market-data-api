package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/jamersom/market-data-api/internal/observability"
)

type databasePinger interface {
	Ping(context.Context) error
}

type OperationsHandler struct {
	database databasePinger
	metrics  *observability.Registry
	timeout  time.Duration
}

func NewOperationsHandler(database databasePinger, metrics *observability.Registry, timeout time.Duration) *OperationsHandler {
	return &OperationsHandler{database: database, metrics: metrics, timeout: timeout}
}

func (h *OperationsHandler) Live(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *OperationsHandler) Ready(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), h.timeout)
	defer cancel()
	started := time.Now()
	err := h.database.Ping(ctx)
	h.metrics.RecordDatabase("readiness_ping", time.Since(started), 0, err)
	if err != nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *OperationsHandler) Metrics(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, h.metrics.Snapshot())
}
