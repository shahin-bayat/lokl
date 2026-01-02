package process

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func (s *Service) startHealthCheck(ctx context.Context) {
	if s.Config.Health == nil || s.Config.Health.Path == "" {
		s.Healthy = true
		return
	}

	interval, _ := time.ParseDuration(s.Config.Health.Interval)
	timeout, _ := time.ParseDuration(s.Config.Health.Timeout)
	retries := *s.Config.Health.Retries

	failures := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	time.Sleep(time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.checkHealth(timeout) {
				failures = 0
				s.mu.Lock()
				s.Healthy = true
				s.mu.Unlock()
			} else {
				failures++
				if failures >= retries {
					s.mu.Lock()
					s.Healthy = false
					s.mu.Unlock()
				}
			}
		}
	}
}

func (s *Service) checkHealth(timeout time.Duration) bool {
	url := fmt.Sprintf("http://localhost:%d%s", s.Config.Port, s.Config.Health.Path)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
