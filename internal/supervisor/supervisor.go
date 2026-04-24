// Package supervisor orchestrates services, proxy, and lifecycle management.
package supervisor

import (
	"fmt"
	"slices"
	"strings"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/runner"
	"github.com/shahin-bayat/lokl/internal/types"
)

// ProcessRunner defines what supervisor needs from a running process.
type ProcessRunner interface {
	Start() error
	Stop() error
	IsRunning() bool
	IsHealthy() bool
	Logs() []string
}

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
	EnableServiceProxy(name string) bool
	DisableServiceProxy(name string) bool
	IsServiceProxyEnabled(name string) bool
}

type DNSNotConfiguredError struct {
	Domains []string
}

func (e *DNSNotConfiguredError) Error() string {
	return fmt.Sprintf("DNS not configured for: %s\nRun: sudo lokl dns setup",
		strings.Join(e.Domains, ", "))
}

const eventBufferSize = 100

// ProcessFactory creates a new process runner.
type ProcessFactory func(name string, svc config.Service, onChange func(), onCrash func()) ProcessRunner

type Supervisor struct {
	cfg            *config.Config
	proxyManager   ProxyManager
	processFactory ProcessFactory
	processes      map[string]ProcessRunner
	events         chan types.Event
}

func New(cfg *config.Config, pf ProcessFactory, pm ProxyManager) *Supervisor {
	return &Supervisor{
		cfg:            cfg,
		proxyManager:   pm,
		processFactory: pf,
		processes:      make(map[string]ProcessRunner),
		events:         make(chan types.Event, eventBufferSize),
	}
}

func (s *Supervisor) Start() error {
	if s.cfg.Proxy.Domain != "" {
		if err := s.setupProxy(); err != nil {
			return err
		}
	}

	sorted, err := config.SortByDependency(s.cfg.Services)
	if err != nil {
		return fmt.Errorf("resolving dependencies: %w", err)
	}

	var started []string
	for _, name := range sorted {
		svc := s.cfg.Services[name]

		if svc.AutoStart != nil && !*svc.AutoStart {
			continue
		}

		if err := s.StartService(name); err != nil {
			s.cleanupStarted(started)
			return err
		}
		started = append(started, name)
		s.emit(types.Event{Type: types.EventProgress, Service: name, Message: "started"})
	}

	if s.cfg.Proxy.Domain != "" {
		if err := s.startProxy(); err != nil {
			s.cleanupStarted(started)
			return err
		}
	}

	return nil
}

func (s *Supervisor) Stop() error {
	for name := range s.processes {
		if err := s.StopService(name); err != nil {
			s.emit(types.Event{Type: types.EventError, Service: name, Message: fmt.Sprintf("failed to stop: %v", err)})
		} else {
			s.emit(types.Event{Type: types.EventProgress, Service: name, Message: "stopped"})
		}
	}

	if err := s.proxyManager.Stop(false); err != nil {
		return fmt.Errorf("stopping proxy: %w", err)
	}

	close(s.events)
	return nil
}

func (s *Supervisor) Subscribe() <-chan types.Event {
	return s.events
}

func (s *Supervisor) StartService(name string) error {
	svc, exists := s.cfg.Services[name]
	if !exists {
		return fmt.Errorf("unknown service: %s", name)
	}

	if _, running := s.processes[name]; running {
		return nil // already running, not an error
	}

	onChange := func() {
		s.emit(types.Event{Type: types.EventServiceStateChanged, Service: name})
	}
	onCrash := func() {
		go runner.Notify(s.cfg.Name, name)
	}
	p := s.processFactory(name, svc, onChange, onCrash)
	if err := p.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", name, err)
	}

	s.processes[name] = p
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

func (s *Supervisor) ToggleProxy(name string) (bool, error) {
	_, exists := s.cfg.Services[name]
	if !exists {
		return false, fmt.Errorf("unknown service: %s", name)
	}

	if s.proxyManager.IsServiceProxyEnabled(name) {
		s.proxyManager.DisableServiceProxy(name)
		return false, nil
	}

	if !s.proxyManager.EnableServiceProxy(name) {
		return false, fmt.Errorf("service %s has no proxy route", name)
	}
	return true, nil
}

func (s *Supervisor) Services() []types.ServiceInfo {
	sorted, _ := config.SortByDependency(s.cfg.Services)

	items := make([]types.ServiceInfo, 0, len(sorted))
	for _, name := range sorted {
		svc := s.cfg.Services[name]
		item := types.ServiceInfo{
			Name: name,
			Port: svc.Port,
		}

		if domain := s.serviceDomain(svc); domain != "" {
			item.Domain = domain
			item.ProxyEnabled = s.proxyManager.IsServiceProxyEnabled(name)
			if svc.Rewrite != nil {
				if trimmed := strings.Trim(svc.Rewrite.StripPrefix, "/"); trimmed != "" {
					item.PathPrefix = "/" + trimmed
				}
			}
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

func (s *Supervisor) ServiceLogs(name string) []string {
	if p, ok := s.processes[name]; ok {
		return p.Logs()
	}
	return nil
}

func (s *Supervisor) emit(ev types.Event) {
	select {
	case s.events <- ev:
	default:
	}
}

func (s *Supervisor) cleanupStarted(names []string) {
	for _, name := range slices.Backward(names) {
		if err := s.StopService(name); err != nil {
			s.emit(types.Event{Type: types.EventError, Service: name, Message: fmt.Sprintf("cleanup failed: %v", err)})
		}
	}
}

func (s *Supervisor) serviceDomain(svc config.Service) string {
	if len(svc.Subdomains) == 0 {
		return ""
	}
	sd := svc.Subdomains[0]
	if sd == "" {
		return ""
	}
	if strings.Contains(sd, ".") {
		return sd
	}
	if s.cfg.Proxy.Domain == "" {
		return ""
	}
	return sd + "." + s.cfg.Proxy.Domain
}

func (s *Supervisor) setupProxy() error {
	s.emit(types.Event{Type: types.EventProgress, Message: "setting up proxy"})

	if err := s.proxyManager.Setup(); err != nil {
		return fmt.Errorf("proxy setup: %w", err)
	}
	s.emit(types.Event{Type: types.EventProgress, Message: fmt.Sprintf("certificates ready in %s", s.proxyManager.CertDir())})

	unresolved := s.proxyManager.UnresolvedDomains()
	if len(unresolved) > 0 {
		return &DNSNotConfiguredError{Domains: unresolved}
	}

	s.emit(types.Event{Type: types.EventProgress, Message: fmt.Sprintf("DNS configured for %d domains", len(s.proxyManager.Domains()))})
	return nil
}

func (s *Supervisor) startProxy() error {
	if err := s.proxyManager.Start(); err != nil {
		return fmt.Errorf("proxy: %w", err)
	}

	s.emit(types.Event{Type: types.EventProgress, Message: fmt.Sprintf("proxy listening on :%d", s.proxyManager.Port())})
	return nil
}
