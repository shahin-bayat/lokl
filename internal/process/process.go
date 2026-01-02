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

const stopTimeout = 10 * time.Second

const maxLogLines = 1000

type Process struct {
	Name    string
	Config  config.Service
	State   State
	Healthy bool

	cmd          *exec.Cmd
	logs         *lineBuffer
	healthCancel context.CancelFunc
	mu           sync.Mutex
}

func New(name string, cfg config.Service) *Process {
	return &Process{
		Name:   name,
		Config: cfg,
		State:  StateStopped,
	}
}

func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.State != StateStopped && p.State != StateFailed {
		return fmt.Errorf("process %s: cannot start from state %s", p.Name, p.State)
	}

	p.State = StateStarting

	p.cmd = exec.Command("sh", "-c", p.Config.Command)

	if p.Config.Path != "" {
		p.cmd.Dir = p.Config.Path
	}

	p.cmd.Env = p.buildEnv()

	p.logs = newLineBuffer(maxLogLines)
	p.cmd.Stdout = p.logs
	p.cmd.Stderr = p.logs

	if err := p.cmd.Start(); err != nil {
		p.State = StateFailed
		return fmt.Errorf("process %s: failed to start: %w", p.Name, err)
	}

	p.State = StateRunning

	// Watch for unexpected exit - if process dies while still "running", mark as failed
	go func() {
		_ = p.cmd.Wait()
		p.mu.Lock()
		if p.State == StateRunning {
			p.State = StateFailed
		}
		p.mu.Unlock()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	p.healthCancel = cancel
	go p.startHealthCheck(ctx)

	return nil
}

func (p *Process) Stop() error {
	p.mu.Lock()
	if p.State != StateRunning && p.State != StateStarting {
		p.mu.Unlock()
		return nil
	}
	p.State = StateStopping
	cmd := p.cmd
	if p.healthCancel != nil {
		p.healthCancel()
	}
	p.mu.Unlock()

	_ = cmd.Process.Signal(syscall.SIGTERM)

	time.AfterFunc(stopTimeout, func() {
		_ = cmd.Process.Kill()
	})
	_ = cmd.Wait()

	p.mu.Lock()
	p.State = StateStopped
	p.mu.Unlock()

	return nil
}

func (p *Process) buildEnv() []string {
	env := os.Environ()
	for k, v := range p.Config.Env {
		env = append(env, k+"="+v)
	}
	return env
}

func (p *Process) Logs() []string {
	if p.logs == nil {
		return nil
	}
	return p.logs.Lines()
}
