package process

import (
	"net"
	"os"
	"strings"
	"testing"

	"github.com/shahin-bayat/lokl/internal/config"
)

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
