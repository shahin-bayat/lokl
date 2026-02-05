// Package docker provides Docker container management for lokl services.
package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/go-sdk/client"
	"github.com/docker/go-sdk/image"
	mobyclient "github.com/moby/moby/client"
)

const (
	connectTimeout = 5 * time.Second
)

// Client wraps the Docker SDK with lokl-specific operations.
type Client struct {
	api client.SDKClient
}

// NewClient creates a Docker client and verifies daemon connectivity.
func NewClient() (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	api, err := client.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	// Verify daemon is reachable
	if _, err := api.Ping(ctx, mobyclient.PingOptions{}); err != nil {
		return nil, fmt.Errorf("docker daemon not running: %w", err)
	}

	return &Client{api: api}, nil
}

// Close closes the Docker client connection.
func (c *Client) Close() error {
	return c.api.Close()
}

// PullImage pulls a Docker image, reporting progress via callback.
func (c *Client) PullImage(ctx context.Context, imageName string, onProgress func(string)) error {
	opts := []image.PullOption{
		image.WithPullClient(c.api),
	}

	if onProgress != nil {
		opts = append(opts, image.WithPullHandler(func(r io.ReadCloser) error {
			defer r.Close()
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				onProgress(scanner.Text())
			}
			return scanner.Err()
		}))
	}

	if err := image.Pull(ctx, imageName, opts...); err != nil {
		return fmt.Errorf("pulling image %s: %w", imageName, err)
	}

	return nil
}

// ImageExists checks if an image exists locally.
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

// isNotFoundError checks if error indicates resource not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "not found") || strings.Contains(errStr, "No such image")
}
