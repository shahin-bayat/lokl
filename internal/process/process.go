// Package process manages service process lifecycle.
package process

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/runner"
)

const (
	stopTimeout = 10 * time.Second
	maxLogLines = 1000
)

type Process struct {
	name     string
	config   config.Service
	state    runner.State
	healthy  bool
	onChange func()
	onCrash  func()

	manuallyStopped bool
	cmd             *exec.Cmd
	logs            *runner.Logs
	cancel          context.CancelFunc
	exitCh          chan struct{}
	mu              sync.Mutex
}

func New(name string, cfg config.Service, onChange func(), onCrash func()) *Process {
	return &Process{
		name:     name,
		config:   cfg,
		state:    runner.StateStopped,
		onChange: onChange,
		onCrash:  onCrash,
	}
}

func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state == runner.StateRunning
}

func (p *Process) IsHealthy() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.healthy
}

func (p *Process) Logs() []string {
	if p.logs == nil {
		return nil
	}
	return p.logs.Lines()
}

func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != runner.StateStopped && p.state != runner.StateFailed {
		return fmt.Errorf("process %s: cannot start from state %s", p.name, p.state)
	}

	if p.config.Port > 0 {
		if err := checkPortFree(p.config.Port); err != nil {
			return fmt.Errorf("process %s: %w", p.name, err)
		}
	}

	p.state = runner.StateStarting

	// exec replaces the shell so there's one process to manage, not sh + child.
	// Setpgid isolates the process tree for clean group-kill on shutdown.
	p.cmd = exec.Command("sh", "-c", "exec "+p.config.Command)
	p.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if p.config.Path != "" {
		p.cmd.Dir = p.config.Path
	}

	p.cmd.Env = p.buildEnv()

	p.logs = runner.NewLogs(maxLogLines)

	// Use os.Pipe so cmd.Wait() only waits for process exit, not I/O.
	// When Stdout is an *os.File, Go passes the fd directly — no internal
	// copy goroutine. This prevents Wait() from blocking when orphaned
	// child processes (e.g. esbuild watchers) keep the pipe open.
	pr, pw, err := os.Pipe()
	if err != nil {
		p.state = runner.StateFailed
		return fmt.Errorf("process %s: creating pipe: %w", p.name, err)
	}
	p.cmd.Stdout = pw
	p.cmd.Stderr = pw

	if err := p.cmd.Start(); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		p.state = runner.StateFailed
		return fmt.Errorf("process %s: failed to start: %w", p.name, err)
	}

	_ = pw.Close()
	go func() { _, _ = io.Copy(p.logs, pr) }()

	p.state = runner.StateRunning
	if p.config.Health == nil || p.config.Health.Path == "" {
		p.healthy = true
	}
	p.exitCh = make(chan struct{})
	p.onChange()

	// Reaper: waits for process exit (crash or signal), updates state,
	// cancels health checks, and signals exitCh so Stop() can unblock.
	go func() {
		_ = p.cmd.Wait()
		_ = pr.Close()
		p.mu.Lock()
		shouldNotify := !p.healthy && !p.manuallyStopped && p.state == runner.StateRunning
		if p.state == runner.StateRunning {
			p.state = runner.StateFailed
		}
		p.healthy = false
		if p.cancel != nil {
			p.cancel()
		}
		p.mu.Unlock()
		if shouldNotify {
			p.onCrash()
		}
		p.onChange()
		close(p.exitCh)
	}()

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	if p.config.Health != nil && p.config.Health.Path != "" {
		interval, _ := time.ParseDuration(p.config.Health.Interval)
		timeout, _ := time.ParseDuration(p.config.Health.Timeout)
		retries := *p.config.Health.Retries
		go runner.RunHealthCheck(ctx, p.config.Port, p.config.Health.Path, interval, timeout, retries, func(healthy bool) {
			if !healthy {
				p.onCrash()
			}
			p.mu.Lock()
			p.healthy = healthy
			p.mu.Unlock()
			p.onChange()
		})
	}

	return nil
}

// PGID returns the process group ID. Because Setpgid is true, PGID == PID.
// Returns 0 if the process has not started.
func (p *Process) PGID() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func (p *Process) Stop() error {
	p.mu.Lock()
	if p.state != runner.StateRunning && p.state != runner.StateStarting {
		p.mu.Unlock()
		return nil
	}
	p.manuallyStopped = true
	p.state = runner.StateStopping
	exitCh := p.exitCh
	pgid := p.cmd.Process.Pid
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Unlock()

	runner.SignalTree(pgid, syscall.SIGTERM)
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	// Schedule SIGKILL but cancel if process exits cleanly
	killTimer := time.AfterFunc(stopTimeout, func() {
		runner.SignalTree(pgid, syscall.SIGKILL)
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	})

	// Wait for the exit signal from the goroutine that called Wait()
	<-exitCh
	killTimer.Stop()

	p.mu.Lock()
	p.state = runner.StateStopped
	p.mu.Unlock()
	p.onChange()

	return nil
}

func (p *Process) buildEnv() []string {
	env := os.Environ()
	for k, v := range p.config.Env {
		env = append(env, k+"="+v)
	}
	return env
}

func checkPortFree(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("port %d already in use", port)
	}
	_ = ln.Close()
	return nil
}
