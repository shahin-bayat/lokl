package proxy

import "testing"

func TestRewritePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		rw   *RewriteConfig
		want string
	}{
		{
			name: "strip prefix",
			path: "/customer-funnel/dashboard",
			rw:   &RewriteConfig{StripPrefix: "customer-funnel"},
			want: "/dashboard",
		},
		{
			name: "strip prefix root",
			path: "/customer-funnel",
			rw:   &RewriteConfig{StripPrefix: "customer-funnel"},
			want: "/",
		},
		{
			name: "strip prefix with trailing slash",
			path: "/customer-funnel/",
			rw:   &RewriteConfig{StripPrefix: "customer-funnel"},
			want: "/",
		},
		{
			name: "no match prefix",
			path: "/other/path",
			rw:   &RewriteConfig{StripPrefix: "customer-funnel"},
			want: "/other/path",
		},
		{
			name: "fallback for non-asset",
			path: "/dashboard",
			rw:   &RewriteConfig{Fallback: "/index.html"},
			want: "/index.html",
		},
		{
			name: "no fallback for asset",
			path: "/assets/main.js",
			rw:   &RewriteConfig{Fallback: "/index.html"},
			want: "/assets/main.js",
		},
		{
			name: "strip prefix then fallback",
			path: "/customer-funnel/dashboard",
			rw:   &RewriteConfig{StripPrefix: "customer-funnel", Fallback: "/index.html"},
			want: "/index.html",
		},
		{
			name: "strip prefix keep asset",
			path: "/customer-funnel/assets/main.js",
			rw:   &RewriteConfig{StripPrefix: "customer-funnel", Fallback: "/index.html"},
			want: "/assets/main.js",
		},
		{
			name: "empty config",
			path: "/some/path",
			rw:   &RewriteConfig{},
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

func TestIsAssetPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// By extension
		{"/main.js", true},
		{"/style.css", true},
		{"/image.png", true},
		{"/font.woff2", true},
		{"/data.json", true},
		{"/page.html", true},
		{"/app.mjs", true},

		// By prefix
		{"/assets/anything", true},
		{"/static/file.txt", true},
		{"/@vite/client", true},
		{"/@fs/some/path", true},
		{"/__vite_ping", true},

		// Non-assets
		{"/dashboard", false},
		{"/users/123", false},
		{"/api/data", false},
		{"/", false},
		{"/settings", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isAssetPath(tt.path)
			if got != tt.want {
				t.Errorf("isAssetPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
