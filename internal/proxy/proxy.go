// Package proxy provides HTTPS reverse proxy setup with cert and DNS management.
package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/shahin-bayat/lokl/internal/config"
)

const (
	defaultPort     = 443
	defaultCertDir  = ".lokl/certs"
	shutdownTimeout = 5 * time.Second
)

type Proxy struct {
	cfg             *config.Config
	router          *router
	certs           *certManager
	sysDNS          *dnsManager
	handler         *handler
	server          *http.Server
	port            int
	hasWildcard     bool
	wildcardParents []string
	dnsPort         int
	dns             *dnsServer
}

func New(cfg *config.Config) *Proxy {
	r := newRouter(cfg)
	parents := collectWildcardParents(cfg)
	return &Proxy{
		cfg:             cfg,
		router:          r,
		certs:           newCertManager(defaultCertDir),
		sysDNS:          newDNSManager(cfg.Name, parents, defaultDNSPort),
		handler:         newHandler(r),
		port:            defaultPort,
		hasWildcard:     len(parents) > 0,
		wildcardParents: parents,
		dnsPort:         defaultDNSPort,
	}
}

func (p *Proxy) Setup() error {
	primary, sans := p.certDomains()
	if primary == "" {
		return fmt.Errorf("no proxy domain configured")
	}

	if err := p.certs.ensureCA(); err != nil {
		return fmt.Errorf("setting up CA: %w", err)
	}

	if _, _, err := p.certs.generate(primary, sans); err != nil {
		return fmt.Errorf("generating certificate: %w", err)
	}

	return nil
}

// certDomains returns the primary cert name and the full SAN list, deduped and ordered
// so base-domain-only configs produce the same output as before wildcard support.
func (p *Proxy) certDomains() (string, []string) {
	primary := p.router.domain()
	seen := map[string]struct{}{}
	var sans []string
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		sans = append(sans, s)
	}
	if primary != "" {
		add("*." + primary)
		add(primary)
	}
	for _, parent := range p.wildcardParents {
		add("*." + parent)
		add(parent)
	}
	for _, d := range p.router.domains() {
		add(d)
	}
	if primary == "" && len(p.wildcardParents) > 0 {
		primary = p.wildcardParents[0]
	}
	return primary, sans
}

func (p *Proxy) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p.port))
	if err != nil {
		return fmt.Errorf("binding port %d: %w", p.port, err)
	}

	primary, sans := p.certDomains()
	cert, err := tls.LoadX509KeyPair(p.certs.certPath(primary, sans), p.certs.keyPath(primary, sans))
	if err != nil {
		_ = ln.Close()
		return fmt.Errorf("loading certificate: %w", err)
	}

	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
	p.server = &http.Server{
		Handler:   p.handler,
		ErrorLog:  log.New(io.Discard, "", 0),
		TLSConfig: tlsCfg,
	}

	go func() {
		if err := p.server.ServeTLS(ln, "", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("proxy server error: %v", err)
		}
	}()

	if p.hasWildcard {
		srv, err := newDNSServer(fmt.Sprintf("127.0.0.1:%d", p.dnsPort), p.wildcardParents)
		if err != nil {
			_ = p.server.Shutdown(context.Background())
			p.server = nil
			return fmt.Errorf("dns server: %w", err)
		}
		if err := srv.Start(); err != nil {
			_ = p.server.Shutdown(context.Background())
			p.server = nil
			return fmt.Errorf("dns server on 127.0.0.1:%d: %w", p.dnsPort, err)
		}
		// TODO: surface p.dns.Err() to the supervisor so runtime DNS failures are visible.
		p.dns = srv
	}

	return nil
}

func collectWildcardParents(cfg *config.Config) []string {
	seen := map[string]struct{}{}
	var parents []string
	for _, svc := range cfg.Services {
		for _, sd := range svc.Subdomains {
			after, ok := strings.CutPrefix(sd, "*.")
			if !ok {
				continue
			}
			if _, dup := seen[after]; dup {
				continue
			}
			seen[after] = struct{}{}
			parents = append(parents, after)
		}
	}
	return parents
}

func (p *Proxy) Stop(cleanupDNS bool) error {
	var errs []error

	if p.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := p.server.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutting down server: %w", err))
		}
	}

	if p.dns != nil {
		if err := p.dns.Shutdown(); err != nil {
			errs = append(errs, fmt.Errorf("shutting down dns server: %w", err))
		}
		p.dns = nil
	}

	if cleanupDNS {
		if err := p.sysDNS.remove(); err != nil {
			errs = append(errs, fmt.Errorf("removing DNS entries: %w", err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (p *Proxy) Port() int {
	return p.port
}

func (p *Proxy) Domains() []string {
	return p.router.domains()
}

func (p *Proxy) CertDir() string {
	if abs, err := filepath.Abs(defaultCertDir); err == nil {
		return abs
	}
	return defaultCertDir
}

func (p *Proxy) NeedsSudo() bool {
	return p.sysDNS.needsSudo()
}

func (p *Proxy) UnresolvedDomains() []string {
	return p.sysDNS.unresolved(p.router.enabledDomains())
}

func (p *Proxy) DNSBlock() string {
	return p.sysDNS.block(p.router.enabledDomains())
}

func (p *Proxy) SetupDNS() error {
	return p.sysDNS.Setup(p.router.enabledDomains())
}

func (p *Proxy) RemoveDNS() error {
	return p.sysDNS.Remove()
}

func (p *Proxy) WildcardParentCount() int {
	return len(p.wildcardParents)
}

func (p *Proxy) EnableServiceProxy(name string) bool {
	if rt := p.router.byName[name]; rt != nil {
		p.handler.invalidateCache(rt.domain)
	}
	return p.router.setEnabled(name, true)
}

func (p *Proxy) DisableServiceProxy(name string) bool {
	if rt := p.router.byName[name]; rt != nil {
		p.handler.invalidateCache(rt.domain)
	}
	return p.router.setEnabled(name, false)
}

func (p *Proxy) IsServiceProxyEnabled(name string) bool {
	rt := p.router.byName[name]
	if rt == nil {
		return false
	}
	return rt.enabled.Load()
}
