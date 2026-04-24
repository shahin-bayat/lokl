package config

import (
	"strings"
	"testing"
)

func TestValidateWildcardSubdomains(t *testing.T) {
	tests := []struct {
		name       string
		subdomains Subdomains
		wantErr    string
	}{
		{"bare star", Subdomains{"*"}, "invalid subdomain"},
		{"star dot only", Subdomains{"*."}, "wildcard must have a parent domain"},
		{"star mid label", Subdomains{"a.*.b"}, "invalid subdomain"},
		{"trailing star", Subdomains{"foo.*"}, "invalid subdomain"},
		{"prefix star", Subdomains{"*foo.bar"}, "invalid subdomain"},
		{"reserved com", Subdomains{"*.com"}, "reserved wildcard parent"},
		{"reserved local", Subdomains{"*.local"}, "reserved wildcard parent"},
		{"reserved test", Subdomains{"*.test"}, "reserved wildcard parent"},
		{"reserved localhost", Subdomains{"*.localhost"}, "reserved wildcard parent"},
		{"single label parent", Subdomains{"*.singlelabel"}, "at least two labels"},

		{"exact host", Subdomains{"api.x.test"}, ""},
		{"wildcard two label parent", Subdomains{"*.x.test"}, ""},
		{"wildcard deep parent", Subdomains{"*.api.x.test"}, ""},
		{"exact host reserved-looking", Subdomains{"x.test"}, ""},

		{"dup entries", Subdomains{"x.test", "x.test"}, "duplicate subdomain"},
		{"exact plus wildcard no dup", Subdomains{"x.test", "*.x.test"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Name:  "test",
				Proxy: ProxyConfig{Domain: "test.dev"},
				Services: map[string]Service{
					"a": {Command: cmdStr("x"), Port: 3000, Subdomains: tt.subdomains},
				},
			}
			err := validate(&cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErr)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}
