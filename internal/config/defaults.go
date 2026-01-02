package config

const (
	DefaultRestartPolicy  = "on-failure"
	DefaultHealthInterval = "10s"
	DefaultHealthTimeout  = "3s"
	DefaultHealthRetries  = 3
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
			svc.Restart = DefaultRestartPolicy
		}

		if svc.Health != nil {
			applyHealthDefaults(svc.Health)
		}

		cfg.Services[name] = svc
	}
}

func applyHealthDefaults(h *HealthConfig) {
	if h.Interval == "" {
		h.Interval = DefaultHealthInterval
	}
	if h.Timeout == "" {
		h.Timeout = DefaultHealthTimeout
	}
	if h.Retries == nil {
		r := DefaultHealthRetries
		h.Retries = &r
	}
}
