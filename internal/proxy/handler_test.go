package proxy

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/shahin-bayat/lokl/internal/config"
)

func TestRewritePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		rw   *rewriteConfig
		want string
	}{
		{
			name: "strip prefix",
			path: "/customer-funnel/dashboard",
			rw:   &rewriteConfig{stripPrefix: "customer-funnel"},
			want: "/dashboard",
		},
		{
			name: "strip prefix root",
			path: "/customer-funnel",
			rw:   &rewriteConfig{stripPrefix: "customer-funnel"},
			want: "/",
		},
		{
			name: "strip prefix with trailing slash",
			path: "/customer-funnel/",
			rw:   &rewriteConfig{stripPrefix: "customer-funnel"},
			want: "/",
		},
		{
			name: "no match prefix",
			path: "/other/path",
			rw:   &rewriteConfig{stripPrefix: "customer-funnel"},
			want: "/other/path",
		},
		{
			name: "fallback for non-asset",
			path: "/dashboard",
			rw:   &rewriteConfig{fallback: "/index.html"},
			want: "/index.html",
		},
		{
			name: "no fallback for asset",
			path: "/assets/main.js",
			rw:   &rewriteConfig{fallback: "/index.html"},
			want: "/assets/main.js",
		},
		{
			name: "strip prefix then fallback",
			path: "/customer-funnel/dashboard",
			rw:   &rewriteConfig{stripPrefix: "customer-funnel", fallback: "/index.html"},
			want: "/index.html",
		},
		{
			name: "strip prefix keep asset",
			path: "/customer-funnel/assets/main.js",
			rw:   &rewriteConfig{stripPrefix: "customer-funnel", fallback: "/index.html"},
			want: "/assets/main.js",
		},
		{
			name: "empty config",
			path: "/some/path",
			rw:   &rewriteConfig{},
			want: "/some/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewritePath(tt.path, tt.rw)
			if got != tt.want {
				t.Errorf("rewritePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func upstreamPort(t *testing.T, ts *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	_, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return port
}

func TestHandlerServeHTTPLocalRoute(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Test", "upstream")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	port := upstreamPort(t, upstream)
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "t.example.com"},
		Services: map[string]config.Service{
			"web": {Subdomain: "app", Port: port},
		},
	}

	r := newRouter(cfg)
	h := newHandler(r)

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "app.t.example.com"
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("X-Lokl-Proxy"); got != "local" {
		t.Errorf("X-Lokl-Proxy = %q, want local", got)
	}
}

func TestHandlerServeHTTPUnknownHost(t *testing.T) {
	cfg := &config.Config{
		Proxy:    config.ProxyConfig{Domain: "t.example.com"},
		Services: map[string]config.Service{},
	}

	r := newRouter(cfg)
	h := newHandler(r)

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "unknown.t.example.com"
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestHandlerXForwardedHeaders(t *testing.T) {
	var gotHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
	}))
	defer upstream.Close()

	port := upstreamPort(t, upstream)
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "t.example.com"},
		Services: map[string]config.Service{
			"web": {Subdomain: "app", Port: port},
		},
	}

	r := newRouter(cfg)
	h := newHandler(r)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "app.t.example.com"
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if got := gotHeaders.Get("X-Forwarded-Host"); got != "app.t.example.com" {
		t.Errorf("X-Forwarded-Host = %q, want app.t.example.com", got)
	}
	if got := gotHeaders.Get("X-Forwarded-Proto"); got != "https" {
		t.Errorf("X-Forwarded-Proto = %q, want https", got)
	}
}

func TestHandlerResponseHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"abc"`)
		w.Header().Set("Last-Modified", "Thu, 01 Jan 2026 00:00:00 GMT")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	port := upstreamPort(t, upstream)
	cfg := &config.Config{
		Proxy: config.ProxyConfig{Domain: "t.example.com"},
		Services: map[string]config.Service{
			"web": {Subdomain: "app", Port: port},
		},
	}

	r := newRouter(cfg)
	h := newHandler(r)

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "app.t.example.com"
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if got := rr.Header().Get("ETag"); got != "" {
		t.Errorf("ETag should be stripped, got %q", got)
	}
	if got := rr.Header().Get("Last-Modified"); got != "" {
		t.Errorf("Last-Modified should be stripped, got %q", got)
	}
}

func TestHasFileExtension(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/main.js", true},
		{"/style.css", true},
		{"/image.png", true},
		{"/font.woff2", true},
		{"/data.json", true},
		{"/page.html", true},
		{"/static/file.txt", true},

		{"/dashboard", false},
		{"/users/123", false},
		{"/api/data", false},
		{"/", false},
		{"/settings", false},
		{"/@vite/client", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := hasFileExtension(tt.path)
			if got != tt.want {
				t.Errorf("hasFileExtension(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
