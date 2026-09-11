package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

type managedServerStub struct {
	started       chan struct{}
	shutdown      chan struct{}
	serveErr      error
	shutdownErr   error
	shutdownCalls int
	hasDeadline   bool
}

func (s *managedServerStub) ListenAndServe() error {
	close(s.started)
	if s.serveErr != nil {
		return s.serveErr
	}
	<-s.shutdown
	return http.ErrServerClosed
}

func (s *managedServerStub) Shutdown(ctx context.Context) error {
	s.shutdownCalls++
	_, s.hasDeadline = ctx.Deadline()
	close(s.shutdown)
	return s.shutdownErr
}

func TestServeHTTPGracefulShutdown(t *testing.T) {
	server := &managedServerStub{started: make(chan struct{}), shutdown: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- serveHTTP(ctx, server, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	}()
	<-server.started
	cancel()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if server.shutdownCalls != 1 || !server.hasDeadline {
		t.Fatalf("shutdown calls=%d deadline=%t", server.shutdownCalls, server.hasDeadline)
	}
}

func TestServeHTTPReportsShutdownAndServeErrors(t *testing.T) {
	shutdownErr := errors.New("shutdown failed")
	server := &managedServerStub{started: make(chan struct{}), shutdown: make(chan struct{}), shutdownErr: shutdownErr}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- serveHTTP(ctx, server, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	}()
	<-server.started
	cancel()
	if err := <-result; !errors.Is(err, shutdownErr) {
		t.Fatalf("shutdown error = %v", err)
	}

	serveErr := errors.New("listen failed")
	server = &managedServerStub{started: make(chan struct{}), shutdown: make(chan struct{}), serveErr: serveErr}
	if err := serveHTTP(context.Background(), server, slog.Default(), time.Second); !errors.Is(err, serveErr) {
		t.Fatalf("serve error = %v", err)
	}
}

func TestDurationFromEnv(t *testing.T) {
	t.Setenv("TEST_DURATION", "250ms")
	if got := durationFromEnv("TEST_DURATION", time.Second); got != 250*time.Millisecond {
		t.Fatalf("duration = %s", got)
	}
	for _, value := range []string{"", "invalid", "0s", "-1s"} {
		t.Setenv("TEST_DURATION", value)
		if got := durationFromEnv("TEST_DURATION", time.Second); got != time.Second {
			t.Fatalf("value %q produced %s", value, got)
		}
	}
}
