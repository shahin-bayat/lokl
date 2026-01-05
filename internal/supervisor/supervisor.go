// Package supervisor orchestrates services, proxy, and lifecycle management.
package supervisor

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/process"
	"github.com/shahin-bayat/lokl/internal/proxy"
	"github.com/shahin-bayat/lokl/internal/types"
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

		if err := s.StartService(name); err != nil {
			return err
		}
		fmt.Printf("  ✓ Started %s\n", name)
	}

	if err := s.startProxy(); err != nil {
		return err
	}

	return nil
}

func (s *Supervisor) StartService(name string) error {
	svc, exists := s.cfg.Services[name]
	if !exists {
		return fmt.Errorf("unknown service: %s", name)
	}

	if _, running := s.processes[name]; running {
		return nil // already running, not an error
	}

	if svc.Image != "" {
		return fmt.Errorf("docker services not yet supported")
	}

	p := process.New(name, svc)
	if err := p.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", name, err)
	}

	s.processes[name] = p
	return nil
}

func (s *Supervisor) Stop() error {
	for name := range s.processes {
		if err := s.StopService(name); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Failed to stop %s: %v\n", name, err)
		} else {
			fmt.Printf("  ✓ Stopped %s\n", name)
		}
	}

	if err := s.proxy.Stop(false); err != nil {
		return fmt.Errorf("stopping proxy: %w", err)
	}

	return nil
}

func (s *Supervisor) StopService(name string) error {
	p, exists := s.processes[name]
	if !exists {
		return nil
	}

	if err := p.Stop(); err != nil {
		return fmt.Errorf("stopping %s: %w", name, err)
	}

	delete(s.processes, name)
	return nil
}

func (s *Supervisor) RestartService(name string) error {
	if err := s.StopService(name); err != nil {
		return err
	}
	return s.StartService(name)
}

func (s *Supervisor) Services() []types.ServiceInfo {
	order, _ := process.SortByDependency(s.cfg.Services)

	items := make([]types.ServiceInfo, 0, len(order))
	for _, name := range order {
		svc := s.cfg.Services[name]
		item := types.ServiceInfo{
			Name: name,
			Port: svc.Port,
		}

		if svc.Subdomain != "" && s.cfg.Proxy.Domain != "" {
			item.Domain = svc.Subdomain + "." + s.cfg.Proxy.Domain
		}

		if p, ok := s.processes[name]; ok {
			item.Running = p.State == process.StateRunning
			item.Healthy = p.Healthy
		}

		items = append(items, item)
	}

	return items
}

func (s *Supervisor) ProjectName() string {
	return s.cfg.Name
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

	if err := s.proxy.Setup(); err != nil {
		return fmt.Errorf("proxy setup: %w", err)
	}
	fmt.Printf("  ✓ Certificates ready in %s\n", s.proxy.CertDir())

	unresolved := s.proxy.UnresolvedDomains()
	if len(unresolved) > 0 {
		fmt.Printf("\n  ⚠ DNS entries needed for: %s\n", strings.Join(unresolved, ", "))
		fmt.Println("\n  Option 1 - Run:")
		fmt.Println("    sudo lokl dns setup")
		fmt.Println("\n  Option 2 - Add manually to /etc/hosts:")
		fmt.Printf("    %s\n", strings.ReplaceAll(s.proxy.DNSBlock(), "\n", "\n    "))
		return fmt.Errorf("DNS not configured")
	}

	fmt.Printf("  ✓ DNS configured for %d domains\n", len(s.proxy.Domains()))
	return nil
}

func (s *Supervisor) startProxy() error {
	if s.cfg.Proxy.Domain == "" {
		return nil
	}

	go func() {
		if err := s.proxy.Start(); err != nil && err.Error() != "http: Server closed" {
			fmt.Fprintf(os.Stderr, "  ✗ Proxy error: %v\n", err)
		}
	}()

	fmt.Printf("  ✓ Proxy listening on :%d\n", s.proxy.Port())
	return nil
}
