package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
)

type Handler struct {
	router *Router
}

func NewHandler(router *Router) *Handler {
	return &Handler{router: router}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route := h.router.Match(r.Host)
	if route == nil {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	if route.Rewrite != nil {
		r.URL.Path = rewritePath(r.URL.Path, route.Rewrite)
	}

	target := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", route.Port),
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
	}

	// Preserve original host for backends that check it
	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("X-Forwarded-Proto", "https")

	proxy.ServeHTTP(w, r)
}

func rewritePath(p string, rw *RewriteConfig) string {
	// Strip prefix if configured
	if rw.StripPrefix != "" {
		prefix := "/" + rw.StripPrefix
		if after, found := strings.CutPrefix(p, prefix); found {
			p = after
			if p == "" {
				p = "/"
			}
		}
	}

	// Apply fallback for non-asset paths
	if rw.Fallback != "" && !isAssetPath(p) {
		return rw.Fallback
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
