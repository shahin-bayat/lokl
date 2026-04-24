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
	parent  string
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
	wildcards  []*route
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

			if after, isWild := strings.CutPrefix(fqdn, "*."); isWild {
				rt.parent = after
				r.wildcards = append(r.wildcards, rt)
			} else {
				r.byHost[fqdn] = append(r.byHost[fqdn], rt)
			}
			r.byName[name] = rt
		}
	}

	for _, routes := range r.byHost {
		sort.Slice(routes, func(i, j int) bool {
			return prefixLen(routes[i]) > prefixLen(routes[j])
		})
	}

	// Longest parent first so nested wildcards (e.g. *.api.x.test) win over *.x.test during match.
	sort.SliceStable(r.wildcards, func(i, j int) bool {
		return len(r.wildcards[i].parent) > len(r.wildcards[j].parent)
	})

	return r
}

func (r *router) match(host, path string) *route {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	if routes := r.byHost[host]; len(routes) > 0 {
		return selectByPath(routes, path)
	}

	for _, rt := range r.wildcards {
		if !rt.enabled.Load() {
			continue
		}
		if wildcardMatches(host, rt.parent) {
			return selectByPath(wildcardsWithParent(r.wildcards, rt.parent), path)
		}
	}
	return nil
}

// wildcardMatches enforces a dot boundary so evil-sellify.shop does not match *.sellify.shop.
func wildcardMatches(host, parent string) bool {
	prefix, ok := strings.CutSuffix(host, "."+parent)
	if !ok || prefix == "" {
		return false
	}
	for _, label := range strings.Split(prefix, ".") {
		if label == "" {
			return false
		}
	}
	return true
}

func wildcardsWithParent(all []*route, parent string) []*route {
	out := make([]*route, 0, 1)
	for _, rt := range all {
		if rt.parent == parent {
			out = append(out, rt)
		}
	}
	return out
}

func selectByPath(routes []*route, path string) *route {
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
