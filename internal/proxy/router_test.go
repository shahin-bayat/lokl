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

	r := newRouter(cfg)

	if r.domain() != "example.com" {
		t.Errorf("domain() = %q, want %q", r.domain(), "example.com")
	}

	domains := r.domains()
	if len(domains) != 2 {
		t.Errorf("domains() len = %d, want 2", len(domains))
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

	r := newRouter(cfg)

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
			rt := r.match(tt.host)
			if tt.wantNil {
				if rt != nil {
					t.Errorf("match(%q) = %+v, want nil", tt.host, rt)
				}
				return
			}
			if rt == nil {
				t.Fatalf("match(%q) = nil, want route", tt.host)
			}
			if rt.port != tt.wantPort {
				t.Errorf("match(%q).port = %d, want %d", tt.host, rt.port, tt.wantPort)
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

	r := newRouter(cfg)
	rt := r.match("app.example.com")

	if rt.rewrite == nil {
		t.Fatal("rewrite is nil")
	}
	if rt.rewrite.stripPrefix != "web" {
		t.Errorf("stripPrefix = %q, want %q", rt.rewrite.stripPrefix, "web")
	}
	if rt.rewrite.fallback != "/index.html" {
		t.Errorf("fallback = %q, want %q", rt.rewrite.fallback, "/index.html")
	}
}

func TestRouterSetEnabled(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"web": {Subdomain: "app", Port: 8080},
		},
	}

	r := newRouter(cfg)

	// Initially enabled
	if rt := r.match("app.example.com"); rt == nil {
		t.Fatal("route should be enabled initially")
	}

	// Disable
	if !r.setEnabled("app.example.com", false) {
		t.Fatal("setEnabled returned false")
	}
	if rt := r.match("app.example.com"); rt == nil || rt.enabled {
		t.Error("route should exist but be disabled")
	}

	// Re-enable
	r.setEnabled("app.example.com", true)
	if rt := r.match("app.example.com"); rt == nil || !rt.enabled {
		t.Error("route should be enabled again")
	}

	// Unknown domain
	if r.setEnabled("unknown.example.com", false) {
		t.Error("setEnabled should return false for unknown domain")
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

	r := newRouter(cfg)

	if len(r.enabledDomains()) != 2 {
		t.Errorf("enabledDomains() len = %d, want 2", len(r.enabledDomains()))
	}

	r.setEnabled("app.example.com", false)

	if len(r.enabledDomains()) != 1 {
		t.Errorf("enabledDomains() len = %d, want 1", len(r.enabledDomains()))
	}
}
