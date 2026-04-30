package proxyonly

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shahin-bayat/lokl/internal/config"
)

func TestRunnerStartStop(t *testing.T) {
	svc := config.Service{ProxyOnly: true, Port: 0, Subdomains: config.Subdomains{"x"}}
	r := New("x", svc, func() {}, func() {})

	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !r.IsRunning() {
		t.Fatal("IsRunning should be true after Start")
	}
	if err := r.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if r.IsRunning() {
		t.Fatal("IsRunning should be false after Stop")
	}
}

func TestRunnerHealthyWhenTargetListens(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	var changes atomic.Int32
	svc := config.Service{ProxyOnly: true, Port: port, Subdomains: config.Subdomains{"x"}}
	r := New("x", svc, func() { changes.Add(1) }, func() {})
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = r.Stop() })

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if r.IsHealthy() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("IsHealthy did not become true within 5s (onChange calls=%d)", changes.Load())
}

func TestRunnerUnhealthyWhenNoListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	svc := config.Service{ProxyOnly: true, Port: port, Subdomains: config.Subdomains{"x"}}
	r := New("x", svc, func() {}, func() {})
	if err := r.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = r.Stop() })

	time.Sleep(2500 * time.Millisecond)
	if r.IsHealthy() {
		t.Fatal("IsHealthy should remain false when nothing listens on target port")
	}
}

func TestRunnerLogsBanner(t *testing.T) {
	svc := config.Service{ProxyOnly: true, Port: 9999, Subdomains: config.Subdomains{"x"}}
	r := New("x", svc, func() {}, func() {})
	logs := r.Logs()
	if len(logs) == 0 {
		t.Fatal("Logs should return a banner")
	}
}

func TestRunnerHealthyViaHTTPPath(t *testing.T) {
	ready := make(chan struct{})
	var mu sync.Mutex
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		if r.URL.Path == "/healthz" {
			w.WriteHeader(200)
			select {
			case ready <- struct{}{}:
			default:
			}
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)

	port, err := strconv.Atoi(strings.TrimPrefix(srv.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	svc := config.Service{
		ProxyOnly:  true,
		Port:       port,
		Subdomains: config.Subdomains{"x"},
		Health:     &config.HealthConfig{Path: "/healthz"},
	}
	r := New("x", svc, func() {}, func() {})
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Stop() })

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if r.IsHealthy() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	mu.Lock()
	calls := callCount
	mu.Unlock()
	t.Fatalf("IsHealthy did not flip to true within 5s (probe calls=%d)", calls)
}

func TestRunnerUnhealthyViaHTTPPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	t.Cleanup(srv.Close)

	port, err := strconv.Atoi(strings.TrimPrefix(srv.URL, "http://127.0.0.1:"))
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}

	svc := config.Service{
		ProxyOnly:  true,
		Port:       port,
		Subdomains: config.Subdomains{"x"},
		Health:     &config.HealthConfig{Path: "/healthz"},
	}
	r := New("x", svc, func() {}, func() {})
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Stop() })

	time.Sleep(2500 * time.Millisecond)
	if r.IsHealthy() {
		t.Fatal("IsHealthy must stay false when HTTP probe gets 503")
	}
}

func TestRunnerHonorsHealthInterval(t *testing.T) {
	five := 5
	svc := config.Service{
		ProxyOnly:  true,
		Port:       9999,
		Subdomains: config.Subdomains{"x"},
		Health:     &config.HealthConfig{Interval: "100ms", Timeout: "50ms", Retries: &five},
	}
	r := New("x", svc, func() {}, func() {})
	interval, timeout, retries := r.probeTimings()
	if interval != 100*time.Millisecond {
		t.Errorf("interval=%v, want 100ms", interval)
	}
	if timeout != 50*time.Millisecond {
		t.Errorf("timeout=%v, want 50ms", timeout)
	}
	if retries != 5 {
		t.Errorf("retries=%d, want 5", retries)
	}
}

func TestRunnerFallsBackToDefaultsWhenHealthOmitted(t *testing.T) {
	svc := config.Service{ProxyOnly: true, Port: 9999, Subdomains: config.Subdomains{"x"}}
	r := New("x", svc, func() {}, func() {})
	interval, timeout, retries := r.probeTimings()
	if interval != defaultProbeInterval {
		t.Errorf("interval=%v, want %v", interval, defaultProbeInterval)
	}
	if timeout != defaultProbeTimeout {
		t.Errorf("timeout=%v, want %v", timeout, defaultProbeTimeout)
	}
	if retries != defaultProbeRetries {
		t.Errorf("retries=%d, want %d", retries, defaultProbeRetries)
	}
}

func TestRunnerStopIsIdempotent(t *testing.T) {
	svc := config.Service{ProxyOnly: true, Port: 9999, Subdomains: config.Subdomains{"x"}}
	r := New("x", svc, func() {}, func() {})
	if err := r.Start(); err != nil {
		t.Fatal(err)
	}
	if err := r.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := r.Stop(); err != nil {
		t.Fatalf("second Stop should be no-op: %v", err)
	}
}
