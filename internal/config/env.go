package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveEnv(cfg *Config, configDir string) error {
	if err := loadEnvFiles(cfg.EnvFile, cfg.Env, &cfg.Env, configDir); err != nil {
		return fmt.Errorf("global env_file: %w", err)
	}

	for name, svc := range cfg.Services {
		if err := loadEnvFiles(svc.EnvFile, svc.Env, &svc.Env, configDir); err != nil {
			return fmt.Errorf("service %q env_file: %w", name, err)
		}
		cfg.Services[name] = svc
	}

	interpolate(cfg.Env)
	for name, svc := range cfg.Services {
		interpolate(svc.Env)
		cfg.Services[name] = svc
	}

	return nil
}

// loadEnvFiles reads dotenv files and merges them into dst.
// Inline values (already in inline map) take priority over file values.
func loadEnvFiles(files []string, inline map[string]string, dst *map[string]string, configDir string) error {
	if len(files) == 0 {
		return nil
	}

	if *dst == nil {
		*dst = make(map[string]string)
	}

	for _, f := range files {
		path := f
		if !filepath.IsAbs(path) {
			path = filepath.Join(configDir, path)
		}
		fileEnv, err := parseEnvFile(path)
		if err != nil {
			return fmt.Errorf("%q: %w", f, err)
		}
		for k, v := range fileEnv {
			if _, exists := inline[k]; !exists {
				(*dst)[k] = v
			}
		}
	}
	return nil
}

func interpolate(env map[string]string) {
	for k, v := range env {
		env[k] = os.Expand(v, os.Getenv)
	}
}

func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	env := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		env[strings.TrimSpace(key)] = unquote(strings.TrimSpace(value))
	}
	return env, scanner.Err()
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[0] == s[len(s)-1] {
		return s[1 : len(s)-1]
	}
	return s
}
