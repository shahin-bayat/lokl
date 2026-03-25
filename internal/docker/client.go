package docker

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

const (
	connectTimeout = 5 * time.Second
	labelManagedBy = "managed-by"
	labelManagedV  = "lokl"
)

var _ DockerAPI = (*Client)(nil)

type Client struct {
	api client.APIClient
}

func NewClient() (*Client, error) {
	api, err := client.New(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	if _, err := api.Ping(ctx, client.PingOptions{}); err != nil {
		_ = api.Close()
		return nil, fmt.Errorf("docker daemon not running: %w", err)
	}

	return &Client{api: api}, nil
}

func (c *Client) Close() error {
	return c.api.Close()
}

func (c *Client) PullImage(ctx context.Context, imageName string, onProgress func(string)) error {
	resp, err := c.api.ImagePull(ctx, imageName, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", imageName, err)
	}
	defer func() { _ = resp.Close() }()

	if onProgress != nil {
		buf := make([]byte, 4096)
		for {
			n, readErr := resp.Read(buf)
			if n > 0 {
				onProgress(strings.TrimSpace(string(buf[:n])))
			}
			if readErr != nil {
				if readErr == io.EOF {
					break
				}
				return fmt.Errorf("reading pull progress for %s: %w", imageName, readErr)
			}
		}
	} else {
		if _, err := io.Copy(io.Discard, resp); err != nil {
			return fmt.Errorf("pulling image %s: %w", imageName, err)
		}
	}

	return nil
}

func (c *Client) ImageExists(ctx context.Context, imageName string) (bool, error) {
	_, err := c.api.ImageInspect(ctx, imageName)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspecting image %s: %w", imageName, err)
	}
	return true, nil
}

func (c *Client) FindContainerByName(ctx context.Context, name string) (string, error) {
	resp, err := c.api.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("name", "^"+name+"$"),
	})
	if err != nil {
		return "", fmt.Errorf("listing containers: %w", err)
	}
	if len(resp.Items) == 0 {
		return "", nil
	}
	return resp.Items[0].ID, nil
}

func (c *Client) CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error) {
	env := make([]string, 0, len(cfg.Env))
	for k, v := range cfg.Env {
		env = append(env, k+"="+v)
	}

	labels := cfg.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[labelManagedBy] = labelManagedV

	exposedPorts := make(network.PortSet, len(cfg.Ports))
	bindings := make(network.PortMap, len(cfg.Ports))
	for _, p := range cfg.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		port, err := network.ParsePort(fmt.Sprintf("%d/%s", p.Container, proto))
		if err != nil {
			return "", fmt.Errorf("parsing port %d/%s: %w", p.Container, proto, err)
		}
		exposedPorts[port] = struct{}{}
		bindings[port] = []network.PortBinding{
			{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: fmt.Sprintf("%d", p.Host)},
		}
	}

	resp, err := c.api.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: cfg.Name,
		Config: &container.Config{
			Image:        cfg.Image,
			Env:          env,
			Labels:       labels,
			ExposedPorts: exposedPorts,
		},
		HostConfig: &container.HostConfig{
			Binds:        cfg.Volumes,
			PortBindings: bindings,
		},
	})
	if err != nil {
		return "", fmt.Errorf("creating container: %w", err)
	}

	if cfg.Network != "" {
		if _, err := c.api.NetworkConnect(ctx, cfg.Network, client.NetworkConnectOptions{
			Container: resp.ID,
		}); err != nil {
			_ = c.RemoveContainer(ctx, resp.ID)
			return "", fmt.Errorf("attaching container to network %q: %w", cfg.Network, err)
		}
	}

	return resp.ID, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	if _, err := c.api.ContainerStart(ctx, id, client.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("starting container %s: %w", shortID(id), err)
	}
	return nil
}

func (c *Client) StopContainer(ctx context.Context, id string, timeoutSeconds int) error {
	if _, err := c.api.ContainerStop(ctx, id, client.ContainerStopOptions{
		Timeout: &timeoutSeconds,
	}); err != nil {
		return fmt.Errorf("stopping container %s: %w", shortID(id), err)
	}
	return nil
}

func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	if _, err := c.api.ContainerRemove(ctx, id, client.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}); err != nil {
		return fmt.Errorf("removing container %s: %w", shortID(id), err)
	}
	return nil
}

func (c *Client) IsContainerRunning(ctx context.Context, id string) (bool, error) {
	resp, err := c.api.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspecting container %s: %w", shortID(id), err)
	}
	return resp.Container.State != nil && resp.Container.State.Running, nil
}

func (c *Client) StreamLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error) {
	rc, err := c.api.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
	})
	if err != nil {
		return nil, fmt.Errorf("streaming logs for container %s: %w", shortID(id), err)
	}

	// Docker multiplexes stdout/stderr with 8-byte headers when TTY is disabled.
	// We strip those headers so consumers get clean log lines.
	return newDemuxReader(rc), nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "not found") || strings.Contains(errStr, "No such")
}

func (c *Client) EnsureProjectNetwork(ctx context.Context, name string) error {
	result, err := c.api.NetworkList(ctx, client.NetworkListOptions{
		Filters: make(client.Filters).Add("name", name),
	})
	if err != nil {
		return fmt.Errorf("listing networks: %w", err)
	}
	for _, n := range result.Items {
		if n.Name == name {
			return nil // already exists
		}
	}
	if _, err := c.api.NetworkCreate(ctx, name, client.NetworkCreateOptions{
		Driver: "bridge",
	}); err != nil {
		return fmt.Errorf("creating network %q: %w", name, err)
	}
	return nil
}

func (c *Client) RemoveNetwork(ctx context.Context, name string) error {
	if _, err := c.api.NetworkRemove(ctx, name, client.NetworkRemoveOptions{}); err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("removing network %q: %w", name, err)
	}
	return nil
}

func (c *Client) ExecContainer(ctx context.Context, id string, cmd []string) (int, error) {
	exec, err := c.api.ExecCreate(ctx, id, client.ExecCreateOptions{
		Cmd:          cmd,
		AttachStdout: false,
		AttachStderr: false,
		AttachStdin:  false,
	})
	if err != nil {
		return -1, fmt.Errorf("exec create: %w", err)
	}

	// Detach: true returns immediately; we then poll ExecInspect for completion.
	if _, err := c.api.ExecStart(ctx, exec.ID, client.ExecStartOptions{Detach: true}); err != nil {
		return -1, fmt.Errorf("exec start: %w", err)
	}

	// Poll until exec completes or ctx is cancelled.
	for {
		insp, err := c.api.ExecInspect(ctx, exec.ID, client.ExecInspectOptions{})
		if err != nil {
			return -1, fmt.Errorf("exec inspect: %w", err)
		}
		if !insp.Running {
			return insp.ExitCode, nil
		}
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
