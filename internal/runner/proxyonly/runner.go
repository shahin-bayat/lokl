// Package proxyonly implements a supervisor.ProcessRunner for services that
// declare `proxy_only: true`. These services do not start a process or
// container; they only register a forwarding route and run a TCP health
// probe against 127.0.0.1:<port> so the TUI can reflect whether the target
// is reachable.
package proxyonly

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/runner"
)

const (
	defaultProbeInterval = 2 * time.Second
	defaultProbeTimeout  = 1 * time.Second
	defaultProbeRetries  = 3
)

type Runner struct {
	name     string
	svc      config.Service
	onChange func()
	// onCrash is kept for factory-signature parity with process/docker runners;
	// proxy-only services have no process, so it's never invoked.
	onCrash func()

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	healthy atomic.Bool
}

func New(name string, svc config.Service, onChange, onCrash func()) *Runner {
	return &Runner{
		name:     name,
		svc:      svc,
		onChange: onChange,
		onCrash:  onCrash,
	}
}

func (r *Runner) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.running = true
	r.cancel = cancel
	r.mu.Unlock()

	go r.probeLoop(ctx)
	r.onChange()
	return nil
}

func (r *Runner) Stop() error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = false
	cancel := r.cancel
	r.cancel = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	r.healthy.Store(false)
	r.onChange()
	return nil
}

func (r *Runner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *Runner) IsHealthy() bool {
	return r.healthy.Load()
}

func (r *Runner) Logs() []string {
	return []string{
		fmt.Sprintf("proxy-only service · forwarding to 127.0.0.1:%d", r.svc.Port),
		"no process output",
	}
}

func (r *Runner) probeTimings() (interval, timeout time.Duration, retries int) {
	interval = defaultProbeInterval
	timeout = defaultProbeTimeout
	retries = defaultProbeRetries

	if r.svc.Health == nil {
		return
	}
	if r.svc.Health.Interval != "" {
		if d, err := time.ParseDuration(r.svc.Health.Interval); err == nil {
			interval = d
		}
	}
	if r.svc.Health.Timeout != "" {
		if d, err := time.ParseDuration(r.svc.Health.Timeout); err == nil {
			timeout = d
		}
	}
	if r.svc.Health.Retries != nil {
		retries = *r.svc.Health.Retries
	}
	return
}

func (r *Runner) probeLoop(ctx context.Context) {
	interval, timeout, retries := r.probeTimings()

	onChange := func(healthy bool) {
		was := r.healthy.Load()
		r.healthy.Store(healthy)
		if was != healthy {
			r.onChange()
		}
	}

	if r.svc.Health != nil && r.svc.Health.Path != "" {
		runner.RunHealthCheck(ctx, r.svc.Port, r.svc.Health.Path, interval, timeout, retries, onChange)
		return
	}

	probe := func() bool {
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", r.svc.Port))
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}
	runner.RunProbe(ctx, probe, interval, retries, onChange)
}
