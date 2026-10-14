package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jamersom/market-data-api/internal/observability"
)

type pingerStub struct {
	err    error
	calls  int
	verify func(context.Context)
}

func (p *pingerStub) Ping(ctx context.Context) error {
	p.calls++
	if p.verify != nil {
		p.verify(ctx)
	}
	return p.err
}

func TestOperationsLivenessDoesNotPingDatabase(t *testing.T) {
	pinger := &pingerStub{verify: func(context.Context) { t.Fatal("liveness pinged database") }}
	handler := NewOperationsHandler(pinger, observability.NewRegistry(), time.Second)
	recorder := httptest.NewRecorder()
	handler.Live(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "{\"status\":\"ok\"}\n" || pinger.calls != 0 {
		t.Fatalf("unexpected liveness response: status=%d body=%q calls=%d", recorder.Code, recorder.Body.String(), pinger.calls)
	}
}

func TestOperationsReadiness(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "ready", status: http.StatusOK, body: "{\"status\":\"ok\"}\n"},
		{name: "database unavailable", err: errors.New("database unavailable"), status: http.StatusServiceUnavailable, body: "{\"status\":\"unavailable\"}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pinger := &pingerStub{err: tc.err, verify: func(ctx context.Context) {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("readiness context has no timeout")
				}
			}}
			registry := observability.NewRegistry()
			handler := NewOperationsHandler(pinger, registry, 50*time.Millisecond)
			recorder := httptest.NewRecorder()
			handler.Ready(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
			if recorder.Code != tc.status || recorder.Body.String() != tc.body || pinger.calls != 1 {
				t.Fatalf("unexpected readiness response: status=%d body=%q calls=%d", recorder.Code, recorder.Body.String(), pinger.calls)
			}
			metrics := registry.Snapshot().Database
			if len(metrics) != 1 || metrics[0].Operation != "readiness_ping" || metrics[0].Calls != 1 || metrics[0].Errors != boolUint64(tc.err != nil) {
				t.Fatalf("unexpected readiness metrics: %+v", metrics)
			}
		})
	}
}

func TestOperationsReadinessTimeout(t *testing.T) {
	pinger := &pingerStub{}
	pinger.verify = func(ctx context.Context) {
		<-ctx.Done()
		pinger.err = ctx.Err()
	}
	handler := NewOperationsHandler(pinger, observability.NewRegistry(), time.Millisecond)
	recorder := httptest.NewRecorder()
	handler.Ready(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable || !errors.Is(pinger.err, context.DeadlineExceeded) {
		t.Fatalf("status=%d ping error=%v", recorder.Code, pinger.err)
	}
}

func TestOperationsMetrics(t *testing.T) {
	registry := observability.NewRegistry()
	registry.RecordHTTP(http.MethodGet, "/health/live", http.StatusOK, time.Millisecond)
	handler := NewOperationsHandler(&pingerStub{}, registry, time.Second)
	recorder := httptest.NewRecorder()
	handler.Metrics(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	var body observability.Snapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || len(body.HTTP) != 1 || body.HTTP[0].Route != "/health/live" {
		t.Fatalf("unexpected metrics response: status=%d body=%+v", recorder.Code, body)
	}
}

func boolUint64(value bool) uint64 {
	if value {
		return 1
	}
	return 0
}
