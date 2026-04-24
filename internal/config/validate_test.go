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

func TestValidateProxyOnly(t *testing.T) {
	base := func() *Config {
		return &Config{
			Name:  "demo",
			Proxy: ProxyConfig{Domain: "test"},
			Services: map[string]Service{
				"real": {Command: StringOrSlice{Args: []string{"sh"}, Shell: true}, Port: 8000, Subdomains: Subdomains{"api"}},
			},
		}
	}

	t.Run("accept minimal", func(t *testing.T) {
		cfg := base()
		cfg.Services["console"] = Service{ProxyOnly: true, Port: 9001, Subdomains: Subdomains{"console"}}
		if err := validate(cfg); err != nil {
			t.Fatalf("want accept, got %v", err)
		}
	})

	t.Run("reject command", func(t *testing.T) {
		cfg := base()
		cfg.Services["bad"] = Service{ProxyOnly: true, Port: 9001, Subdomains: Subdomains{"x"}, Command: StringOrSlice{Args: []string{"sh"}, Shell: true}}
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on command+proxy_only")
		}
	})

	t.Run("reject image", func(t *testing.T) {
		cfg := base()
		cfg.Services["bad"] = Service{ProxyOnly: true, Port: 9001, Subdomains: Subdomains{"x"}, Image: "foo"}
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on image+proxy_only")
		}
	})

	t.Run("reject missing port", func(t *testing.T) {
		cfg := base()
		cfg.Services["bad"] = Service{ProxyOnly: true, Subdomains: Subdomains{"x"}}
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on missing port")
		}
	})

	t.Run("reject missing subdomain", func(t *testing.T) {
		cfg := base()
		cfg.Services["bad"] = Service{ProxyOnly: true, Port: 9001}
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on missing subdomain")
		}
	})

	t.Run("reject health command", func(t *testing.T) {
		cfg := base()
		cfg.Services["bad"] = Service{
			ProxyOnly:  true,
			Port:       9001,
			Subdomains: Subdomains{"x"},
			Health:     &HealthConfig{Command: StringOrSlice{Args: []string{"true"}, Shell: true}},
		}
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on health.command+proxy_only")
		}
	})

	t.Run("reject volumes", func(t *testing.T) {
		cfg := base()
		cfg.Services["bad"] = Service{ProxyOnly: true, Port: 9001, Subdomains: Subdomains{"x"}, Volumes: []string{"./a:/b"}}
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on volumes+proxy_only")
		}
	})

	t.Run("accept wildcard subdomain", func(t *testing.T) {
		cfg := base()
		cfg.Services["tenant"] = Service{ProxyOnly: true, Port: 9001, Subdomains: Subdomains{"*.x.test"}}
		if err := validate(cfg); err != nil {
			t.Fatalf("wildcard subdomain should be accepted: %v", err)
		}
	})
}

func TestProxyOnlyDoesNotTripDuplicatePorts(t *testing.T) {
	cfg := &Config{
		Name:  "demo",
		Proxy: ProxyConfig{Domain: "test"},
		Services: map[string]Service{
			"minio": {
				Image:      "minio/minio",
				Port:       9000,
				Ports:      []string{"9000:9000", "9001:9001"},
				Subdomains: Subdomains{"s3"},
			},
			"console": {
				ProxyOnly:  true,
				Port:       9001,
				Subdomains: Subdomains{"console"},
			},
		},
	}
	if err := validate(cfg); err != nil {
		t.Fatalf("proxy-only should not collide with published port: %v", err)
	}
}

func TestValidateProxyOnlyRejectsEveryForbiddenField(t *testing.T) {
	base := func() Service {
		return Service{ProxyOnly: true, Port: 9001, Subdomains: Subdomains{"console"}}
	}
	t.Run("reject ports", func(t *testing.T) {
		cfg := &Config{Name: "demo", Proxy: ProxyConfig{Domain: "test"}, Services: map[string]Service{"bad": {}}}
		svc := base()
		svc.Ports = []string{"9001:9001"}
		cfg.Services["bad"] = svc
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on ports")
		}
	})
	t.Run("reject env", func(t *testing.T) {
		cfg := &Config{Name: "demo", Proxy: ProxyConfig{Domain: "test"}, Services: map[string]Service{"bad": {}}}
		svc := base()
		svc.Env = map[string]string{"FOO": "bar"}
		cfg.Services["bad"] = svc
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on env")
		}
	})
	t.Run("reject env_file", func(t *testing.T) {
		cfg := &Config{Name: "demo", Proxy: ProxyConfig{Domain: "test"}, Services: map[string]Service{"bad": {}}}
		svc := base()
		svc.EnvFile = []string{".env"}
		cfg.Services["bad"] = svc
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on env_file")
		}
	})
	t.Run("reject autostart", func(t *testing.T) {
		cfg := &Config{Name: "demo", Proxy: ProxyConfig{Domain: "test"}, Services: map[string]Service{"bad": {}}}
		svc := base()
		trueVal := true
		svc.AutoStart = &trueVal
		cfg.Services["bad"] = svc
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on autostart")
		}
	})
	t.Run("reject restart", func(t *testing.T) {
		cfg := &Config{Name: "demo", Proxy: ProxyConfig{Domain: "test"}, Services: map[string]Service{"bad": {}}}
		svc := base()
		svc.Restart = "always"
		cfg.Services["bad"] = svc
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on restart")
		}
	})
	t.Run("reject ready_timeout", func(t *testing.T) {
		cfg := &Config{Name: "demo", Proxy: ProxyConfig{Domain: "test"}, Services: map[string]Service{"bad": {}}}
		svc := base()
		svc.ReadyTimeout = "5s"
		cfg.Services["bad"] = svc
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on ready_timeout")
		}
	})
	t.Run("reject limits", func(t *testing.T) {
		cfg := &Config{Name: "demo", Proxy: ProxyConfig{Domain: "test"}, Services: map[string]Service{"bad": {}}}
		svc := base()
		svc.Limits = &LimitsConfig{}
		cfg.Services["bad"] = svc
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on limits")
		}
	})
	t.Run("reject bare asterisk subdomain", func(t *testing.T) {
		cfg := &Config{Name: "demo", Proxy: ProxyConfig{Domain: "test"}, Services: map[string]Service{"bad": {}}}
		svc := base()
		svc.Subdomains = Subdomains{"*"}
		cfg.Services["bad"] = svc
		if err := validate(cfg); err == nil {
			t.Fatal("expected rejection on bare asterisk subdomain")
		}
	})
}
