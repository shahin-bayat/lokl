package config

import (
	"fmt"
	"strconv"
	"strings"
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

	if err := checkDuplicatePorts(cfg.Services); err != nil {
		return err
	}

	for name, svc := range cfg.Services {
		if err := validateService(name, &svc, cfg.Services); err != nil {
			return err
		}
	}

	return nil
}

func checkDuplicatePorts(services map[string]Service) error {
	portToService := make(map[int]string)

	register := func(name string, port int) error {
		if port == 0 {
			return nil
		}
		if existing, exists := portToService[port]; exists && existing != name {
			return fmt.Errorf("services %q and %q both use port %d", existing, name, port)
		}
		portToService[port] = name
		return nil
	}

	for name, svc := range services {
		if err := register(name, svc.Port); err != nil {
			return err
		}
		for _, raw := range svc.Ports {
			host, _, err := parsePortMapping(raw)
			if err != nil {
				continue // validated later in validateDockerService
			}
			if err := register(name, host); err != nil {
				return err
			}
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

	if hasImage {
		if err := validateDockerService(name, svc); err != nil {
			return err
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

func validateDockerService(name string, svc *Service) error {
	for _, raw := range svc.Ports {
		host, container, err := parsePortMapping(raw)
		if err != nil {
			return fmt.Errorf("service %q: invalid port mapping %q: %w", name, raw, err)
		}
		if err := validatePortNumber(host); err != nil {
			return fmt.Errorf("service %q: port mapping %q: host %w", name, raw, err)
		}
		if err := validatePortNumber(container); err != nil {
			return fmt.Errorf("service %q: port mapping %q: container %w", name, raw, err)
		}
	}

	for _, raw := range svc.Volumes {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("service %q: invalid volume %q: expected host:container format", name, raw)
		}
		if !strings.HasPrefix(parts[1], "/") {
			return fmt.Errorf("service %q: invalid volume %q: container path must be absolute", name, raw)
		}
	}

	return nil
}

func parsePortMapping(s string) (host, container int, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected host:container format")
	}
	host, err = strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid host port: %w", err)
	}
	container, err = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid container port: %w", err)
	}
	return host, container, nil
}

func validatePortNumber(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port %d out of range (1-65535)", port)
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
