package proxy

import (
	"reflect"
	"testing"

	"github.com/shahin-bayat/lokl/internal/config"
)

func TestCertDomains(t *testing.T) {
	t.Run("base domain only", func(t *testing.T) {
		cfg := &config.Config{
			Proxy: config.ProxyConfig{Domain: "test"},
			Services: map[string]config.Service{
				"a": {Port: 8000, Subdomains: config.Subdomains{"api"}},
			},
		}
		p := New(cfg)
		primary, sans := p.certDomains()
		if primary != "test" {
			t.Fatalf("primary=%q want test", primary)
		}
		want := []string{"*.test", "test", "api.test"}
		if !reflect.DeepEqual(sans, want) {
			t.Fatalf("sans=%v want %v", sans, want)
		}
	})

	t.Run("wildcards outside proxy.Domain", func(t *testing.T) {
		cfg := &config.Config{
			Proxy: config.ProxyConfig{Domain: "test"},
			Services: map[string]config.Service{
				"w": {Port: 8000, Subdomains: config.Subdomains{"*.sellify.shop", "sellify.shop"}},
			},
		}
		p := New(cfg)
		primary, sans := p.certDomains()
		if primary != "test" {
			t.Fatalf("primary=%q want test", primary)
		}
		want := []string{"*.test", "test", "*.sellify.shop", "sellify.shop"}
		if !reflect.DeepEqual(sans, want) {
			t.Fatalf("sans=%v want %v", sans, want)
		}
	})

	t.Run("no base domain, only wildcard", func(t *testing.T) {
		cfg := &config.Config{
			Services: map[string]config.Service{
				"w": {Port: 8000, Subdomains: config.Subdomains{"*.sellify.shop"}},
			},
		}
		p := New(cfg)
		primary, sans := p.certDomains()
		if primary != "sellify.shop" {
			t.Fatalf("primary=%q want sellify.shop (first wildcard parent)", primary)
		}
		want := []string{"*.sellify.shop", "sellify.shop"}
		if !reflect.DeepEqual(sans, want) {
			t.Fatalf("sans=%v want %v", sans, want)
		}
	})
}
