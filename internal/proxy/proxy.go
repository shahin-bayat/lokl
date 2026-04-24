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
	hosts           *hostsManager
	handler         *handler
	server          *http.Server
	port            int
	hasWildcard     bool
	wildcardParents []string
	dnsPort         int
	dns             *dnsServer
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

func New(cfg *config.Config) *Proxy {
	r := newRouter(cfg)
	parents := collectWildcardParents(cfg)
	return &Proxy{
		cfg:             cfg,
		router:          r,
		certs:           newCertManager(defaultCertDir),
		hosts:           newHostsManager(cfg.Name),
		handler:         newHandler(r),
		port:            defaultPort,
		hasWildcard:     len(parents) > 0,
		wildcardParents: parents,
		dnsPort:         defaultDNSPort,
	}
}

func (p *Proxy) Setup() error {
	domain := p.router.domain()
	if domain == "" {
		return fmt.Errorf("no proxy domain configured")
	}

	if err := p.certs.ensureCA(); err != nil {
		return fmt.Errorf("setting up CA: %w", err)
	}

	if _, _, err := p.certs.generate(domain); err != nil {
		return fmt.Errorf("generating certificate: %w", err)
	}

	return nil
}

func (p *Proxy) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p.port))
	if err != nil {
		return fmt.Errorf("binding port %d: %w", p.port, err)
	}

	domain := p.router.domain()
	cert, err := tls.LoadX509KeyPair(p.certs.certPath(domain), p.certs.keyPath(domain))
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
			return fmt.Errorf("dns server: %w", err)
		}
		if err := srv.Start(); err != nil {
			return fmt.Errorf("dns server on 127.0.0.1:%d (set proxy.dns_port to override): %w", p.dnsPort, err)
		}
		p.dns = srv
	}

	return nil
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
		if err := p.hosts.remove(); err != nil {
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
	return p.hosts.needsSudo()
}

func (p *Proxy) UnresolvedDomains() []string {
	return p.hosts.unresolved(p.router.enabledDomains())
}

func (p *Proxy) DNSBlock() string {
	return p.hosts.block(p.router.enabledDomains())
}

func (p *Proxy) SetupDNS() error {
	return p.hosts.add(p.router.enabledDomains())
}

func (p *Proxy) RemoveDNS() error {
	return p.hosts.remove()
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
