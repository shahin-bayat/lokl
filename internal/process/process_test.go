package process

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shahin-bayat/lokl/internal/config"
)

func TestLineBuffer(t *testing.T) {
	t.Run("basic write and read", func(t *testing.T) {
		buf := newLogs(10)
		_, _ = buf.Write([]byte("line1\nline2\nline3\n"))

		lines := buf.Lines()
		if len(lines) != 3 {
			t.Errorf("got %d lines, want 3", len(lines))
		}
		if lines[0] != "line1" {
			t.Errorf("lines[0] = %q, want %q", lines[0], "line1")
		}
	})

	t.Run("exceeds max lines", func(t *testing.T) {
		buf := newLogs(3)
		_, _ = buf.Write([]byte("a\nb\nc\nd\ne\n"))

		lines := buf.Lines()
		if len(lines) != 3 {
			t.Errorf("got %d lines, want 3", len(lines))
		}
		if lines[0] != "c" {
			t.Errorf("oldest line should be 'c', got %q", lines[0])
		}
	})

	t.Run("partial line", func(t *testing.T) {
		buf := newLogs(10)
		_, _ = buf.Write([]byte("complete\npartial"))
		_, _ = buf.Write([]byte(" continued\n"))

		lines := buf.Lines()
		if len(lines) != 2 {
			t.Errorf("got %d lines, want 2", len(lines))
		}
		if lines[1] != "partial continued" {
			t.Errorf("lines[1] = %q, want %q", lines[1], "partial continued")
		}
	})
}

func TestNewProcess(t *testing.T) {
	p := New("web", config.Service{Command: "npm start"}, func() {})
	if p.IsRunning() {
		t.Error("new process should not be running")
	}
	if p.IsHealthy() {
		t.Error("new process should not be healthy")
	}
	if p.Logs() != nil {
		t.Error("new process should have nil logs")
	}
}

func TestBuildEnv(t *testing.T) {
	p := New("web", config.Service{
		Command: "npm start",
		Env:     map[string]string{"NODE_ENV": "production", "PORT": "3000"},
	}, func() {})

	env := p.buildEnv()

	// Should contain os.Environ() entries plus our custom ones
	osEnvLen := len(os.Environ())
	if len(env) != osEnvLen+2 {
		t.Errorf("env len = %d, want %d (os=%d + 2)", len(env), osEnvLen+2, osEnvLen)
	}

	found := map[string]bool{}
	for _, e := range env {
		if strings.HasPrefix(e, "NODE_ENV=") {
			found["NODE_ENV"] = true
		}
		if strings.HasPrefix(e, "PORT=") {
			found["PORT"] = true
		}
	}
	if !found["NODE_ENV"] || !found["PORT"] {
		t.Errorf("custom env vars not found: %v", found)
	}
}

func TestCheckPortFree(t *testing.T) {
	if err := checkPortFree(0); err != nil {
		t.Errorf("port 0 should be free: %v", err)
	}

	// Occupy a port, then check
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	port := ln.Addr().(*net.TCPAddr).Port
	if err := checkPortFree(port); err == nil {
		t.Errorf("port %d should be occupied", port)
	}
}

func TestCheckHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Extract port from test server URL
	parts := strings.Split(ts.URL, ":")
	port := parts[len(parts)-1]

	p := New("web", config.Service{
		Command: "npm start",
		Port:    mustAtoi(port),
		Health:  &config.HealthConfig{Path: "/"},
	}, func() {})

	got := p.checkHealth(2 * time.Second)
	if !got {
		t.Error("expected healthy for 200 OK server")
	}
}

func TestCheckHealthUnhealthy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	parts := strings.Split(ts.URL, ":")
	port := parts[len(parts)-1]

	p := New("web", config.Service{
		Command: "npm start",
		Port:    mustAtoi(port),
		Health:  &config.HealthConfig{Path: "/"},
	}, func() {})

	got := p.checkHealth(2 * time.Second)
	if got {
		t.Error("expected unhealthy for 500 server")
	}
}

func TestCheckHealthTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	parts := strings.Split(ts.URL, ":")
	port := parts[len(parts)-1]

	p := New("web", config.Service{
		Command: "npm start",
		Port:    mustAtoi(port),
		Health:  &config.HealthConfig{Path: "/"},
	}, func() {})

	got := p.checkHealth(50 * time.Millisecond)
	if got {
		t.Error("expected unhealthy for slow server with short timeout")
	}
}

func mustAtoi(s string) int {
	var n int
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state state
		want  string
	}{
		{stateStopped, "stopped"},
		{stateStarting, "starting"},
		{stateRunning, "running"},
		{stateStopping, "stopping"},
		{stateFailed, "failed"},
		{state(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("state(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
