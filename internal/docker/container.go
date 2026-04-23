package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shahin-bayat/lokl/internal/config"
	"github.com/shahin-bayat/lokl/internal/runner"
)

const (
	maxLogLines    = 1000
	stopTimeout    = 10
	watchInterval  = 2 * time.Second
	containerLabel = "lokl-service"
	defaultRetries = 3
)

type Container struct {
	name     string
	config   config.Service
	api      DockerAPI
	network  string
	state    runner.State
	healthy  bool
	onChange func()
	onCrash  func()

	manuallyStopped bool
	containerID     string
	logs            *runner.Logs
	cancel          context.CancelFunc
	mu              sync.Mutex
}

func NewContainer(name string, cfg config.Service, api DockerAPI, network string, onChange func(), onCrash func()) *Container {
	return &Container{
		name:     name,
		config:   cfg,
		api:      api,
		network:  network,
		state:    runner.StateStopped,
		onChange: onChange,
		onCrash:  onCrash,
	}
}

func (c *Container) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state == runner.StateRunning
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
	if c.state != runner.StateStopped && c.state != runner.StateFailed {
		c.mu.Unlock()
		return fmt.Errorf("container %s: cannot start from state %s", c.name, c.state)
	}
	c.manuallyStopped = false
	c.state = runner.StateStarting
	c.logs = runner.NewLogs(maxLogLines)
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

	volumes, anonVolumes, err := splitVolumes(c.config.Volumes)
	if err != nil {
		c.setFailed()
		return fmt.Errorf("container %s: %w", c.name, err)
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
		Name:             containerName,
		Image:            c.config.Image,
		Env:              c.config.Env,
		Ports:            ports,
		Volumes:          volumes,
		AnonymousVolumes: anonVolumes,
		Labels:           map[string]string{containerLabel: c.name},
		Network:          c.network,
		NetworkAliases:   []string{c.name},
	}
	if c.config.Command.IsSet() {
		cfg.Cmd = buildExecCmd(c.config.Command)
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
	c.state = runner.StateRunning
	c.mu.Unlock()
	c.onChange()

	runCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go c.streamLogs(runCtx, id)
	go c.watchContainer(runCtx, id)

	switch {
	case c.config.Health != nil && c.config.Health.Command.IsSet():
		cmd := buildExecCmd(expandHealthCmd(c.config.Health.Command, c.config.Env))
		interval, timeout, retries := parseHealthParams(c.config.Health)
		go runner.RunProbe(runCtx, func() bool {
			execCtx, cancel := context.WithTimeout(runCtx, timeout)
			defer cancel()
			code, err := c.api.ExecContainer(execCtx, id, cmd)
			if err != nil || code != 0 {
				c.logf("health exec: code=%d err=%v", code, err)
			}
			return err == nil && code == 0
		}, interval, retries, func(healthy bool) {
			c.mu.Lock()
			wasHealthy := c.healthy
			c.healthy = healthy
			c.mu.Unlock()
			if !healthy && wasHealthy {
				c.onCrash()
			}
			c.onChange()
		})

	case c.config.Health != nil && c.config.Health.Path != "":
		interval, timeout, retries := parseHealthParams(c.config.Health)
		go runner.RunHealthCheck(runCtx, c.config.Port, c.config.Health.Path, interval, timeout, retries, func(healthy bool) {
			c.mu.Lock()
			wasHealthy := c.healthy
			c.healthy = healthy
			c.mu.Unlock()
			if !healthy && wasHealthy {
				c.onCrash()
			}
			c.onChange()
		})

	default:
		c.mu.Lock()
		c.healthy = true
		c.mu.Unlock()
		c.onChange()
	}

	return nil
}

func (c *Container) Stop() error {
	c.mu.Lock()
	if c.state != runner.StateRunning && c.state != runner.StateStarting {
		c.mu.Unlock()
		return nil
	}
	c.manuallyStopped = true
	c.state = runner.StateStopping
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
	c.state = runner.StateStopped
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
				shouldNotify := !c.manuallyStopped && c.state == runner.StateRunning
				if c.state == runner.StateRunning {
					c.state = runner.StateFailed
					c.healthy = false
				}
				c.mu.Unlock()
				if shouldNotify {
					c.onCrash()
				}
				c.onChange()
				return
			}
		}
	}
}

func (c *Container) setFailed() {
	c.mu.Lock()
	c.state = runner.StateFailed
	c.mu.Unlock()
	c.onChange()
}

func (c *Container) logf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_, _ = c.logs.Write([]byte(msg + "\n"))
}

func parseHealthParams(h *config.HealthConfig) (interval, timeout time.Duration, retries int) {
	interval, _ = time.ParseDuration(h.Interval)
	timeout, _ = time.ParseDuration(h.Timeout)
	retries = defaultRetries
	if h.Retries != nil {
		retries = *h.Retries
	}
	return
}

func buildExecCmd(s config.StringOrSlice) []string {
	if s.Shell {
		// `exec` replaces the shell so the real process becomes PID 1 and
		// receives signals directly (otherwise `sh` swallows SIGTERM).
		return []string{"sh", "-c", "exec " + strings.Join(s.Args, " ")}
	}
	return s.Args
}

// expandHealthCmd expands ${VAR} references in health command args using the
// service env map, so exec inherits the container's PATH instead of getting a
// replacement environment.
func expandHealthCmd(s config.StringOrSlice, env map[string]string) config.StringOrSlice {
	lookup := func(key string) string {
		if v, ok := env[key]; ok {
			return v
		}
		return os.Getenv(key)
	}
	expanded := make([]string, len(s.Args))
	for i, arg := range s.Args {
		expanded[i] = os.Expand(arg, lookup)
	}
	return config.StringOrSlice{Args: expanded, Shell: s.Shell}
}

// splitVolumes separates bind/named mounts from anonymous volumes (bare container paths).
// Bind mount host paths are resolved to absolute and the directory is pre-created (Docker
// requires absolute host paths; Docker Desktop won't auto-create missing dirs).
// Named volumes (e.g. "pgdata:/var/lib/...") are passed through unchanged.
// Anonymous volumes ("/container/path") let Docker create an unnamed volume that masks
// a directory from an outer bind mount (e.g. vendor/ on top of a source bind).
func splitVolumes(raw []string) (binds []string, anon []string, err error) {
	binds = make([]string, 0, len(raw))
	for _, v := range raw {
		host, rest, ok := strings.Cut(v, ":")
		if !ok {
			anon = append(anon, v)
			continue
		}
		if strings.HasPrefix(host, "/") || strings.HasPrefix(host, ".") {
			if !filepath.IsAbs(host) {
				if abs, absErr := filepath.Abs(host); absErr == nil {
					host = abs
				}
			}
			if info, statErr := os.Lstat(host); os.IsNotExist(statErr) || (statErr == nil && info.IsDir()) {
				if mkErr := os.MkdirAll(host, 0o755); mkErr != nil {
					return nil, nil, fmt.Errorf("creating volume directory %q: %w", host, mkErr)
				}
			}
			binds = append(binds, host+":"+rest)
			continue
		}
		binds = append(binds, v)
	}
	return binds, anon, nil
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
