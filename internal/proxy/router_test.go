package proxy

import (
	"testing"

	"github.com/shahin-bayat/lokl/internal/config"
)

func TestNewRouter(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"web":          {Subdomain: "app", Port: 8080},
			"api":          {Subdomain: "api", Port: 3000},
			"no-subdomain": {Port: 5000},           // should be skipped
			"no-port":      {Subdomain: "ignored"}, // should be skipped
		},
	}

	r := NewRouter(cfg)

	if r.Domain() != "example.com" {
		t.Errorf("Domain() = %q, want %q", r.Domain(), "example.com")
	}

	domains := r.Domains()
	if len(domains) != 2 {
		t.Errorf("Domains() len = %d, want 2", len(domains))
	}
}

func TestRouterMatch(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"web": {
				Subdomain: "app",
				Port:      8080,
				Rewrite: &config.RewriteConfig{
					StripPrefix: "web",
					Fallback:    "/index.html",
				},
			},
			"api": {Subdomain: "api.example.com", Port: 3000}, // FQDN already
		},
	}

	r := NewRouter(cfg)

	tests := []struct {
		name     string
		host     string
		wantNil  bool
		wantPort int
	}{
		{"subdomain", "app.example.com", false, 8080},
		{"with port", "app.example.com:8443", false, 8080},
		{"fqdn", "api.example.com", false, 3000},
		{"unknown", "unknown.example.com", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			route := r.Match(tt.host)
			if tt.wantNil {
				if route != nil {
					t.Errorf("Match(%q) = %+v, want nil", tt.host, route)
				}
				return
			}
			if route == nil {
				t.Fatalf("Match(%q) = nil, want route", tt.host)
			}
			if route.Port != tt.wantPort {
				t.Errorf("Match(%q).Port = %d, want %d", tt.host, route.Port, tt.wantPort)
			}
		})
	}
}

func TestRouterMatchWithRewrite(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"web": {
				Subdomain: "app",
				Port:      8080,
				Rewrite: &config.RewriteConfig{
					StripPrefix: "web",
					Fallback:    "/index.html",
				},
			},
		},
	}

	r := NewRouter(cfg)
	route := r.Match("app.example.com")

	if route.Rewrite == nil {
		t.Fatal("Rewrite is nil")
	}
	if route.Rewrite.StripPrefix != "web" {
		t.Errorf("StripPrefix = %q, want %q", route.Rewrite.StripPrefix, "web")
	}
	if route.Rewrite.Fallback != "/index.html" {
		t.Errorf("Fallback = %q, want %q", route.Rewrite.Fallback, "/index.html")
	}
}

func TestRouterSetEnabled(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"web": {Subdomain: "app", Port: 8080},
		},
	}

	r := NewRouter(cfg)

	// Initially enabled
	if route := r.Match("app.example.com"); route == nil {
		t.Fatal("route should be enabled initially")
	}

	// Disable
	if !r.SetEnabled("app.example.com", false) {
		t.Fatal("SetEnabled returned false")
	}
	if route := r.Match("app.example.com"); route != nil {
		t.Error("route should be nil when disabled")
	}

	// Re-enable
	r.SetEnabled("app.example.com", true)
	if route := r.Match("app.example.com"); route == nil {
		t.Error("route should be enabled again")
	}

	// Unknown domain
	if r.SetEnabled("unknown.example.com", false) {
		t.Error("SetEnabled should return false for unknown domain")
	}
}

func TestRouterEnabledDomains(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"web": {Subdomain: "app", Port: 8080},
			"api": {Subdomain: "api", Port: 3000},
		},
	}

	r := NewRouter(cfg)

	if len(r.EnabledDomains()) != 2 {
		t.Errorf("EnabledDomains() len = %d, want 2", len(r.EnabledDomains()))
	}

	r.SetEnabled("app.example.com", false)

	if len(r.EnabledDomains()) != 1 {
		t.Errorf("EnabledDomains() len = %d, want 1", len(r.EnabledDomains()))
	}
}
