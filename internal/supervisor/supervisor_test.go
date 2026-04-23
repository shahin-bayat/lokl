package supervisor

import (
	"fmt"
	"testing"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/types"
)

// --- Mocks ---

var _ ProcessRunner = (*mockRunner)(nil)

type mockRunner struct {
	StartFn     func() error
	StopFn      func() error
	IsRunningFn func() bool
	IsHealthyFn func() bool
	LogsFn      func() []string
}

func (m *mockRunner) Start() error {
	if m.StartFn != nil {
		return m.StartFn()
	}
	return nil
}

func (m *mockRunner) Stop() error {
	if m.StopFn != nil {
		return m.StopFn()
	}
	return nil
}

func (m *mockRunner) IsRunning() bool {
	if m.IsRunningFn != nil {
		return m.IsRunningFn()
	}
	return true
}

func (m *mockRunner) IsHealthy() bool {
	if m.IsHealthyFn != nil {
		return m.IsHealthyFn()
	}
	return true
}

func (m *mockRunner) Logs() []string {
	if m.LogsFn != nil {
		return m.LogsFn()
	}
	return nil
}

var _ ProxyManager = (*mockProxy)(nil)

type mockProxy struct {
	SetupFn                 func() error
	StartFn                 func() error
	StopFn                  func(bool) error
	CertDirFn               func() string
	PortFn                  func() int
	DomainsFn               func() []string
	UnresolvedDomainsFn     func() []string
	DNSBlockFn              func() string
	EnableServiceProxyFn    func(string) bool
	DisableServiceProxyFn   func(string) bool
	IsServiceProxyEnabledFn func(string) bool
}

func (m *mockProxy) Setup() error {
	if m.SetupFn != nil {
		return m.SetupFn()
	}
	return nil
}

func (m *mockProxy) Start() error {
	if m.StartFn != nil {
		return m.StartFn()
	}
	return nil
}

func (m *mockProxy) Stop(cleanup bool) error {
	if m.StopFn != nil {
		return m.StopFn(cleanup)
	}
	return nil
}

func (m *mockProxy) CertDir() string {
	if m.CertDirFn != nil {
		return m.CertDirFn()
	}
	return "/tmp/certs"
}

func (m *mockProxy) Port() int {
	if m.PortFn != nil {
		return m.PortFn()
	}
	return 443
}

func (m *mockProxy) Domains() []string {
	if m.DomainsFn != nil {
		return m.DomainsFn()
	}
	return nil
}

func (m *mockProxy) UnresolvedDomains() []string {
	if m.UnresolvedDomainsFn != nil {
		return m.UnresolvedDomainsFn()
	}
	return nil
}

func (m *mockProxy) DNSBlock() string {
	if m.DNSBlockFn != nil {
		return m.DNSBlockFn()
	}
	return ""
}

func (m *mockProxy) EnableServiceProxy(name string) bool {
	if m.EnableServiceProxyFn != nil {
		return m.EnableServiceProxyFn(name)
	}
	return true
}

func (m *mockProxy) DisableServiceProxy(name string) bool {
	if m.DisableServiceProxyFn != nil {
		return m.DisableServiceProxyFn(name)
	}
	return true
}

func (m *mockProxy) IsServiceProxyEnabled(name string) bool {
	if m.IsServiceProxyEnabledFn != nil {
		return m.IsServiceProxyEnabledFn(name)
	}
	return true
}

// --- Helpers ---

func shCmd(s string) config.StringOrSlice {
	return config.StringOrSlice{Args: []string{s}, Shell: true}
}

func newTestSupervisor(cfg *config.Config, factory ProcessFactory, proxy ProxyManager) *Supervisor {
	if proxy == nil {
		proxy = &mockProxy{}
	}
	return New(cfg, factory, proxy)
}

func simpleConfig(services map[string]config.Service) *config.Config {
	return &config.Config{
		Name:     "test-project",
		Services: services,
	}
}

func proxyConfig(services map[string]config.Service) *config.Config {
	return &config.Config{
		Name:     "test-project",
		Proxy:    config.ProxyConfig{Domain: "example.com"},
		Services: services,
	}
}

func boolPtr(v bool) *bool { return &v }

// --- Tests ---

func TestStartService(t *testing.T) {
	var created []string
	factory := func(name string, _ config.Service, _ func(), _ func()) ProcessRunner {
		created = append(created, name)
		return &mockRunner{}
	}

	cfg := simpleConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start")},
	})
	s := newTestSupervisor(cfg, factory, nil)

	if err := s.StartService("web"); err != nil {
		t.Fatalf("StartService: %v", err)
	}
	if len(created) != 1 || created[0] != "web" {
		t.Errorf("factory called with %v, want [web]", created)
	}
}

func TestStartServiceIdempotent(t *testing.T) {
	calls := 0
	factory := func(_ string, _ config.Service, _ func(), _ func()) ProcessRunner {
		calls++
		return &mockRunner{}
	}

	cfg := simpleConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start")},
	})
	s := newTestSupervisor(cfg, factory, nil)

	_ = s.StartService("web")
	_ = s.StartService("web")

	if calls != 1 {
		t.Errorf("factory called %d times, want 1 (idempotent)", calls)
	}
}

func TestStartServiceUnknown(t *testing.T) {
	cfg := simpleConfig(map[string]config.Service{})
	s := newTestSupervisor(cfg, nil, nil)

	err := s.StartService("ghost")
	if err == nil {
		t.Fatal("expected error for unknown service")
	}
}

func TestStartServiceStartError(t *testing.T) {
	factory := func(_ string, _ config.Service, _ func(), _ func()) ProcessRunner {
		return &mockRunner{
			StartFn: func() error { return errTest },
		}
	}

	cfg := simpleConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start")},
	})
	s := newTestSupervisor(cfg, factory, nil)

	if err := s.StartService("web"); err == nil {
		t.Fatal("expected error when runner.Start fails")
	}
	if _, ok := s.processes["web"]; ok {
		t.Error("failed process should not be in map")
	}
}

func TestStopService(t *testing.T) {
	stopped := false
	factory := func(_ string, _ config.Service, _ func(), _ func()) ProcessRunner {
		return &mockRunner{
			StopFn: func() error { stopped = true; return nil },
		}
	}

	cfg := simpleConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start")},
	})
	s := newTestSupervisor(cfg, factory, nil)

	_ = s.StartService("web")
	if err := s.StopService("web"); err != nil {
		t.Fatalf("StopService: %v", err)
	}
	if !stopped {
		t.Error("expected Stop called on runner")
	}
	if _, ok := s.processes["web"]; ok {
		t.Error("stopped process should be removed from map")
	}
}

func TestStopServiceNotRunning(t *testing.T) {
	cfg := simpleConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start")},
	})
	s := newTestSupervisor(cfg, nil, nil)

	if err := s.StopService("web"); err != nil {
		t.Fatalf("StopService on non-running should be no-op, got: %v", err)
	}
}

func TestRestartService(t *testing.T) {
	var sequence []string
	factory := func(_ string, _ config.Service, _ func(), _ func()) ProcessRunner {
		return &mockRunner{
			StartFn: func() error { sequence = append(sequence, "start"); return nil },
			StopFn:  func() error { sequence = append(sequence, "stop"); return nil },
		}
	}

	cfg := simpleConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start")},
	})
	s := newTestSupervisor(cfg, factory, nil)

	_ = s.StartService("web")
	sequence = nil

	if err := s.RestartService("web"); err != nil {
		t.Fatalf("RestartService: %v", err)
	}
	if len(sequence) != 2 || sequence[0] != "stop" || sequence[1] != "start" {
		t.Errorf("sequence = %v, want [stop start]", sequence)
	}
}

func TestStart(t *testing.T) {
	var started []string
	factory := func(name string, _ config.Service, _ func(), _ func()) ProcessRunner {
		return &mockRunner{
			StartFn: func() error { started = append(started, name); return nil },
		}
	}

	cfg := proxyConfig(map[string]config.Service{
		"db":  {Command: shCmd("postgres")},
		"web": {Command: shCmd("npm start"), DependsOn: []string{"db"}},
	})

	proxy := &mockProxy{
		DomainsFn: func() []string { return []string{"example.com"} },
	}
	s := newTestSupervisor(cfg, factory, proxy)

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(started) != 2 {
		t.Fatalf("started %d services, want 2", len(started))
	}
	if started[0] != "db" {
		t.Errorf("started[0] = %q, want db (dependency order)", started[0])
	}
}

func TestStartSkipsAutoStartFalse(t *testing.T) {
	var started []string
	factory := func(name string, _ config.Service, _ func(), _ func()) ProcessRunner {
		return &mockRunner{
			StartFn: func() error { started = append(started, name); return nil },
		}
	}

	cfg := proxyConfig(map[string]config.Service{
		"web":    {Command: shCmd("npm start")},
		"worker": {Command: shCmd("worker"), AutoStart: boolPtr(false)},
	})

	proxy := &mockProxy{
		DomainsFn: func() []string { return []string{"example.com"} },
	}
	s := newTestSupervisor(cfg, factory, proxy)

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	for _, name := range started {
		if name == "worker" {
			t.Error("worker should be skipped (AutoStart=false)")
		}
	}
}

func TestStartServiceFailureTriggersCleanup(t *testing.T) {
	var stopped []string
	callCount := 0
	factory := func(name string, _ config.Service, _ func(), _ func()) ProcessRunner {
		return &mockRunner{
			StartFn: func() error {
				callCount++
				if callCount == 2 {
					return errTest
				}
				return nil
			},
			StopFn: func() error {
				stopped = append(stopped, name)
				return nil
			},
		}
	}

	cfg := proxyConfig(map[string]config.Service{
		"db":  {Command: shCmd("postgres")},
		"web": {Command: shCmd("npm start"), DependsOn: []string{"db"}},
	})

	proxy := &mockProxy{
		DomainsFn: func() []string { return []string{"example.com"} },
	}
	s := newTestSupervisor(cfg, factory, proxy)

	err := s.Start()
	if err == nil {
		t.Fatal("expected error from Start")
	}
	if len(stopped) == 0 {
		t.Error("expected cleanup of previously started services")
	}
}

func TestStartNoProxy(t *testing.T) {
	var started []string
	factory := func(name string, _ config.Service, _ func(), _ func()) ProcessRunner {
		return &mockRunner{
			StartFn: func() error { started = append(started, name); return nil },
		}
	}

	cfg := simpleConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start")},
	})
	s := newTestSupervisor(cfg, factory, nil)

	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(started) != 1 {
		t.Errorf("started %d services, want 1", len(started))
	}
}

func TestStop(t *testing.T) {
	proxyStopped := false
	factory := func(_ string, _ config.Service, _ func(), _ func()) ProcessRunner {
		return &mockRunner{}
	}

	proxy := &mockProxy{
		StopFn: func(_ bool) error { proxyStopped = true; return nil },
	}

	cfg := simpleConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start")},
	})
	s := newTestSupervisor(cfg, factory, proxy)

	_ = s.StartService("web")
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !proxyStopped {
		t.Error("proxy not stopped")
	}
	if len(s.processes) != 0 {
		t.Errorf("processes map not empty after Stop: %d", len(s.processes))
	}
}

func TestToggleProxy(t *testing.T) {
	enabled := true
	proxy := &mockProxy{
		IsServiceProxyEnabledFn: func(_ string) bool { return enabled },
		DisableServiceProxyFn:   func(_ string) bool { enabled = false; return true },
		EnableServiceProxyFn:    func(_ string) bool { enabled = true; return true },
	}

	cfg := proxyConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start"), Subdomain: "app"},
	})
	s := newTestSupervisor(cfg, nil, proxy)

	// Toggle off
	got, err := s.ToggleProxy("web")
	if err != nil {
		t.Fatalf("ToggleProxy: %v", err)
	}
	if got != false {
		t.Error("expected false after disabling")
	}

	// Toggle on
	got, err = s.ToggleProxy("web")
	if err != nil {
		t.Fatalf("ToggleProxy: %v", err)
	}
	if got != true {
		t.Error("expected true after enabling")
	}
}

func TestToggleProxyUnknown(t *testing.T) {
	cfg := simpleConfig(map[string]config.Service{})
	s := newTestSupervisor(cfg, nil, nil)

	_, err := s.ToggleProxy("ghost")
	if err == nil {
		t.Fatal("expected error for unknown service")
	}
}

func TestToggleProxyNoRoute(t *testing.T) {
	proxy := &mockProxy{
		IsServiceProxyEnabledFn: func(_ string) bool { return false },
		EnableServiceProxyFn:    func(_ string) bool { return false },
	}

	cfg := simpleConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start")},
	})
	s := newTestSupervisor(cfg, nil, proxy)

	_, err := s.ToggleProxy("web")
	if err == nil {
		t.Fatal("expected error for service without proxy route")
	}
}

func TestServices(t *testing.T) {
	factory := func(_ string, _ config.Service, _ func(), _ func()) ProcessRunner {
		return &mockRunner{
			IsRunningFn: func() bool { return true },
			IsHealthyFn: func() bool { return true },
		}
	}

	proxy := &mockProxy{
		IsServiceProxyEnabledFn: func(_ string) bool { return true },
	}

	cfg := proxyConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start"), Port: 3000, Subdomain: "app"},
		"api": {Command: shCmd("go run ."), Port: 8080, Subdomain: "api"},
	})
	s := newTestSupervisor(cfg, factory, proxy)

	_ = s.StartService("web")
	_ = s.StartService("api")

	items := s.Services()
	if len(items) != 2 {
		t.Fatalf("Services() len = %d, want 2", len(items))
	}

	found := map[string]types.ServiceInfo{}
	for _, item := range items {
		found[item.Name] = item
	}

	web := found["web"]
	if !web.Running || !web.Healthy {
		t.Errorf("web: Running=%v Healthy=%v, want both true", web.Running, web.Healthy)
	}
	if web.Domain != "app.example.com" {
		t.Errorf("web.Domain = %q, want app.example.com", web.Domain)
	}
	if web.Port != 3000 {
		t.Errorf("web.Port = %d, want 3000", web.Port)
	}
}

func TestServicesWithPathPrefix(t *testing.T) {
	proxy := &mockProxy{
		IsServiceProxyEnabledFn: func(_ string) bool { return true },
	}

	cfg := proxyConfig(map[string]config.Service{
		"web": {
			Command:   shCmd("npm start"),
			Port:      3000,
			Subdomain: "app",
			Rewrite:   &config.RewriteConfig{StripPrefix: "web"},
		},
	})
	s := newTestSupervisor(cfg, nil, proxy)

	items := s.Services()
	if items[0].PathPrefix != "/web" {
		t.Errorf("PathPrefix = %q, want /web", items[0].PathPrefix)
	}
}

func TestServiceLogs(t *testing.T) {
	factory := func(_ string, _ config.Service, _ func(), _ func()) ProcessRunner {
		return &mockRunner{
			LogsFn: func() []string { return []string{"line1", "line2"} },
		}
	}

	cfg := simpleConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start")},
	})
	s := newTestSupervisor(cfg, factory, nil)

	_ = s.StartService("web")
	logs := s.ServiceLogs("web")
	if len(logs) != 2 {
		t.Errorf("ServiceLogs len = %d, want 2", len(logs))
	}
}

func TestServiceLogsNotRunning(t *testing.T) {
	cfg := simpleConfig(map[string]config.Service{
		"web": {Command: shCmd("npm start")},
	})
	s := newTestSupervisor(cfg, nil, nil)

	if logs := s.ServiceLogs("web"); logs != nil {
		t.Errorf("ServiceLogs for non-running = %v, want nil", logs)
	}
}

func TestProjectName(t *testing.T) {
	cfg := simpleConfig(nil)
	s := newTestSupervisor(cfg, nil, nil)

	if got := s.ProjectName(); got != "test-project" {
		t.Errorf("ProjectName() = %q, want test-project", got)
	}
}

func TestServiceDomain(t *testing.T) {
	cfg := proxyConfig(nil)
	s := newTestSupervisor(cfg, nil, nil)

	tests := []struct {
		name string
		svc  config.Service
		want string
	}{
		{"subdomain expansion", config.Service{Subdomain: "api"}, "api.example.com"},
		{"fqdn passthrough", config.Service{Subdomain: "api.other.com"}, "api.other.com"},
		{"empty subdomain", config.Service{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.serviceDomain(tt.svc)
			if got != tt.want {
				t.Errorf("serviceDomain() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestServiceDomainNoProxyDomain(t *testing.T) {
	cfg := simpleConfig(nil)
	s := newTestSupervisor(cfg, nil, nil)

	got := s.serviceDomain(config.Service{Subdomain: "api"})
	if got != "" {
		t.Errorf("serviceDomain() = %q, want empty (no proxy domain)", got)
	}
}

func TestEmit(t *testing.T) {
	cfg := simpleConfig(nil)
	s := newTestSupervisor(cfg, nil, nil)

	ch := s.Subscribe()
	s.emit(types.Event{Type: types.EventProgress, Service: "web", Message: "started"})

	select {
	case ev := <-ch:
		if ev.Service != "web" {
			t.Errorf("event.Service = %q, want web", ev.Service)
		}
		if ev.Message != "started" {
			t.Errorf("event.Message = %q, want started", ev.Message)
		}
	default:
		t.Fatal("expected event on channel")
	}
}

func TestEmitDropsWhenFull(t *testing.T) {
	cfg := simpleConfig(nil)
	s := newTestSupervisor(cfg, nil, nil)

	for range eventBufferSize {
		s.emit(types.Event{Type: types.EventProgress, Message: "fill"})
	}

	// Must not block
	s.emit(types.Event{Type: types.EventProgress, Message: "overflow"})
}

var errTest = fmt.Errorf("test error")
