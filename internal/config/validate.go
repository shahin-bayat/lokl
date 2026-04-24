package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func validate(cfg *Config) error {
	if cfg.Name == "" {
		return fmt.Errorf("name is required")
	}

	if len(cfg.Services) == 0 {
		return fmt.Errorf("at least one service is required")
	}

	for name, svc := range cfg.Services {
		if len(svc.Subdomains) > 0 && cfg.Proxy.Domain == "" {
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

	// Run after per-service validation so duplicate detection sees only well-formed subdomains.
	if err := checkDuplicateSubdomains(cfg); err != nil {
		return err
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
		if svc.ProxyOnly {
			continue // forwards to a port rather than claiming it
		}
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
	if svc.ProxyOnly {
		return validateProxyOnlyService(name, svc)
	}

	hasCommand := svc.Command.IsSet()
	hasImage := svc.Image != ""

	if !hasCommand && !hasImage {
		return fmt.Errorf("service %q: command or image is required", name)
	}

	if hasCommand && strings.TrimSpace(svc.Command.Args[0]) == "" {
		return fmt.Errorf("service %q: command must not be empty", name)
	}

	if hasImage && svc.Path != "" {
		return fmt.Errorf("service %q: path is only valid for command-based services; container services cannot set path", name)
	}

	if len(svc.Subdomains) > 0 && svc.Port == 0 {
		return fmt.Errorf("service %q: port is required when subdomain is set", name)
	}

	if err := validateSubdomainsOnService(name, svc.Subdomains); err != nil {
		return err
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

	if svc.Port > 0 && len(svc.Ports) > 0 {
		found := false
		for _, raw := range svc.Ports {
			host, _, err := parsePortMapping(raw)
			if err != nil {
				continue
			}
			if host == svc.Port {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("service %q: port %d is not mapped in ports", name, svc.Port)
		}
	}

	for _, raw := range svc.Volumes {
		if !strings.Contains(raw, ":") {
			if !strings.HasPrefix(raw, "/") {
				return fmt.Errorf("service %q: invalid volume %q: container path must be absolute", name, raw)
			}
			continue
		}
		parts := strings.SplitN(raw, ":", 2)
		if parts[0] == "" || parts[1] == "" {
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

func checkDuplicateSubdomains(cfg *Config) error {
	type key struct{ fqdn, prefix string }
	seen := make(map[key]string)
	for name, svc := range cfg.Services {
		for _, sd := range svc.Subdomains {
			if sd == "" {
				continue
			}
			fqdn := sd
			if !strings.Contains(fqdn, ".") && cfg.Proxy.Domain != "" {
				fqdn = sd + "." + cfg.Proxy.Domain
			}
			prefix := ""
			if svc.Rewrite != nil {
				prefix = strings.Trim(svc.Rewrite.StripPrefix, "/")
			}
			k := key{fqdn, prefix}
			if existing, ok := seen[k]; ok {
				if prefix == "" {
					return fmt.Errorf("services %q and %q: same subdomain with no prefix",
						existing, name)
				}
				return fmt.Errorf("services %q and %q: same subdomain with same prefix %q",
					existing, name, prefix)
			}
			seen[k] = name
		}
	}
	return nil
}

var reservedWildcardParents = map[string]struct{}{
	"com": {}, "org": {}, "net": {},
	"local": {}, "localhost": {}, "test": {},
}

func validateProxyOnlyService(name string, svc *Service) error {
	if svc.Command.IsSet() || svc.Image != "" {
		return fmt.Errorf("service %q: proxy_only cannot be combined with command or image", name)
	}
	if len(svc.Ports) > 0 {
		return fmt.Errorf("service %q: proxy_only cannot declare ports", name)
	}
	if len(svc.Volumes) > 0 {
		return fmt.Errorf("service %q: proxy_only cannot declare volumes", name)
	}
	if len(svc.Env) > 0 {
		return fmt.Errorf("service %q: proxy_only cannot declare env", name)
	}
	if len(svc.EnvFile) > 0 {
		return fmt.Errorf("service %q: proxy_only cannot declare env_file", name)
	}
	if svc.AutoStart != nil {
		return fmt.Errorf("service %q: autostart is not supported for proxy_only", name)
	}
	if svc.Restart != "" {
		return fmt.Errorf("service %q: restart is not supported for proxy_only", name)
	}
	if svc.ReadyTimeout != "" {
		return fmt.Errorf("service %q: ready_timeout is not supported for proxy_only", name)
	}
	if svc.Limits != nil {
		return fmt.Errorf("service %q: limits is not supported for proxy_only", name)
	}
	if svc.Port == 0 {
		return fmt.Errorf("service %q: proxy_only requires port", name)
	}
	if len(svc.Subdomains) == 0 {
		return fmt.Errorf("service %q: proxy_only requires subdomain", name)
	}
	if svc.Health != nil && svc.Health.Command.IsSet() {
		return fmt.Errorf("service %q: health.command is not supported for proxy_only", name)
	}

	return validateSubdomainsOnService(name, svc.Subdomains)
}

// validateSubdomainsOnService runs per-service subdomain checks (shape,
// reserved parents, duplicates) that apply equally to normal and proxy_only
// services.
func validateSubdomainsOnService(name string, subdomains []string) error {
	seen := map[string]struct{}{}
	for _, sd := range subdomains {
		if _, dup := seen[sd]; dup {
			return fmt.Errorf("service %q: duplicate subdomain %q", name, sd)
		}
		seen[sd] = struct{}{}
		if after, isWild := strings.CutPrefix(sd, "*."); isWild {
			if err := validateWildcardParent(after); err != nil {
				return fmt.Errorf("service %q: subdomain %q: %w", name, sd, err)
			}
			continue
		}
		if strings.Contains(sd, "*") {
			return fmt.Errorf("service %q: invalid subdomain %q", name, sd)
		}
	}
	return nil
}

func validateWildcardParent(parent string) error {
	if parent == "" {
		return fmt.Errorf("wildcard must have a parent domain")
	}
	if _, reserved := reservedWildcardParents[parent]; reserved {
		return fmt.Errorf("reserved wildcard parent %q", parent)
	}
	labels := strings.Split(parent, ".")
	if len(labels) < 2 {
		return fmt.Errorf("wildcard parent must have at least two labels")
	}
	for _, l := range labels {
		if l == "" || strings.Contains(l, "*") {
			return fmt.Errorf("invalid wildcard parent %q", parent)
		}
	}
	return nil
}

func validateHealth(svcName string, h *HealthConfig) error {
	if h.Path != "" && h.Command.IsSet() {
		return fmt.Errorf("service %q: health.path and health.command are mutually exclusive", svcName)
	}

	if h.Command.IsSet() && strings.TrimSpace(h.Command.Args[0]) == "" {
		return fmt.Errorf("service %q: health.command must not be empty", svcName)
	}

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
