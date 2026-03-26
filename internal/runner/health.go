package runner

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// RunProbe polls probe() on interval. Calls onChange on every health state transition.
// Blocks until ctx is canceled.
func RunProbe(ctx context.Context, probe func() bool, interval time.Duration, retries int, onChange func(bool)) {
	healthy := false
	failures := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	time.Sleep(time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if probe() {
				failures = 0
				if !healthy {
					healthy = true
					onChange(true)
				}
			} else {
				failures++
				if failures >= retries && healthy {
					healthy = false
					onChange(false)
				}
			}
		}
	}
}

// RunHealthCheck polls an HTTP endpoint. Thin wrapper over RunProbe.
func RunHealthCheck(ctx context.Context, port int, path string, interval, timeout time.Duration, retries int, onResult func(healthy bool)) {
	client := &http.Client{Timeout: timeout}
	RunProbe(ctx, func() bool {
		return checkHealth(client, port, path)
	}, interval, retries, onResult)
}

func checkHealth(client *http.Client, port int, path string) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
