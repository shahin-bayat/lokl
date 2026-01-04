package proxy

import (
	"strings"

	"github.com/shahin-bayat/lokl/internal/config"
)

type Route struct {
	Domain  string
	Port    int
	Rewrite *RewriteConfig
	Enabled bool
}

type RewriteConfig struct {
	StripPrefix string
	Fallback    string
}

type Router struct {
	domain string
	routes map[string]*Route
}

func NewRouter(cfg *config.Config) *Router {
	r := &Router{
		domain: cfg.Proxy.Domain,
		routes: make(map[string]*Route),
	}

	for _, svc := range cfg.Services {
		if svc.Subdomain == "" || svc.Port == 0 {
			continue
		}

		fqdn := svc.Subdomain
		if !strings.Contains(svc.Subdomain, ".") && cfg.Proxy.Domain != "" {
			fqdn = svc.Subdomain + "." + cfg.Proxy.Domain
		}

		route := &Route{
			Domain:  fqdn,
			Port:    svc.Port,
			Enabled: true,
		}

		if svc.Rewrite != nil {
			route.Rewrite = &RewriteConfig{
				StripPrefix: svc.Rewrite.StripPrefix,
				Fallback:    svc.Rewrite.Fallback,
			}
		}

		r.routes[fqdn] = route
	}

	return r
}

func (r *Router) Match(host string) *Route {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	route, ok := r.routes[host]
	if !ok || !route.Enabled {
		return nil
	}
	return route
}

func (r *Router) Domains() []string {
	domains := make([]string, 0, len(r.routes))
	for domain := range r.routes {
		domains = append(domains, domain)
	}
	return domains
}

func (r *Router) EnabledDomains() []string {
	var domains []string
	for domain, route := range r.routes {
		if route.Enabled {
			domains = append(domains, domain)
		}
	}
	return domains
}

func (r *Router) Domain() string {
	return r.domain
}

func (r *Router) SetEnabled(domain string, enabled bool) bool {
	route, ok := r.routes[domain]
	if !ok {
		return false
	}
	route.Enabled = enabled
	return true
}
