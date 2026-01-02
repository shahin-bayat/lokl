package config

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
			svc.Restart = "on-failure"
		}

		if svc.Health != nil {
			applyHealthDefaults(svc.Health)
		}

		cfg.Services[name] = svc
	}
}

func applyHealthDefaults(h *HealthConfig) {
	if h.Interval == "" {
		h.Interval = "10s"
	}
	if h.Timeout == "" {
		h.Timeout = "3s"
	}
	if h.Retries == nil {
		r := 3
		h.Retries = &r
	}
}
