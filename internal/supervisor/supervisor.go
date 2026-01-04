// Package supervisor orchestrates services, proxy, and lifecycle management.
package supervisor

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/process"
	"github.com/shahin-bayat/lokl/internal/proxy"
)

type Supervisor struct {
	cfg       *config.Config
	proxy     *proxy.Proxy
	processes map[string]*process.Process
}

func New(cfg *config.Config) *Supervisor {
	return &Supervisor{
		cfg:       cfg,
		proxy:     proxy.New(cfg),
		processes: make(map[string]*process.Process),
	}
}

func (s *Supervisor) Start() error {
	if err := s.setupProxy(); err != nil {
		return err
	}

	startSequence, err := process.SortByDependency(s.cfg.Services)
	if err != nil {
		return fmt.Errorf("resolving dependencies: %w", err)
	}

	for _, name := range startSequence {
		svc := s.cfg.Services[name]

		if svc.AutoStart != nil && !*svc.AutoStart {
			continue
		}

		if svc.Image != "" {
			continue
		}

		p := process.New(name, svc)

		if err := p.Start(); err != nil {
			return fmt.Errorf("starting %s: %w", name, err)
		}

		s.processes[name] = p
		fmt.Printf("  ✓ Started %s\n", name)
	}

	if err := s.startProxy(); err != nil {
		return err
	}

	return nil
}

func (s *Supervisor) Stop() error {
	for name, p := range s.processes {
		if err := p.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Failed to stop %s: %v\n", name, err)
		} else {
			fmt.Printf("  ✓ Stopped %s\n", name)
		}
	}

	if err := s.proxy.Stop(true); err != nil {
		return fmt.Errorf("stopping proxy: %w", err)
	}

	return nil
}

func (s *Supervisor) Wait() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\nShutting down...")
}

func (s *Supervisor) setupProxy() error {
	if s.cfg.Proxy.Domain == "" {
		return nil
	}

	fmt.Println("  Setting up proxy...")

	if s.proxy.NeedsSudo() {
		fmt.Println("  ⚠ DNS configuration requires sudo")
	}

	if err := s.proxy.Setup(); err != nil {
		return fmt.Errorf("proxy setup: %w", err)
	}

	fmt.Printf("  ✓ Certificates ready in %s\n", s.proxy.CertDir())
	fmt.Printf("  ✓ DNS configured for %d domains\n", len(s.proxy.Domains()))

	return nil
}

func (s *Supervisor) startProxy() error {
	if s.cfg.Proxy.Domain == "" {
		return nil
	}

	go func() {
		if err := s.proxy.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Proxy error: %v\n", err)
		}
	}()

	fmt.Printf("  ✓ Proxy listening on :%d\n", s.proxy.Port())
	return nil
}

func (s *Supervisor) Processes() map[string]*process.Process {
	return s.processes
}

func (s *Supervisor) Config() *config.Config {
	return s.cfg
}
