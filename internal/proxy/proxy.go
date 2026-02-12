// Package proxy provides HTTPS reverse proxy setup with cert and DNS management.
package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/shahin-bayat/lokl/internal/config"
)

const (
	defaultPort     = 443
	defaultCertDir  = ".lokl/certs"
	shutdownTimeout = 5 * time.Second
)

type Proxy struct {
	cfg     *config.Config
	router  *router
	certs   *certManager
	hosts   *hostsManager
	handler *handler
	server  *http.Server
	port    int
}

func New(cfg *config.Config) *Proxy {
	r := newRouter(cfg)
	return &Proxy{
		cfg:     cfg,
		router:  r,
		certs:   newCertManager(defaultCertDir),
		hosts:   newHostsManager(cfg.Name),
		handler: newHandler(r),
		port:    defaultPort,
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
	domain := p.router.domain()
	certPath := p.certs.certPath(domain)
	keyPath := p.certs.keyPath(domain)

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("loading certificate: %w", err)
	}

	p.server = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", p.port),
		Handler: p.handler,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}

	return p.server.ListenAndServeTLS("", "")
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
