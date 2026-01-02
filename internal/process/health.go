package process

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func (p *Process) startHealthCheck(ctx context.Context) {
	if p.Config.Health == nil || p.Config.Health.Path == "" {
		p.Healthy = true
		return
	}

	interval, _ := time.ParseDuration(p.Config.Health.Interval)
	timeout, _ := time.ParseDuration(p.Config.Health.Timeout)
	retries := *p.Config.Health.Retries

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
				p.Healthy = true
				p.mu.Unlock()
			} else {
				failures++
				if failures >= retries {
					p.mu.Lock()
					p.Healthy = false
					p.mu.Unlock()
				}
			}
		}
	}
}

func (p *Process) checkHealth(timeout time.Duration) bool {
	url := fmt.Sprintf("http://localhost:%d%s", p.Config.Port, p.Config.Health.Path)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
