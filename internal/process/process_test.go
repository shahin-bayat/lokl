package process

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/runner"
)

func shCmd(s string) config.StringOrSlice {
	return config.StringOrSlice{Args: []string{s}, Shell: true}
}

func execCmd(args ...string) config.StringOrSlice {
	return config.StringOrSlice{Args: args, Shell: false}
}

func TestNewProcess(t *testing.T) {
	p := New("web", config.Service{Command: shCmd("npm start")}, func() {}, func() {})
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
		Command: shCmd("npm start"),
		Env:     map[string]string{"NODE_ENV": "production", "PORT": "3000"},
	}, func() {}, func() {})

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

func TestOnCrashCalledOnHealthyToCrash(t *testing.T) {
	var crashCount int
	onCrash := func() { crashCount++ }
	p := New("web", config.Service{Command: shCmd("npm start")}, func() {}, onCrash)

	p.mu.Lock()
	p.state = runner.StateRunning
	p.healthy = true
	p.mu.Unlock()

	// Simulate the wasHealthy check in the health callback.
	p.mu.Lock()
	wasHealthy := p.healthy
	p.healthy = false
	p.mu.Unlock()
	if wasHealthy {
		p.onCrash()
	}

	if crashCount != 1 {
		t.Errorf("onCrash should fire once on healthy→crash; got %d", crashCount)
	}
}

func TestOnCrashCalledOnNeverHealthyExit(t *testing.T) {
	var crashCount int
	onCrash := func() { crashCount++ }
	p := New("web", config.Service{Command: shCmd("npm start")}, func() {}, onCrash)

	p.mu.Lock()
	p.state = runner.StateRunning
	p.healthy = false
	p.manuallyStopped = false
	p.mu.Unlock()

	// Simulate reaper logic: fires regardless of healthy state.
	p.mu.Lock()
	shouldNotify := !p.manuallyStopped && p.state == runner.StateRunning
	p.state = runner.StateFailed
	p.healthy = false
	p.mu.Unlock()
	if shouldNotify {
		p.onCrash()
	}

	if crashCount != 1 {
		t.Errorf("onCrash should fire on never-healthy exit; got %d", crashCount)
	}
}

func TestOnCrashCalledOnHealthyProcessExit(t *testing.T) {
	var crashCount int
	onCrash := func() { crashCount++ }
	p := New("web", config.Service{Command: shCmd("npm start")}, func() {}, onCrash)

	// Simulate: process was healthy (e.g. no health check) and crashes.
	p.mu.Lock()
	p.state = runner.StateRunning
	p.healthy = true
	p.manuallyStopped = false
	p.mu.Unlock()

	// Simulate reaper logic: !manuallyStopped && Running → fires.
	p.mu.Lock()
	shouldNotify := !p.manuallyStopped && p.state == runner.StateRunning
	p.state = runner.StateFailed
	p.healthy = false
	p.mu.Unlock()
	if shouldNotify {
		p.onCrash()
	}

	if crashCount != 1 {
		t.Errorf("onCrash should fire on healthy process exit; got %d", crashCount)
	}
}

func TestOnCrashNotCalledOnManualStop(t *testing.T) {
	var crashCount int
	onCrash := func() { crashCount++ }
	p := New("web", config.Service{Command: shCmd("npm start")}, func() {}, onCrash)

	p.mu.Lock()
	p.state = runner.StateRunning
	p.healthy = false
	p.manuallyStopped = true
	p.mu.Unlock()

	// Simulate reaper logic: manuallyStopped=true suppresses notification.
	p.mu.Lock()
	shouldNotify := !p.manuallyStopped && p.state == runner.StateRunning
	p.state = runner.StateFailed
	p.mu.Unlock()
	if shouldNotify {
		p.onCrash()
	}

	if crashCount != 0 {
		t.Errorf("onCrash should NOT fire on manual stop; got %d", crashCount)
	}
}

func TestOnCrashNotCalledOnHealthyManualStop(t *testing.T) {
	var crashCount int
	onCrash := func() { crashCount++ }
	p := New("web", config.Service{Command: shCmd("npm start")}, func() {}, onCrash)

	p.mu.Lock()
	p.state = runner.StateRunning
	p.healthy = true
	p.manuallyStopped = true
	p.mu.Unlock()

	// manuallyStopped=true suppresses even when healthy.
	p.mu.Lock()
	shouldNotify := !p.manuallyStopped && p.state == runner.StateRunning
	p.mu.Unlock()
	if shouldNotify {
		p.onCrash()
	}

	if crashCount != 0 {
		t.Errorf("onCrash should NOT fire on healthy manual stop; got %d", crashCount)
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

func TestProcessExecForm(t *testing.T) {
	p := New("echo", config.Service{Command: execCmd("/bin/echo", "hello-exec")}, func() {}, func() {})

	if err := p.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		st := p.state
		p.mu.Unlock()
		if st != runner.StateRunning && st != runner.StateStarting {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	logs := p.Logs()
	for _, line := range logs {
		if strings.Contains(line, "hello-exec") {
			return
		}
	}
	t.Errorf("expected 'hello-exec' in logs, got: %v", logs)
}
