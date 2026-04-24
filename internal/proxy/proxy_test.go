package proxy

import (
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/shahin-bayat/lokl/internal/config"
)

func TestStartPortConflict(t *testing.T) {
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("pre-bind: %v", err)
	}
	defer func() { _ = ln.Close() }()

	port := ln.Addr().(*net.TCPAddr).Port

	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "test.dev"},
		Services: map[string]config.Service{
			"svc": {Port: 9999, Subdomains: config.Subdomains{"app"}},
		},
	}
	p := New(cfg)
	p.port = port

	err = p.Start()
	if err == nil {
		t.Fatal("expected error when port is already in use")
	}
	if !strings.Contains(err.Error(), "binding port") {
		t.Fatalf("expected 'binding port' error, got: %v", err)
	}
}

func TestWildcardParentsDeterministic(t *testing.T) {
	cfgA := &config.Config{Services: map[string]config.Service{
		"w": {Port: 8000, Subdomains: config.Subdomains{"*.b.test", "*.a.test"}},
	}}
	cfgB := &config.Config{Services: map[string]config.Service{
		"w": {Port: 8000, Subdomains: config.Subdomains{"*.a.test", "*.b.test"}},
	}}
	a := collectWildcardParents(cfgA)
	b := collectWildcardParents(cfgB)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("order differs: a=%v b=%v", a, b)
	}
}

func TestProxyDetectsWildcard(t *testing.T) {
	t.Run("no wildcard", func(t *testing.T) {
		cfg := &config.Config{
			Proxy:    config.ProxyConfig{Domain: "test"},
			Services: map[string]config.Service{"a": {Port: 1234, Subdomains: config.Subdomains{"a.test"}}},
		}
		p := New(cfg)
		if p.hasWildcard {
			t.Fatal("hasWildcard should be false for exact-only config")
		}
		if len(p.wildcardParents) != 0 {
			t.Fatalf("want 0 parents, got %v", p.wildcardParents)
		}
	})

	t.Run("with wildcard", func(t *testing.T) {
		cfg := &config.Config{
			Proxy: config.ProxyConfig{Domain: "test"},
			Services: map[string]config.Service{
				"a": {Port: 1234, Subdomains: config.Subdomains{"*.sellify.shop"}},
				"b": {Port: 5678, Subdomains: config.Subdomains{"*.sellify.shop", "*.sellify.dev"}},
			},
		}
		p := New(cfg)
		if !p.hasWildcard {
			t.Fatal("hasWildcard should be true")
		}
		want := map[string]bool{"sellify.shop": true, "sellify.dev": true}
		got := map[string]bool{}
		for _, parent := range p.wildcardParents {
			got[parent] = true
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("parents=%v want=%v", p.wildcardParents, want)
		}
	})
}
