package docker

import (
	"context"
	"io"
)

type ContainerConfig struct {
	Name           string
	Image          string
	Env            map[string]string
	Ports          []PortMapping
	Volumes        []string
	Labels         map[string]string
	Network        string   // "lokl-{project}"; empty = default bridge
	NetworkAliases []string // DNS aliases inside the project network (e.g. service name)
}

type PortMapping struct {
	Host      int
	Container int
	Protocol  string // "tcp" or "udp", defaults to "tcp"
}

// DockerAPI abstracts Docker operations behind lokl-specific types.
type DockerAPI interface {
	PullImage(ctx context.Context, image string, onProgress func(string)) error
	ImageExists(ctx context.Context, image string) (bool, error)

	FindContainerByName(ctx context.Context, name string) (string, error)
	CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string, timeoutSeconds int) error
	RemoveContainer(ctx context.Context, id string) error
	IsContainerRunning(ctx context.Context, id string) (bool, error)
	StreamLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error)

	Close() error
	EnsureProjectNetwork(ctx context.Context, name string) error
	RemoveNetwork(ctx context.Context, name string) error
	// ExecContainer execs cmd inside the running container and returns the exit code.
	// cmd is taken verbatim — shell wrapping is the caller's responsibility.
	ExecContainer(ctx context.Context, id string, cmd []string, env []string) (int, error)
}
