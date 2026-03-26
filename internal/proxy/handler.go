package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const (
	// DNS resolver for bypassing /etc/hosts (Cloudflare)
	// Alternatives: "8.8.8.8:53" (Google), "9.9.9.9:53" (Quad9)
	dnsResolver    = "1.1.1.1:53"
	dnsTimeout     = 5 * time.Second
	dialTimeout    = 10 * time.Second
	idleConnCount  = 100
	idleTimeout    = 90 * time.Second
	tlsTimeout     = 10 * time.Second
	continueTimout = 1 * time.Second
)

type handler struct {
	router   *router
	dnsCache map[string]string // domain -> IP cache
	dnsMu    sync.RWMutex
}

func newHandler(router *router) *handler {
	return &handler{
		router:   router,
		dnsCache: make(map[string]string),
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rt := h.router.match(r.Host, r.URL.Path)
	if rt == nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	target, transport, remote, err := h.selectTarget(rt, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			if remote {
				req.Host = target.Host
			}
		},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			resp.Header.Set("Pragma", "no-cache")
			resp.Header.Set("Expires", "0")
			resp.Header.Del("ETag")
			resp.Header.Del("Last-Modified")

			if remote {
				resp.Header.Set("X-Lokl-Proxy", "remote")
			} else {
				resp.Header.Set("X-Lokl-Proxy", "local")
			}
			return nil
		},
	}

	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("X-Forwarded-Proto", "https")

	proxy.ServeHTTP(w, r)
}

func (h *handler) invalidateCache(domain string) {
	h.dnsMu.Lock()
	delete(h.dnsCache, domain)
	h.dnsMu.Unlock()
}

func (h *handler) selectTarget(rt *route, r *http.Request) (target *url.URL, transport http.RoundTripper, remote bool, err error) {
	if rt.enabled.Load() {
		if rt.rewrite != nil {
			r.URL.Path = rewritePath(r.URL.Path, rt.rewrite)
		}
		return &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", rt.port)}, nil, false, nil
	}

	transport, err = h.remoteTransport(rt.domain)
	if err != nil {
		return nil, nil, false, err
	}
	return &url.URL{Scheme: "https", Host: rt.domain}, transport, true, nil
}

// resolveViaDNS queries external DNS directly, bypassing /etc/hosts
func (h *handler) resolveViaDNS(host string) (string, error) {
	h.dnsMu.RLock()
	ip, ok := h.dnsCache[host]
	h.dnsMu.RUnlock()
	if ok {
		return ip, nil
	}

	c := &dns.Client{Timeout: dnsTimeout}
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(host), dns.TypeA)

	r, _, err := c.Exchange(m, dnsResolver)
	if err != nil {
		return "", fmt.Errorf("DNS query failed: %w", err)
	}

	for _, ans := range r.Answer {
		if a, ok := ans.(*dns.A); ok {
			ip := a.A.String()
			h.dnsMu.Lock()
			h.dnsCache[host] = ip
			h.dnsMu.Unlock()
			return ip, nil
		}
	}

	return "", fmt.Errorf("no A record found for %s", host)
}

func (h *handler) remoteTransport(host string) (http.RoundTripper, error) {
	ip, err := h.resolveViaDNS(host)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", host, err)
	}

	dialer := &net.Dialer{Timeout: dialTimeout}

	return &http.Transport{
		DialContext: dialer.DialContext,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Connect to resolved IP but use original host for TLS
			_, port, _ := net.SplitHostPort(addr)
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
			if err != nil {
				return nil, err
			}
			tlsConn := tls.Client(conn, &tls.Config{ServerName: host})
			if err := tlsConn.Handshake(); err != nil {
				_ = conn.Close()
				return nil, err
			}
			return tlsConn, nil
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          idleConnCount,
		IdleConnTimeout:       idleTimeout,
		TLSHandshakeTimeout:   tlsTimeout,
		ExpectContinueTimeout: continueTimout,
	}, nil
}

func rewritePath(p string, rw *rewriteConfig) string {
	if rw.stripPrefix != "" {
		prefix := "/" + rw.stripPrefix
		if after, found := strings.CutPrefix(p, prefix); found {
			p = after
			if p == "" {
				p = "/"
			}
		}
	}

	if rw.fallback != "" && !hasFileExtension(p) {
		return rw.fallback
	}

	return p
}

func hasFileExtension(p string) bool {
	return path.Ext(p) != ""
}
