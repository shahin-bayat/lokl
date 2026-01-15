package process

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func (p *Process) startHealthCheck(ctx context.Context) {
	if p.config.Health == nil || p.config.Health.Path == "" {
		p.mu.Lock()
		p.healthy = true
		p.mu.Unlock()
		return
	}

	interval, _ := time.ParseDuration(p.config.Health.Interval)
	timeout, _ := time.ParseDuration(p.config.Health.Timeout)
	retries := *p.config.Health.Retries

	failures := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	time.Sleep(time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if p.checkHealth(timeout) {
				failures = 0
				p.mu.Lock()
				prev := p.healthy
				p.healthy = true
				p.mu.Unlock()
				if !prev {
					p.onChange()
				}
			} else {
				failures++
				if failures >= retries {
					p.mu.Lock()
					prev := p.healthy
					p.healthy = false
					p.mu.Unlock()
					if prev {
						p.onChange()
					}
				}
			}
		}
	}
}

func (p *Process) checkHealth(timeout time.Duration) bool {
	url := fmt.Sprintf("http://localhost:%d%s", p.config.Port, p.config.Health.Path)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
