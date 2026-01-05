package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
)

type handler struct {
	router *router
}

func newHandler(router *router) *handler {
	return &handler{router: router}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rt := h.router.match(r.Host)
	if rt == nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	if rt.rewrite != nil {
		r.URL.Path = rewritePath(r.URL.Path, rt.rewrite)
	}

	target := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", rt.port),
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("X-Lokl-Proxy", "true")
		return nil
	}

	// Preserve original host for backends that check it
	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("X-Forwarded-Proto", "https")

	proxy.ServeHTTP(w, r)
}

func rewritePath(p string, rw *rewriteConfig) string {
	// Strip prefix if configured
	if rw.stripPrefix != "" {
		prefix := "/" + rw.stripPrefix
		if after, found := strings.CutPrefix(p, prefix); found {
			p = after
			if p == "" {
				p = "/"
			}
		}
	}

	// Apply fallback for non-asset paths
	if rw.fallback != "" && !isAssetPath(p) {
		return rw.fallback
	}

	return p
}

func isAssetPath(p string) bool {
	// Check path prefixes used by dev servers
	assetPrefixes := []string{"/assets/", "/static/", "/@vite/", "/@fs/", "/__vite_ping"}
	for _, prefix := range assetPrefixes {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}

	// Check file extensions
	ext := strings.ToLower(path.Ext(p))
	assetExts := map[string]bool{
		".js": true, ".mjs": true, ".cjs": true,
		".css": true, ".scss": true, ".sass": true, ".less": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".ico": true, ".webp": true,
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
		".json": true, ".map": true,
		".html": true, ".htm": true,
		".mp4": true, ".webm": true, ".mp3": true, ".wav": true,
		".pdf": true,
	}

	return assetExts[ext]
}
