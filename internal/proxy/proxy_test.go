package proxy

import (
	"net"
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
