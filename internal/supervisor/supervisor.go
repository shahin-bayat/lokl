// Package supervisor orchestrates services, proxy, and lifecycle management.
package supervisor

import (
	"fmt"
	"strings"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/types"
)

// ProcessRunner defines what supervisor needs from a running process.
type ProcessRunner interface {
	Start() error
	Stop() error
	IsRunning() bool
	IsHealthy() bool
}

// ProcessFactory creates a new process runner.
type ProcessFactory func(name string, svc config.Service) ProcessRunner

// ProxyManager defines what supervisor needs from the reverse proxy.
type ProxyManager interface {
	Setup() error
	Start() error
	Stop(cleanupDNS bool) error
	CertDir() string
	Port() int
	Domains() []string
	UnresolvedDomains() []string
	DNSBlock() string
}

type Supervisor struct {
	cfg        *config.Config
	proxy      ProxyManager
	newProcess ProcessFactory
	processes  map[string]ProcessRunner
	log        Logger
}

func New(cfg *config.Config, pf ProcessFactory, pm ProxyManager, log Logger) *Supervisor {
	return &Supervisor{
		cfg:        cfg,
		proxy:      pm,
		newProcess: pf,
		processes:  make(map[string]ProcessRunner),
		log:        log,
	}
}

func (s *Supervisor) Start() error {
	if err := s.setupProxy(); err != nil {
		return err
	}

	startSequence, err := config.SortByDependency(s.cfg.Services)
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
		s.log.Infof("✓ Started %s\n", name)
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

	p := s.newProcess(name, svc)
	if err := p.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", name, err)
	}

	s.processes[name] = p
	return nil
}

func (s *Supervisor) Stop() error {
	for name := range s.processes {
		if err := s.StopService(name); err != nil {
			s.log.Errorf("✗ Failed to stop %s: %v\n", name, err)
		} else {
			s.log.Infof("✓ Stopped %s\n", name)
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
	order, _ := config.SortByDependency(s.cfg.Services)

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
			item.Running = p.IsRunning()
			item.Healthy = p.IsHealthy()
		}

		items = append(items, item)
	}

	return items
}

func (s *Supervisor) ProjectName() string {
	return s.cfg.Name
}

func (s *Supervisor) setupProxy() error {
	if s.cfg.Proxy.Domain == "" {
		return nil
	}

	s.log.Infof("Setting up proxy...\n")

	if err := s.proxy.Setup(); err != nil {
		return fmt.Errorf("proxy setup: %w", err)
	}
	s.log.Infof("✓ Certificates ready in %s\n", s.proxy.CertDir())

	unresolved := s.proxy.UnresolvedDomains()
	if len(unresolved) > 0 {
		s.log.Infof("\n⚠ DNS entries needed for: %s\n", strings.Join(unresolved, ", "))
		s.log.Infof("\nOption 1 - Run:\n")
		s.log.Infof("  sudo lokl dns setup\n")
		s.log.Infof("\nOption 2 - Add manually to /etc/hosts:\n")
		s.log.Infof("  %s\n", strings.ReplaceAll(s.proxy.DNSBlock(), "\n", "\n  "))
		return fmt.Errorf("DNS not configured")
	}

	s.log.Infof("✓ DNS configured for %d domains\n", len(s.proxy.Domains()))
	return nil
}

func (s *Supervisor) startProxy() error {
	if s.cfg.Proxy.Domain == "" {
		return nil
	}

	go func() {
		if err := s.proxy.Start(); err != nil && err.Error() != "http: Server closed" {
			s.log.Errorf("✗ Proxy error: %v\n", err)
		}
	}()

	s.log.Infof("✓ Proxy listening on :%d\n", s.proxy.Port())
	return nil
}
