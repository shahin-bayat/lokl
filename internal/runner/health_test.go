package runner

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestCheckHealth(t *testing.T) {
	t.Run("healthy server", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		client := &http.Client{Timeout: 2 * time.Second}
		port := testServerPort(t, ts)
		if !checkHealth(client, port, "/") {
			t.Error("expected healthy for 200 OK server")
		}
	})

	t.Run("unhealthy server", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		client := &http.Client{Timeout: 2 * time.Second}
		port := testServerPort(t, ts)
		if checkHealth(client, port, "/") {
			t.Error("expected unhealthy for 500 server")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(500 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		client := &http.Client{Timeout: 50 * time.Millisecond}
		port := testServerPort(t, ts)
		if checkHealth(client, port, "/") {
			t.Error("expected unhealthy for slow server with short timeout")
		}
	})
}

func testServerPort(t *testing.T, ts *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	_, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return port
}

func TestRunProbeSucceeds(t *testing.T) {
	calls := 0
	probe := func() bool {
		calls++
		return calls >= 2 // healthy on 2nd call
	}

	var gotHealthy bool
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	RunProbe(ctx, probe, 10*time.Millisecond, 100*time.Millisecond, 5, func(healthy bool) {
		gotHealthy = healthy
		cancel()
	})

	if !gotHealthy {
		t.Error("expected healthy=true callback")
	}
}

func TestRunProbeNeverHealthyOnAlwaysFailing(t *testing.T) {
	// A probe that always fails should never trigger onChange(true).
	// onChange(false) is only called when transitioning healthy→unhealthy,
	// which can't happen if the service never became healthy.
	probe := func() bool { return false }

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	called := false
	RunProbe(ctx, probe, 10*time.Millisecond, 100*time.Millisecond, 3, func(healthy bool) {
		called = true
	})

	if called {
		t.Error("expected no onChange callback for always-failing probe (never was healthy)")
	}
}

func TestRunProbeCancellation(t *testing.T) {
	// Verify RunProbe stops polling after context is cancelled.
	// Use a counter to track probe invocations.
	probeCount := 0
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	RunProbe(ctx, func() bool {
		probeCount++
		return false // never healthy
	}, 20*time.Millisecond, 100*time.Millisecond, 3, func(bool) {
		// no callbacks expected (never transitions if always unhealthy)
	})

	// After cancellation, probeCount should be small (not many iterations).
	// With 100ms timeout and 20ms interval, expect ~5 calls max.
	// This verifies the context cancellation stopped the polling loop.
	if probeCount > 10 {
		t.Errorf("expected RunProbe to stop after context cancelled, got %d probe calls", probeCount)
	}
}
