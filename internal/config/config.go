package config

import "fmt"

type Config struct {
	Name     string             `yaml:"name"`
	Version  string             `yaml:"version"`
	Domains  DomainsConfig      `yaml:"domains"`
	Env      map[string]string  `yaml:"env"`
	Services map[string]Service `yaml:"services"`
}

type DomainsConfig struct {
	Zone  string `yaml:"zone"`
	HTTPS bool   `yaml:"https"`
}

type Service struct {
	Command   string            `yaml:"command"`
	Image     string            `yaml:"image"`
	Path      string            `yaml:"path"`
	Port      int               `yaml:"port"`
	Domain    string            `yaml:"domain"`
	Env       map[string]string `yaml:"env"`
	DependsOn []string          `yaml:"depends_on"`
}

func Load(path string) (*Config, error) {
	return nil, fmt.Errorf("not implemented")
}
