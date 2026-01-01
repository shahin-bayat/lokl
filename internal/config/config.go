package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Name     string             `yaml:"name"`
	Version  string             `yaml:"version"`
	Domains  DomainsConfig      `yaml:"domains"`
	Env      map[string]string  `yaml:"env"`
	Services map[string]Service `yaml:"services"`
}

type DomainsConfig struct {
	Zone  string `yaml:"zone"`
	HTTPS *bool  `yaml:"https"`
}

type Service struct {
	Command string `yaml:"command"`
	Image   string `yaml:"image"`
	Path    string `yaml:"path"`

	Port   int    `yaml:"port"`
	Domain string `yaml:"domain"`

	SPA *SPAConfig `yaml:"spa"`

	Env map[string]string `yaml:"env"`

	DependsOn []string `yaml:"depends_on"`

	Health *HealthConfig `yaml:"health"`

	AutoStart    *bool  `yaml:"autostart"`
	Restart      string `yaml:"restart"`
	ReadyTimeout string `yaml:"ready_timeout"`

	Volumes []string `yaml:"volumes"`
	Ports   []string `yaml:"ports"`

	Limits *LimitsConfig `yaml:"limits"`
}

type SPAConfig struct {
	Root     string `yaml:"root"`
	Fallback string `yaml:"fallback"`
}

type HealthConfig struct {
	Path     string `yaml:"path"`
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout"`
	Retries  *int   `yaml:"retries"`
}

type LimitsConfig struct {
	Memory string `yaml:"memory"`
}

func Load(path string) (*Config, error) {
	cfg, err := parse(path)
	if err != nil {
		return nil, err
	}

	ApplyDefaults(cfg)

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

func parse(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}
