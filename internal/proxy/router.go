package proxy

import (
	"strings"

	"github.com/shahin-bayat/lokl/internal/config"
)

type route struct {
	domain  string
	port    int
	rewrite *rewriteConfig
	enabled bool
}

type rewriteConfig struct {
	stripPrefix string
	fallback    string
}

type router struct {
	baseDomain string
	routes     map[string]*route
}

func newRouter(cfg *config.Config) *router {
	r := &router{
		baseDomain: cfg.Proxy.Domain,
		routes:     make(map[string]*route),
	}

	for _, svc := range cfg.Services {
		if svc.Subdomain == "" || svc.Port == 0 {
			continue
		}

		fqdn := svc.Subdomain
		if !strings.Contains(svc.Subdomain, ".") && cfg.Proxy.Domain != "" {
			fqdn = svc.Subdomain + "." + cfg.Proxy.Domain
		}

		rt := &route{
			domain:  fqdn,
			port:    svc.Port,
			enabled: true,
		}

		if svc.Rewrite != nil {
			rt.rewrite = &rewriteConfig{
				stripPrefix: svc.Rewrite.StripPrefix,
				fallback:    svc.Rewrite.Fallback,
			}
		}

		r.routes[fqdn] = rt
	}

	return r
}

func (r *router) match(host string) *route {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	rt, ok := r.routes[host]
	if !ok {
		return nil
	}
	// Return route even when disabled - handler decides local vs remote
	return rt
}

func (r *router) domains() []string {
	domains := make([]string, 0, len(r.routes))
	for domain := range r.routes {
		domains = append(domains, domain)
	}
	return domains
}

func (r *router) enabledDomains() []string {
	var domains []string
	for domain, rt := range r.routes {
		if rt.enabled {
			domains = append(domains, domain)
		}
	}
	return domains
}

func (r *router) domain() string {
	return r.baseDomain
}

func (r *router) setEnabled(domain string, enabled bool) bool {
	rt, ok := r.routes[domain]
	if !ok {
		return false
	}
	rt.enabled = enabled
	return true
}
