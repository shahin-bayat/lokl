// Package process manages service process lifecycle.
package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/shahin-bayat/lokl/internal/config"
)

const (
	stopTimeout = 10 * time.Second
	maxLogLines = 1000
)

type Process struct {
	name    string
	config  config.Service
	state   state
	healthy bool

	cmd    *exec.Cmd
	logs   *lineBuffer
	cancel context.CancelFunc
	mu     sync.Mutex
}

func New(name string, cfg config.Service) *Process {
	return &Process{
		name:   name,
		config: cfg,
		state:  stateStopped,
	}
}

func (p *Process) IsRunning() bool {
	return p.state == stateRunning
}

func (p *Process) IsHealthy() bool {
	return p.healthy
}

func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != stateStopped && p.state != stateFailed {
		return fmt.Errorf("process %s: cannot start from state %s", p.name, p.state)
	}

	p.state = stateStarting

	// Use exec to replace shell process, making signal handling cleaner
	p.cmd = exec.Command("sh", "-c", "exec "+p.config.Command)
	p.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if p.config.Path != "" {
		p.cmd.Dir = p.config.Path
	}

	p.cmd.Env = p.buildEnv()

	p.logs = newLineBuffer(maxLogLines)
	p.cmd.Stdout = p.logs
	p.cmd.Stderr = p.logs

	if err := p.cmd.Start(); err != nil {
		p.state = stateFailed
		return fmt.Errorf("process %s: failed to start: %w", p.name, err)
	}

	p.state = stateRunning

	// Watch for unexpected exit - if process dies while still "running", mark as failed
	go func() {
		_ = p.cmd.Wait()
		p.mu.Lock()
		if p.state == stateRunning {
			p.state = stateFailed
		}
		p.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	go p.startHealthCheck(ctx)

	return nil
}

func (p *Process) Stop() error {
	p.mu.Lock()
	if p.state != stateRunning && p.state != stateStarting {
		p.mu.Unlock()
		return nil
	}
	p.state = stateStopping
	cmd := p.cmd
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Unlock()

	// Send SIGTERM to entire process group (negative PID)
	pgid := cmd.Process.Pid
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	time.AfterFunc(stopTimeout, func() {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	})
	_ = cmd.Wait()

	p.mu.Lock()
	p.state = stateStopped
	p.mu.Unlock()

	return nil
}

func (p *Process) buildEnv() []string {
	env := os.Environ()
	for k, v := range p.config.Env {
		env = append(env, k+"="+v)
	}
	return env
}
