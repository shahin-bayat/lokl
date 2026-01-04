package config

import (
	"strings"
	"testing"
)

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
			err := Validate(&tt.cfg)
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

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{
		Services: map[string]Service{
			"a": {Command: "x"},
			"b": {Command: "y", Health: &HealthConfig{Path: "/health"}},
		},
	}

	ApplyDefaults(cfg)

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
