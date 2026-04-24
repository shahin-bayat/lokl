package proxy

import (
	"testing"

	"github.com/shahin-bayat/lokl/internal/config"
)

func TestNewRouter(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"web":          {Subdomains: config.Subdomains{"app"}, Port: 8080},
			"api":          {Subdomains: config.Subdomains{"api"}, Port: 3000},
			"no-subdomain": {Port: 5000},                               // should be skipped
			"no-port":      {Subdomains: config.Subdomains{"ignored"}}, // should be skipped
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
				Subdomains: config.Subdomains{"app"},
				Port:       8080,
				Rewrite: &config.RewriteConfig{
					StripPrefix: "web",
					Fallback:    "/index.html",
				},
			},
			"api": {Subdomains: config.Subdomains{"api.example.com"}, Port: 3000}, // FQDN already
		},
	}

	r := newRouter(cfg)

	tests := []struct {
		name     string
		host     string
		path     string
		wantNil  bool
		wantPort int
	}{
		{"subdomain", "app.example.com", "/", false, 8080},
		{"with port", "app.example.com:8443", "/", false, 8080},
		{"fqdn", "api.example.com", "/", false, 3000},
		{"unknown", "unknown.example.com", "/", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := r.match(tt.host, tt.path)
			if tt.wantNil {
				if rt != nil {
					t.Errorf("match(%q, %q) = %+v, want nil", tt.host, tt.path, rt)
				}
				return
			}
			if rt == nil {
				t.Fatalf("match(%q, %q) = nil, want route", tt.host, tt.path)
			}
			if rt.port != tt.wantPort {
				t.Errorf("match(%q, %q).port = %d, want %d", tt.host, tt.path, rt.port, tt.wantPort)
			}
		})
	}
}

func TestRouterMatchWithRewrite(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"web": {
				Subdomains: config.Subdomains{"app"},
				Port:       8080,
				Rewrite: &config.RewriteConfig{
					StripPrefix: "web",
					Fallback:    "/index.html",
				},
			},
		},
	}

	r := newRouter(cfg)
	rt := r.match("app.example.com", "/web/dashboard")

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
			"web": {Subdomains: config.Subdomains{"app"}, Port: 8080},
		},
	}

	r := newRouter(cfg)

	if rt := r.match("app.example.com", "/"); rt == nil {
		t.Fatal("route should exist initially")
	}

	if !r.setEnabled("web", false) {
		t.Fatal("setEnabled returned false")
	}
	if rt := r.match("app.example.com", "/"); rt == nil || rt.enabled.Load() {
		t.Error("route should exist but be disabled")
	}

	r.setEnabled("web", true)
	if rt := r.match("app.example.com", "/"); rt == nil || !rt.enabled.Load() {
		t.Error("route should be enabled again")
	}

	if r.setEnabled("unknown", false) {
		t.Error("setEnabled should return false for unknown service")
	}
}

func TestRouterEnabledDomains(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"web": {Subdomains: config.Subdomains{"app"}, Port: 8080},
			"api": {Subdomains: config.Subdomains{"api"}, Port: 3000},
		},
	}

	r := newRouter(cfg)

	if len(r.enabledDomains()) != 2 {
		t.Errorf("enabledDomains() len = %d, want 2", len(r.enabledDomains()))
	}

	r.setEnabled("web", false)

	if len(r.enabledDomains()) != 1 {
		t.Errorf("enabledDomains() len = %d, want 1", len(r.enabledDomains()))
	}
}

func TestRouterMatchSharedSubdomain(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"funnel": {
				Subdomains: config.Subdomains{"client"},
				Port:       8080,
				Rewrite:    &config.RewriteConfig{StripPrefix: "customer-funnel"},
			},
			"activation": {
				Subdomains: config.Subdomains{"client"},
				Port:       8083,
				Rewrite:    &config.RewriteConfig{StripPrefix: "activation/waipu"},
			},
		},
	}

	r := newRouter(cfg)

	tests := []struct {
		name     string
		path     string
		wantPort int
	}{
		{"funnel path", "/customer-funnel/dashboard", 8080},
		{"funnel exact", "/customer-funnel", 8080},
		{"activation path", "/activation/waipu/callback", 8083},
		{"activation exact", "/activation/waipu", 8083},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := r.match("client.example.com", tt.path)
			if rt == nil {
				t.Fatalf("match returned nil for path %q", tt.path)
			}
			if rt.port != tt.wantPort {
				t.Errorf("port = %d, want %d", rt.port, tt.wantPort)
			}
		})
	}
}

func TestRouterMatchSegmentSafe(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"api": {
				Subdomains: config.Subdomains{"app"},
				Port:       3000,
				Rewrite:    &config.RewriteConfig{StripPrefix: "api"},
			},
			"api2": {
				Subdomains: config.Subdomains{"app"},
				Port:       3001,
				Rewrite:    &config.RewriteConfig{StripPrefix: "api2"},
			},
		},
	}

	r := newRouter(cfg)

	if rt := r.match("app.example.com", "/api/users"); rt == nil || rt.port != 3000 {
		t.Error("/api/users should match api service")
	}
	if rt := r.match("app.example.com", "/api"); rt == nil || rt.port != 3000 {
		t.Error("/api should match api service")
	}
	if rt := r.match("app.example.com", "/api2/data"); rt == nil || rt.port != 3001 {
		t.Error("/api2/data should match api2 service")
	}
}

func TestRouterMatchNoPrefix404(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"a": {
				Subdomains: config.Subdomains{"client"},
				Port:       8080,
				Rewrite:    &config.RewriteConfig{StripPrefix: "app-a"},
			},
			"b": {
				Subdomains: config.Subdomains{"client"},
				Port:       8081,
				Rewrite:    &config.RewriteConfig{StripPrefix: "app-b"},
			},
		},
	}

	r := newRouter(cfg)

	if rt := r.match("client.example.com", "/unknown"); rt != nil {
		t.Errorf("unmatched path should return nil, got port %d", rt.port)
	}
}

func TestRouterSetEnabledByName(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"funnel": {
				Subdomains: config.Subdomains{"client"},
				Port:       8080,
				Rewrite:    &config.RewriteConfig{StripPrefix: "customer-funnel"},
			},
			"activation": {
				Subdomains: config.Subdomains{"client"},
				Port:       8083,
				Rewrite:    &config.RewriteConfig{StripPrefix: "activation/waipu"},
			},
		},
	}

	r := newRouter(cfg)

	r.setEnabled("funnel", false)

	rt := r.match("client.example.com", "/customer-funnel/page")
	if rt == nil {
		t.Fatal("disabled route should still be returned")
	}
	if rt.enabled.Load() {
		t.Error("funnel should be disabled")
	}

	rt = r.match("client.example.com", "/activation/waipu/page")
	if rt == nil {
		t.Fatal("activation route should exist")
	}
	if !rt.enabled.Load() {
		t.Error("activation should still be enabled")
	}
}

func TestRouterEnabledDomainsSharedSubdomain(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "example.com"},
		Services: map[string]config.Service{
			"a": {
				Subdomains: config.Subdomains{"client"},
				Port:       8080,
				Rewrite:    &config.RewriteConfig{StripPrefix: "app-a"},
			},
			"b": {
				Subdomains: config.Subdomains{"client"},
				Port:       8081,
				Rewrite:    &config.RewriteConfig{StripPrefix: "app-b"},
			},
		},
	}

	r := newRouter(cfg)

	if len(r.enabledDomains()) != 1 {
		t.Errorf("shared subdomain should count as 1 domain, got %d", len(r.enabledDomains()))
	}

	r.setEnabled("a", false)
	if len(r.enabledDomains()) != 1 {
		t.Error("domain should still be enabled (b is still enabled)")
	}

	r.setEnabled("b", false)
	if len(r.enabledDomains()) != 0 {
		t.Error("domain should be disabled (both routes disabled)")
	}
}

func TestRouterBuildsWildcards(t *testing.T) {
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: ""},
		Services: map[string]config.Service{
			"web": {
				Port:       8000,
				Subdomains: config.Subdomains{"sellify.shop", "*.sellify.shop"},
			},
		},
	}
	r := newRouter(cfg)

	if _, ok := r.byHost["sellify.shop"]; !ok {
		t.Fatal("apex route missing from byHost")
	}
	if len(r.wildcards) != 1 {
		t.Fatalf("want 1 wildcard, got %d", len(r.wildcards))
	}
	if r.wildcards[0].parent != "sellify.shop" {
		t.Fatalf("parent=%q", r.wildcards[0].parent)
	}
	if r.wildcards[0].name != "web" {
		t.Fatalf("name=%q", r.wildcards[0].name)
	}
	if r.wildcards[0].port != 8000 {
		t.Fatalf("port=%d", r.wildcards[0].port)
	}
	if !r.wildcards[0].enabled.Load() {
		t.Fatal("wildcard route should default to enabled")
	}
}

func TestRouterWildcardSortByParentLength(t *testing.T) {
	cfg := &config.Config{
		Services: map[string]config.Service{
			"a": {Port: 8000, Subdomains: config.Subdomains{"*.sellify.shop"}},
			"b": {Port: 8001, Subdomains: config.Subdomains{"*.api.sellify.shop"}},
		},
	}
	r := newRouter(cfg)
	if len(r.wildcards) != 2 {
		t.Fatalf("want 2 wildcards, got %d", len(r.wildcards))
	}
	if r.wildcards[0].parent != "api.sellify.shop" {
		t.Fatalf("wildcards[0].parent=%q, want longer parent first", r.wildcards[0].parent)
	}
}

func TestRouterMatchWildcard(t *testing.T) {
	cfg := &config.Config{
		Services: map[string]config.Service{
			"web":      {Port: 8000, Subdomains: config.Subdomains{"sellify.shop", "*.sellify.shop"}},
			"api":      {Port: 9000, Subdomains: config.Subdomains{"api.sellify.shop"}},
			"api-deep": {Port: 9100, Subdomains: config.Subdomains{"*.api.sellify.shop"}},
		},
	}
	r := newRouter(cfg)

	cases := []struct {
		host, want string
	}{
		{"sellify.shop", "web"},
		{"api.sellify.shop", "api"},
		{"foo.sellify.shop", "web"},
		{"a.b.sellify.shop", "web"},
		{"deep.api.sellify.shop", "api-deep"},
		{"evil-sellify.shop", ""},
		{"sellify.shop.attacker.com", ""},
		{"nope.other.test", ""},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			got := ""
			if rt := r.match(tc.host, "/"); rt != nil {
				got = rt.name
			}
			if got != tc.want {
				t.Fatalf("match(%q)=%q want %q", tc.host, got, tc.want)
			}
		})
	}
}

func TestRouterMatchWildcardDisabled(t *testing.T) {
	cfg := &config.Config{
		Services: map[string]config.Service{
			"web": {Port: 8000, Subdomains: config.Subdomains{"*.sellify.shop"}},
		},
	}
	r := newRouter(cfg)
	r.wildcards[0].enabled.Store(false)
	if rt := r.match("foo.sellify.shop", "/"); rt != nil {
		t.Fatalf("disabled wildcard matched: %q", rt.name)
	}
}
