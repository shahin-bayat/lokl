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
	name     string
	config   config.Service
	state    state
	healthy  bool
	onChange func()

	cmd    *exec.Cmd
	logs   *logs
	cancel context.CancelFunc
	exitCh chan struct{}
	mu     sync.Mutex
}

func New(name string, cfg config.Service, onChange func()) *Process {
	return &Process{
		name:     name,
		config:   cfg,
		state:    stateStopped,
		onChange: onChange,
	}
}

func (p *Process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state == stateRunning
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

	if p.state != stateStopped && p.state != stateFailed {
		return fmt.Errorf("process %s: cannot start from state %s", p.name, p.state)
	}

	p.state = stateStarting

	p.cmd = exec.Command("sh", "-c", "exec "+p.config.Command)
	p.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if p.config.Path != "" {
		p.cmd.Dir = p.config.Path
	}

	p.cmd.Env = p.buildEnv()

	p.logs = newLogs(maxLogLines)
	p.cmd.Stdout = p.logs
	p.cmd.Stderr = p.logs

	if err := p.cmd.Start(); err != nil {
		p.state = stateFailed
		return fmt.Errorf("process %s: failed to start: %w", p.name, err)
	}

	p.state = stateRunning
	p.exitCh = make(chan struct{})
	p.onChange()

	// Single goroutine that waits for process exit and signals via channel
	go func() {
		_ = p.cmd.Wait()
		p.mu.Lock()
		if p.state == stateRunning {
			p.state = stateFailed
		}
		p.mu.Unlock()
		p.onChange()
		close(p.exitCh)
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
	exitCh := p.exitCh
	pgid := p.cmd.Process.Pid
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Unlock()

	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	// Schedule SIGKILL but cancel if process exits cleanly
	killTimer := time.AfterFunc(stopTimeout, func() {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	})

	// Wait for the exit signal from the goroutine that called Wait()
	<-exitCh
	killTimer.Stop()

	p.mu.Lock()
	p.state = stateStopped
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
