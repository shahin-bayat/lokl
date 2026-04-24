package proxy

import (
	"sort"
	"strings"
	"sync/atomic"

	"github.com/shahin-bayat/lokl/internal/config"
)

type route struct {
	name    string
	domain  string
	port    int
	rewrite *rewriteConfig
	enabled atomic.Bool
}

type rewriteConfig struct {
	stripPrefix string
	fallback    string
}

type router struct {
	baseDomain string
	byHost     map[string][]*route
	byName     map[string]*route
}

func newRouter(cfg *config.Config) *router {
	r := &router{
		baseDomain: cfg.Proxy.Domain,
		byHost:     make(map[string][]*route),
		byName:     make(map[string]*route),
	}

	for name, svc := range cfg.Services {
		if svc.Port == 0 {
			continue
		}
		for _, sd := range svc.Subdomains {
			if sd == "" {
				continue
			}

			fqdn := sd
			if !strings.Contains(sd, ".") && cfg.Proxy.Domain != "" {
				fqdn = sd + "." + cfg.Proxy.Domain
			}

			rt := &route{
				name:   name,
				domain: fqdn,
				port:   svc.Port,
			}
			rt.enabled.Store(true)

			if svc.Rewrite != nil {
				rt.rewrite = &rewriteConfig{
					stripPrefix: strings.Trim(svc.Rewrite.StripPrefix, "/"),
					fallback:    svc.Rewrite.Fallback,
				}
			}

			r.byHost[fqdn] = append(r.byHost[fqdn], rt)
			r.byName[name] = rt
		}
	}

	for _, routes := range r.byHost {
		sort.Slice(routes, func(i, j int) bool {
			return prefixLen(routes[i]) > prefixLen(routes[j])
		})
	}

	return r
}

func (r *router) match(host, path string) *route {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	routes := r.byHost[host]
	if len(routes) == 0 {
		return nil
	}
	if len(routes) == 1 {
		return routes[0]
	}

	for _, rt := range routes {
		if rt.rewrite != nil && rt.rewrite.stripPrefix != "" {
			prefix := "/" + rt.rewrite.stripPrefix
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				return rt
			}
		}
	}

	for _, rt := range routes {
		if rt.rewrite == nil || rt.rewrite.stripPrefix == "" {
			return rt
		}
	}

	return nil
}

func (r *router) domains() []string {
	domains := make([]string, 0, len(r.byHost))
	for domain := range r.byHost {
		domains = append(domains, domain)
	}
	return domains
}

func (r *router) enabledDomains() []string {
	var domains []string
	for domain, routes := range r.byHost {
		for _, rt := range routes {
			if rt.enabled.Load() {
				domains = append(domains, domain)
				break
			}
		}
	}
	return domains
}

func (r *router) domain() string {
	return r.baseDomain
}

func (r *router) setEnabled(name string, enabled bool) bool {
	rt, ok := r.byName[name]
	if !ok {
		return false
	}
	rt.enabled.Store(enabled)
	return true
}

func prefixLen(rt *route) int {
	if rt.rewrite == nil {
		return 0
	}
	return len(rt.rewrite.stripPrefix)
}
