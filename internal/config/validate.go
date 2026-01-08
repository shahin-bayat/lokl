package config

import (
	"fmt"
	"time"
)

func Validate(cfg *Config) error {
	if cfg.Name == "" {
		return fmt.Errorf("name is required")
	}

	if len(cfg.Services) == 0 {
		return fmt.Errorf("at least one service is required")
	}

	for name, svc := range cfg.Services {
		if svc.Subdomain != "" && cfg.Proxy.Domain == "" {
			return fmt.Errorf("service %q has subdomain but proxy.domain is not configured", name)
		}
	}

	for name, svc := range cfg.Services {
		if err := validateService(name, &svc, cfg.Services); err != nil {
			return err
		}
	}

	return nil
}

func validateService(name string, svc *Service, services map[string]Service) error {
	hasCommand := svc.Command != ""
	hasImage := svc.Image != ""

	if !hasCommand && !hasImage {
		return fmt.Errorf("service %q: command or image is required", name)
	}
	if hasCommand && hasImage {
		return fmt.Errorf("service %q: cannot specify both command and image", name)
	}

	if svc.Subdomain != "" && svc.Port == 0 {
		return fmt.Errorf("service %q: port is required when subdomain is set", name)
	}

	if svc.Health != nil && svc.Health.Path != "" && svc.Port == 0 {
		return fmt.Errorf("service %q: port is required when health check is configured", name)
	}

	for _, dep := range svc.DependsOn {
		if _, exists := services[dep]; !exists {
			return fmt.Errorf("service %q: depends_on references unknown service %q", name, dep)
		}
	}

	if svc.Health != nil {
		if err := validateHealth(name, svc.Health); err != nil {
			return err
		}
	}

	if svc.ReadyTimeout != "" {
		if _, err := time.ParseDuration(svc.ReadyTimeout); err != nil {
			return fmt.Errorf("service %q: invalid ready_timeout %q: %w", name, svc.ReadyTimeout, err)
		}
	}

	if svc.Restart != "" {
		switch svc.Restart {
		case restartAlways, restartOnFailure, restartNever:
		default:
			return fmt.Errorf("service %q: invalid restart policy %q (must be %s, %s, or %s)", name, svc.Restart, restartAlways, restartOnFailure, restartNever)
		}
	}

	return nil
}

func validateHealth(svcName string, h *HealthConfig) error {
	if h.Interval != "" {
		if _, err := time.ParseDuration(h.Interval); err != nil {
			return fmt.Errorf("service %q: invalid health.interval %q: %w", svcName, h.Interval, err)
		}
	}

	if h.Timeout != "" {
		if _, err := time.ParseDuration(h.Timeout); err != nil {
			return fmt.Errorf("service %q: invalid health.timeout %q: %w", svcName, h.Timeout, err)
		}
	}

	return nil
}
