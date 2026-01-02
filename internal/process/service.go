package process

import (
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

type Service struct {
	Name   string
	Config config.Service
	State  State

	cmd  *exec.Cmd
	logs *lineBuffer
	mu   sync.Mutex
}

func New(name string, cfg config.Service) *Service {
	return &Service{
		Name:   name,
		Config: cfg,
		State:  StateStopped,
	}
}

func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.State != StateStopped && s.State != StateFailed {
		return fmt.Errorf("service %s: cannot start from state %s", s.Name, s.State)
	}

	s.State = StateStarting

	s.cmd = exec.Command("sh", "-c", s.Config.Command)

	if s.Config.Path != "" {
		s.cmd.Dir = s.Config.Path
	}

	s.cmd.Env = s.buildEnv()

	s.logs = newLineBuffer(maxLogLines)
	s.cmd.Stdout = s.logs
	s.cmd.Stderr = s.logs

	if err := s.cmd.Start(); err != nil {
		s.State = StateFailed
		return fmt.Errorf("service %s: failed to start: %w", s.Name, err)
	}

	s.State = StateRunning

	// Watch for unexpected exit - if process dies while still "running", mark as failed
	go func() {
		s.cmd.Wait()
		s.mu.Lock()
		if s.State == StateRunning {
			s.State = StateFailed
		}
		s.mu.Unlock()
	}()

	return nil
}

func (s *Service) Stop() error {
	s.mu.Lock()
	if s.State != StateRunning && s.State != StateStarting {
		s.mu.Unlock()
		return nil
	}
	s.State = StateStopping
	cmd := s.cmd
	s.mu.Unlock()

	cmd.Process.Signal(syscall.SIGTERM)

	time.AfterFunc(stopTimeout, func() {
		cmd.Process.Kill()
	})
	cmd.Wait()

	s.mu.Lock()
	s.State = StateStopped
	s.mu.Unlock()

	return nil
}

func (s *Service) buildEnv() []string {
	env := os.Environ()
	for k, v := range s.Config.Env {
		env = append(env, k+"="+v)
	}
	return env
}

func (s *Service) Logs() []string {
	if s.logs == nil {
		return nil
	}
	return s.logs.Lines()
}
