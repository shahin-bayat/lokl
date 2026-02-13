package runner

import (
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
