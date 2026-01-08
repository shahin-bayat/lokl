package config

import "maps"

const (
	restartAlways    = "always"
	restartOnFailure = "on-failure"
	restartNever     = "never"

	defaultRestartPolicy  = restartOnFailure
	defaultHealthInterval = "10s"
	defaultHealthTimeout  = "3s"
	defaultHealthRetries  = 3
)

func ApplyDefaults(cfg *Config) {
	if cfg.Proxy.HTTPS == nil {
		t := true
		cfg.Proxy.HTTPS = &t
	}

	for name, svc := range cfg.Services {
		if svc.AutoStart == nil {
			t := true
			svc.AutoStart = &t
		}

		if svc.Restart == "" {
			svc.Restart = defaultRestartPolicy
		}

		if svc.Health != nil {
			applyHealthDefaults(svc.Health)
		}

		if len(cfg.Env) > 0 {
			merged := make(map[string]string, len(cfg.Env)+len(svc.Env))
			maps.Copy(merged, cfg.Env)
			maps.Copy(merged, svc.Env)
			svc.Env = merged
		}

		cfg.Services[name] = svc
	}
}

func applyHealthDefaults(h *HealthConfig) {
	if h.Interval == "" {
		h.Interval = defaultHealthInterval
	}
	if h.Timeout == "" {
		h.Timeout = defaultHealthTimeout
	}
	if h.Retries == nil {
		r := defaultHealthRetries
		h.Retries = &r
	}
}
