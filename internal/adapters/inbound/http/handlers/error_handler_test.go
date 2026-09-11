package handlers

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnexpectedHTTPErrorIsLoggedWithoutChangingResponse(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	request := httptest.NewRequest(http.MethodGet, "/quotes/PETR4", nil)
	request.Pattern = "GET /quotes/{ticker}"
	recorder := httptest.NewRecorder()
	writeError(recorder, request, errors.New("database diagnostic"))
	if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "database diagnostic") {
		t.Fatalf("unexpected response: status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	logLine := logs.String()
	for _, expected := range []string{"unexpected HTTP error", "GET /quotes/{ticker}", "\"status\":500", "\"category\":\"internal\"", "database diagnostic"} {
		if !strings.Contains(logLine, expected) {
			t.Fatalf("log %q does not contain %q", logLine, expected)
		}
	}
}
