package config

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func LoadBytes(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	applyDefaults(&cfg)
	return &cfg, validate(&cfg)
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid config", "testdata/valid.yaml", false},
		{"minimal config", "testdata/minimal.yaml", false},
		{"file not found", "testdata/nonexistent.yaml", true},
		{"invalid yaml", "testdata/invalid_yaml.yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg == nil {
				t.Fatal("expected config, got nil")
			}
		})
	}
}

func TestLoadValidConfig(t *testing.T) {
	cfg, err := Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Name != "test-project" {
		t.Errorf("name = %q, want %q", cfg.Name, "test-project")
	}
	if cfg.Proxy.Domain != "test.dev" {
		t.Errorf("proxy.domain = %q, want %q", cfg.Proxy.Domain, "test.dev")
	}
	if len(cfg.Services) != 2 {
		t.Errorf("services count = %d, want 2", len(cfg.Services))
	}

	api := cfg.Services["api"]
	if api.Port != 3000 {
		t.Errorf("api.port = %d, want 3000", api.Port)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "missing name",
			cfg:     Config{Services: map[string]Service{"a": {Command: "x"}}},
			wantErr: "name is required",
		},
		{
			name:    "no services",
			cfg:     Config{Name: "test"},
			wantErr: "at least one service is required",
		},
		{
			name: "no command or image",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {}},
			},
			wantErr: "command or image is required",
		},
		{
			name: "both command and image",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Command: "x", Image: "y"}},
			},
			wantErr: "cannot specify both command and image",
		},
		{
			name: "subdomain without proxy domain",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Command: "x", Subdomain: "app", Port: 3000}},
			},
			wantErr: "has subdomain but proxy.domain is not configured",
		},
		{
			name: "subdomain without port",
			cfg: Config{
				Name:     "test",
				Proxy:    ProxyConfig{Domain: "test.dev"},
				Services: map[string]Service{"a": {Command: "x", Subdomain: "app"}},
			},
			wantErr: "port is required when subdomain is set",
		},
		{
			name: "unknown dependency",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Command: "x", DependsOn: []string{"unknown"}}},
			},
			wantErr: "depends_on references unknown service",
		},
		{
			name: "invalid ready_timeout",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Command: "x", ReadyTimeout: "bad"}},
			},
			wantErr: "invalid ready_timeout",
		},
		{
			name: "invalid restart policy",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Command: "x", Restart: "bad"}},
			},
			wantErr: "invalid restart policy",
		},
		{
			name: "invalid health interval",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Command: "x", Health: &HealthConfig{Interval: "bad"}}},
			},
			wantErr: "invalid health.interval",
		},
		{
			name: "duplicate ports",
			cfg: Config{
				Name: "test",
				Services: map[string]Service{
					"a": {Command: "x", Port: 3000},
					"b": {Command: "y", Port: 3000},
				},
			},
			wantErr: "both use port 3000",
		},
		{
			name: "docker: invalid port format",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Image: "nginx", Ports: []string{"bad"}}},
			},
			wantErr: "invalid port mapping",
		},
		{
			name: "docker: port out of range",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Image: "nginx", Ports: []string{"99999:80"}}},
			},
			wantErr: "out of range",
		},
		{
			name: "docker: invalid volume format",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Image: "nginx", Volumes: []string{"bad"}}},
			},
			wantErr: "invalid volume",
		},
		{
			name: "docker: volume relative container path",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Image: "nginx", Volumes: []string{"./data:relative"}}},
			},
			wantErr: "container path must be absolute",
		},
		{
			name: "docker: port not in ports mapping",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Image: "nginx", Port: 8080, Ports: []string{"3000:80"}}},
			},
			wantErr: "port 8080 is not mapped",
		},
		{
			name: "docker: cross-service port conflict with docker host port",
			cfg: Config{
				Name: "test",
				Services: map[string]Service{
					"a": {Command: "x", Port: 8080},
					"b": {Image: "nginx", Ports: []string{"8080:80"}},
				},
			},
			wantErr: "both use port 8080",
		},
		{
			name: "docker: valid service",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Image: "nginx", Port: 8080, Ports: []string{"8080:80"}}},
			},
			wantErr: "",
		},
		{
			name: "shared subdomain same prefix",
			cfg: Config{
				Name:  "test",
				Proxy: ProxyConfig{Domain: "test.dev"},
				Services: map[string]Service{
					"a": {Command: "x", Subdomain: "client", Port: 8080},
					"b": {Command: "y", Subdomain: "client", Port: 8081},
				},
			},
			wantErr: "same subdomain with no prefix",
		},
		{
			name: "shared subdomain different prefix",
			cfg: Config{
				Name:  "test",
				Proxy: ProxyConfig{Domain: "test.dev"},
				Services: map[string]Service{
					"a": {Command: "x", Subdomain: "client", Port: 8080, Rewrite: &RewriteConfig{StripPrefix: "app-a"}},
					"b": {Command: "y", Subdomain: "client", Port: 8081, Rewrite: &RewriteConfig{StripPrefix: "app-b"}},
				},
			},
			wantErr: "",
		},
		{
			name: "valid config",
			cfg: Config{
				Name:     "test",
				Services: map[string]Service{"a": {Command: "x"}},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadWithEnvFile(t *testing.T) {
	t.Setenv("DB_USER", "from-shell")
	t.Setenv("DB_PASS", "shellpass")

	cfg, err := Load("testdata/env_file.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inline env "DB_USER: admin" should override env_file value "postgres"
	if cfg.Env["DB_USER"] != "admin" {
		t.Errorf("global DB_USER = %q, want %q", cfg.Env["DB_USER"], "admin")
	}

	// env_file value should be loaded for keys not in inline
	if cfg.Env["DB_PASS"] != "s3cret" {
		t.Errorf("global DB_PASS = %q, want %q", cfg.Env["DB_PASS"], "s3cret")
	}

	// After applyDefaults merges global into service, interpolation should have
	// resolved ${DB_USER} and ${DB_PASS} from os.Environ
	api := cfg.Services["api"]
	want := "postgres://from-shell:shellpass@localhost/mydb"
	if api.Env["DATABASE_URL"] != want {
		t.Errorf("DATABASE_URL = %q, want %q", api.Env["DATABASE_URL"], want)
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{
		Services: map[string]Service{
			"a": {Command: "x"},
			"b": {Command: "y", Health: &HealthConfig{Path: "/health"}},
		},
	}

	applyDefaults(cfg)

	if cfg.Proxy.HTTPS == nil || !*cfg.Proxy.HTTPS {
		t.Error("proxy.https should default to true")
	}

	svcA := cfg.Services["a"]
	if svcA.AutoStart == nil || !*svcA.AutoStart {
		t.Error("autostart should default to true")
	}
	if svcA.Restart != "on-failure" {
		t.Errorf("restart = %q, want %q", svcA.Restart, "on-failure")
	}

	svcB := cfg.Services["b"]
	if svcB.Health.Interval != "10s" {
		t.Errorf("health.interval = %q, want %q", svcB.Health.Interval, "10s")
	}
	if svcB.Health.Timeout != "3s" {
		t.Errorf("health.timeout = %q, want %q", svcB.Health.Timeout, "3s")
	}
	if svcB.Health.Retries == nil || *svcB.Health.Retries != 3 {
		t.Error("health.retries should default to 3")
	}
}

func TestStringOrSliceUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantArgs  []string
		wantShell bool
	}{
		{
			name:      "string form",
			yaml:      `command: "pg_isready -U postgres"`,
			wantArgs:  []string{"pg_isready -U postgres"},
			wantShell: true,
		},
		{
			name:      "array form",
			yaml:      `command: ["redis-cli", "ping"]`,
			wantArgs:  []string{"redis-cli", "ping"},
			wantShell: false,
		},
		{
			name:      "single-element array",
			yaml:      `command: ["pg_isready"]`,
			wantArgs:  []string{"pg_isready"},
			wantShell: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type wrapper struct {
				Command StringOrSlice `yaml:"command"`
			}
			var w wrapper
			if err := yaml.Unmarshal([]byte(tt.yaml), &w); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !slices.Equal(w.Command.Args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", w.Command.Args, tt.wantArgs)
			}
			if w.Command.Shell != tt.wantShell {
				t.Errorf("shell = %v, want %v", w.Command.Shell, tt.wantShell)
			}
		})
	}
}

func TestStringOrSliceIsSet(t *testing.T) {
	empty := StringOrSlice{}
	if empty.IsSet() {
		t.Error("empty StringOrSlice should not be set")
	}
	set := StringOrSlice{Args: []string{"pg_isready"}}
	if !set.IsSet() {
		t.Error("non-empty StringOrSlice should be set")
	}
}

func TestValidateHealthEmptyCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name: "empty string command",
			input: `
name: test
services:
  db:
    image: postgres:16
    health:
      command: ""
`,
			wantErr: true,
		},
		{
			name: "empty array element",
			input: `
name: test
services:
  db:
    image: postgres:16
    health:
      command: [""]
`,
			wantErr: true,
		},
		{
			name: "valid command",
			input: `
name: test
services:
  db:
    image: postgres:16
    health:
      command: "pg_isready"
`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadBytes([]byte(tt.input))
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateHealthMutualExclusion(t *testing.T) {
	input := `
name: test
services:
  db:
    image: postgres:16
    port: 5432
    health:
      path: /health
      command: "pg_isready"
`
	_, err := LoadBytes([]byte(input))
	if err == nil {
		t.Fatal("expected error when both path and command are set")
	}
	if !strings.Contains(err.Error(), "path") || !strings.Contains(err.Error(), "command") {
		t.Errorf("error should mention both path and command, got: %v", err)
	}
}
