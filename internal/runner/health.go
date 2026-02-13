package runner

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// RunHealthCheck polls an HTTP endpoint and reports health state changes.
// Blocks until ctx is cancelled. onResult is called only on transitions.
func RunHealthCheck(ctx context.Context, port int, path string, interval, timeout time.Duration, retries int, onResult func(healthy bool)) {
	client := &http.Client{Timeout: timeout}
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
			if checkHealth(client, port, path) {
				failures = 0
				if !healthy {
					healthy = true
					onResult(true)
				}
			} else {
				failures++
				if failures >= retries && healthy {
					healthy = false
					onResult(false)
				}
			}
		}
	}
}

func checkHealth(client *http.Client, port int, path string) bool {
	url := fmt.Sprintf("http://localhost:%d%s", port, path)

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
