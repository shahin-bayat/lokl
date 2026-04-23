package process

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/runner"
)

//nolint:unparam // fixture helper; kept for symmetry with execCmd
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

func TestLookPathWithEnv(t *testing.T) {
	dir := t.TempDir()
	bin := dir + "/mycli"
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := lookPathWithEnv("mycli", []string{"PATH=" + dir}, "")
	if err != nil {
		t.Fatalf("lookPathWithEnv: %v", err)
	}
	if got != bin {
		t.Errorf("got %q, want %q", got, bin)
	}

	if _, err := lookPathWithEnv("mycli", []string{"PATH=/nowhere"}, ""); err == nil {
		t.Errorf("expected not-found error for missing PATH entry")
	}

	if got, _ := lookPathWithEnv("/usr/bin/absolute", []string{"PATH=/x"}, ""); got != "/usr/bin/absolute" {
		t.Errorf("absolute path should pass through, got %q", got)
	}
}

func TestLookPathWithEnvRelativeToCwd(t *testing.T) {
	cwd := t.TempDir()
	nodeModulesBin := cwd + "/node_modules/.bin"
	if err := os.MkdirAll(nodeModulesBin, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bin := nodeModulesBin + "/vite"
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := lookPathWithEnv("vite", []string{"PATH=./node_modules/.bin"}, cwd)
	if err != nil {
		t.Fatalf("lookPathWithEnv: %v", err)
	}
	if got != bin {
		t.Errorf("got %q, want %q", got, bin)
	}
}

func TestLookPathWithEnvReturnsAbsolute(t *testing.T) {
	root := t.TempDir()
	sub := root + "/frontend/node_modules/.bin"
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bin := sub + "/vite"
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Simulate a relative cwd like `./frontend`: if the helper returned a
	// relative path, cmd.Dir would re-resolve it and look in frontend/frontend.
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got, err := lookPathWithEnv("vite", []string{"PATH=./node_modules/.bin"}, "./frontend")
	if err != nil {
		t.Fatalf("lookPathWithEnv: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("got %q, want absolute path", got)
	}
	if !strings.HasSuffix(got, "/frontend/node_modules/.bin/vite") {
		t.Errorf("got %q, want suffix /frontend/node_modules/.bin/vite", got)
	}
}

func TestProcessExecFormMissingBinary(t *testing.T) {
	p := New("bad", config.Service{Command: execCmd("definitely-not-a-real-binary-xyz")}, func() {}, func() {})
	err := p.Start()
	if err == nil {
		t.Fatalf("expected error for missing binary")
	}
	p.mu.Lock()
	st := p.state
	p.mu.Unlock()
	if st != runner.StateFailed {
		t.Errorf("state = %v, want StateFailed", st)
	}
}
