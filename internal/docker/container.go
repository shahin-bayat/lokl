package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shahin-bayat/lokl/internal/config"
)

const (
	maxLogLines    = 1000
	stopTimeout    = 10
	watchInterval  = 2 * time.Second
	containerLabel = "lokl-service"
)

type Container struct {
	name     string
	config   config.Service
	api      DockerAPI
	state    state
	healthy  bool
	onChange func()

	containerID string
	logs        *logs
	cancel      context.CancelFunc
	mu          sync.Mutex
}

func NewContainer(name string, cfg config.Service, api DockerAPI, onChange func()) *Container {
	return &Container{
		name:     name,
		config:   cfg,
		api:      api,
		state:    stateStopped,
		onChange: onChange,
	}
}

func (c *Container) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state == stateRunning
}

func (c *Container) IsHealthy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.healthy
}

func (c *Container) Logs() []string {
	if c.logs == nil {
		return nil
	}
	return c.logs.Lines()
}

func (c *Container) Start() error {
	c.mu.Lock()
	if c.state != stateStopped && c.state != stateFailed {
		c.mu.Unlock()
		return fmt.Errorf("container %s: cannot start from state %s", c.name, c.state)
	}
	c.state = stateStarting
	c.logs = newLogs(maxLogLines)
	c.mu.Unlock()
	c.onChange()

	ctx := context.Background()

	exists, err := c.api.ImageExists(ctx, c.config.Image)
	if err != nil {
		c.setFailed()
		return fmt.Errorf("container %s: checking image: %w", c.name, err)
	}
	if !exists {
		c.logf("pulling image %s...", c.config.Image)
		if err := c.api.PullImage(ctx, c.config.Image, func(msg string) {
			c.logf("%s", msg)
		}); err != nil {
			c.setFailed()
			return fmt.Errorf("container %s: pulling image: %w", c.name, err)
		}
	}

	ports, err := parsePorts(c.config.Ports)
	if err != nil {
		c.setFailed()
		return fmt.Errorf("container %s: %w", c.name, err)
	}

	containerName := fmt.Sprintf("lokl-%s", c.name)

	if staleID, _ := c.api.FindContainerByName(ctx, containerName); staleID != "" {
		_ = c.api.StopContainer(ctx, staleID, stopTimeout)
		_ = c.api.RemoveContainer(ctx, staleID)
	}

	cfg := ContainerConfig{
		Name:    containerName,
		Image:   c.config.Image,
		Env:     c.config.Env,
		Ports:   ports,
		Volumes: c.config.Volumes,
		Labels:  map[string]string{containerLabel: c.name},
	}

	id, err := c.api.CreateContainer(ctx, cfg)
	if err != nil {
		c.setFailed()
		return fmt.Errorf("container %s: creating: %w", c.name, err)
	}
	c.containerID = id

	if err := c.api.StartContainer(ctx, id); err != nil {
		_ = c.api.RemoveContainer(ctx, id)
		c.setFailed()
		return fmt.Errorf("container %s: starting: %w", c.name, err)
	}

	c.mu.Lock()
	c.state = stateRunning
	c.mu.Unlock()
	c.onChange()

	runCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go c.streamLogs(runCtx, id)
	go c.watchContainer(runCtx, id)
	go c.startHealthCheck(runCtx)

	return nil
}

func (c *Container) Stop() error {
	c.mu.Lock()
	if c.state != stateRunning && c.state != stateStarting {
		c.mu.Unlock()
		return nil
	}
	c.state = stateStopping
	c.healthy = false
	id := c.containerID
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()

	ctx := context.Background()
	_ = c.api.StopContainer(ctx, id, stopTimeout)
	_ = c.api.RemoveContainer(ctx, id)

	c.mu.Lock()
	c.state = stateStopped
	c.containerID = ""
	c.mu.Unlock()
	c.onChange()

	return nil
}

func (c *Container) streamLogs(ctx context.Context, containerID string) {
	rc, err := c.api.StreamLogs(ctx, containerID, true)
	if err != nil {
		c.logf("log stream error: %v", err)
		return
	}
	defer func() { _ = rc.Close() }()

	buf := make([]byte, 4096)
	for {
		n, readErr := rc.Read(buf)
		if n > 0 {
			_, _ = c.logs.Write(buf[:n])
		}
		if readErr != nil {
			if readErr != io.EOF {
				c.logf("log stream error: %v", readErr)
			}
			return
		}
	}
}

// Polls Docker to detect container crashes. Unlike processes (where cmd.Wait()
// gives instant notification), we don't own the container process so we poll.
func (c *Container) watchContainer(ctx context.Context, containerID string) {
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			running, err := c.api.IsContainerRunning(ctx, containerID)
			if err != nil || !running {
				c.mu.Lock()
				if c.state == stateRunning {
					c.state = stateFailed
					c.healthy = false
				}
				c.mu.Unlock()
				c.onChange()
				return
			}
		}
	}
}

func (c *Container) startHealthCheck(ctx context.Context) {
	if c.config.Health == nil || c.config.Health.Path == "" {
		c.mu.Lock()
		c.healthy = true
		c.mu.Unlock()
		c.onChange()
		return
	}

	interval, _ := time.ParseDuration(c.config.Health.Interval)
	timeout, _ := time.ParseDuration(c.config.Health.Timeout)
	retries := *c.config.Health.Retries

	client := &http.Client{Timeout: timeout}
	failures := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	time.Sleep(time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.checkHealth(client) {
				failures = 0
				c.mu.Lock()
				prev := c.healthy
				c.healthy = true
				c.mu.Unlock()
				if !prev {
					c.onChange()
				}
			} else {
				failures++
				if failures >= retries {
					c.mu.Lock()
					prev := c.healthy
					c.healthy = false
					c.mu.Unlock()
					if prev {
						c.onChange()
					}
				}
			}
		}
	}
}

func (c *Container) checkHealth(client *http.Client) bool {
	url := fmt.Sprintf("http://localhost:%d%s", c.config.Port, c.config.Health.Path)

	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

func (c *Container) setFailed() {
	c.mu.Lock()
	c.state = stateFailed
	c.mu.Unlock()
	c.onChange()
}

func (c *Container) logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_, _ = c.logs.Write([]byte(msg + "\n"))
}

func parsePorts(raw []string) ([]PortMapping, error) {
	ports := make([]PortMapping, 0, len(raw))
	for _, s := range raw {
		host, container, err := parsePortPair(s)
		if err != nil {
			return nil, fmt.Errorf("invalid port mapping %q: %w", s, err)
		}
		ports = append(ports, PortMapping{Host: host, Container: container})
	}
	return ports, nil
}

func parsePortPair(s string) (host, container int, err error) {
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
