package docker

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"testing"

	"github.com/shahin-bayat/lokl/internal/config"
)

var _ DockerAPI = (*mockAPI)(nil)

type mockAPI struct {
	PullImageFn           func(ctx context.Context, image string, onProgress func(string)) error
	ImageExistsFn         func(ctx context.Context, image string) (bool, error)
	FindContainerByNameFn func(ctx context.Context, name string) (string, error)
	CreateContainerFn     func(ctx context.Context, cfg ContainerConfig) (string, error)
	StartContainerFn      func(ctx context.Context, id string) error
	StopContainerFn       func(ctx context.Context, id string, timeoutSeconds int) error
	RemoveContainerFn     func(ctx context.Context, id string) error
	IsContainerRunningFn  func(ctx context.Context, id string) (bool, error)
	StreamLogsFn          func(ctx context.Context, id string, follow bool) (io.ReadCloser, error)
	CloseFn               func() error
}

func (m *mockAPI) PullImage(ctx context.Context, image string, onProgress func(string)) error {
	if m.PullImageFn != nil {
		return m.PullImageFn(ctx, image, onProgress)
	}
	return nil
}

func (m *mockAPI) ImageExists(ctx context.Context, image string) (bool, error) {
	if m.ImageExistsFn != nil {
		return m.ImageExistsFn(ctx, image)
	}
	return true, nil
}

func (m *mockAPI) FindContainerByName(ctx context.Context, name string) (string, error) {
	if m.FindContainerByNameFn != nil {
		return m.FindContainerByNameFn(ctx, name)
	}
	return "", nil
}

func (m *mockAPI) CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error) {
	if m.CreateContainerFn != nil {
		return m.CreateContainerFn(ctx, cfg)
	}
	return "test-id", nil
}

func (m *mockAPI) StartContainer(ctx context.Context, id string) error {
	if m.StartContainerFn != nil {
		return m.StartContainerFn(ctx, id)
	}
	return nil
}

func (m *mockAPI) StopContainer(ctx context.Context, id string, timeoutSeconds int) error {
	if m.StopContainerFn != nil {
		return m.StopContainerFn(ctx, id, timeoutSeconds)
	}
	return nil
}

func (m *mockAPI) RemoveContainer(ctx context.Context, id string) error {
	if m.RemoveContainerFn != nil {
		return m.RemoveContainerFn(ctx, id)
	}
	return nil
}

func (m *mockAPI) IsContainerRunning(ctx context.Context, id string) (bool, error) {
	if m.IsContainerRunningFn != nil {
		return m.IsContainerRunningFn(ctx, id)
	}
	return true, nil
}

func (m *mockAPI) StreamLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error) {
	if m.StreamLogsFn != nil {
		return m.StreamLogsFn(ctx, id, follow)
	}
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (m *mockAPI) Close() error {
	if m.CloseFn != nil {
		return m.CloseFn()
	}
	return nil
}

func TestContainerStartStop(t *testing.T) {
	c := NewContainer("web", config.Service{Image: "nginx"}, &mockAPI{}, func() {})

	if c.IsRunning() {
		t.Fatal("expected not running before start")
	}
	if c.Logs() != nil {
		t.Fatal("expected nil logs before start")
	}

	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !c.IsRunning() {
		t.Fatal("expected running after start")
	}

	if err := c.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if c.IsRunning() {
		t.Fatal("expected not running after stop")
	}
}

func TestContainerStartPullsImage(t *testing.T) {
	pulled := false
	mock := &mockAPI{
		ImageExistsFn: func(_ context.Context, _ string) (bool, error) {
			return false, nil
		},
		PullImageFn: func(_ context.Context, image string, _ func(string)) error {
			pulled = true
			if image != "nginx:latest" {
				t.Errorf("pulled image = %q, want %q", image, "nginx:latest")
			}
			return nil
		},
	}

	c := NewContainer("web", config.Service{Image: "nginx:latest"}, mock, func() {})
	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = c.Stop() }()

	if !pulled {
		t.Error("expected image pull when not present locally")
	}
}

func TestContainerStartCleansStaleContainer(t *testing.T) {
	var stoppedIDs, removedIDs []string
	mock := &mockAPI{
		FindContainerByNameFn: func(_ context.Context, _ string) (string, error) {
			return "stale-id", nil
		},
		StopContainerFn: func(_ context.Context, id string, _ int) error {
			stoppedIDs = append(stoppedIDs, id)
			return nil
		},
		RemoveContainerFn: func(_ context.Context, id string) error {
			removedIDs = append(removedIDs, id)
			return nil
		},
	}

	c := NewContainer("web", config.Service{Image: "nginx"}, mock, func() {})
	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = c.Stop() }()

	if len(stoppedIDs) == 0 || stoppedIDs[0] != "stale-id" {
		t.Errorf("expected stale container stopped first, got %v", stoppedIDs)
	}
	if len(removedIDs) == 0 || removedIDs[0] != "stale-id" {
		t.Errorf("expected stale container removed first, got %v", removedIDs)
	}
}

func TestContainerStartCreateError(t *testing.T) {
	mock := &mockAPI{
		CreateContainerFn: func(_ context.Context, _ ContainerConfig) (string, error) {
			return "", fmt.Errorf("no space left")
		},
	}

	c := NewContainer("web", config.Service{Image: "nginx"}, mock, func() {})
	if err := c.Start(); err == nil {
		t.Fatal("expected error from start")
	}
	if c.IsRunning() {
		t.Error("expected not running after failed create")
	}
}

func TestContainerStartAlreadyRunning(t *testing.T) {
	c := NewContainer("web", config.Service{Image: "nginx"}, &mockAPI{}, func() {})
	if err := c.Start(); err != nil {
		t.Fatalf("first start: %v", err)
	}
	defer func() { _ = c.Stop() }()

	if err := c.Start(); err == nil {
		t.Fatal("expected error from second start")
	}
}

func TestContainerStopWhenStopped(t *testing.T) {
	c := NewContainer("web", config.Service{Image: "nginx"}, &mockAPI{}, func() {})
	if err := c.Stop(); err != nil {
		t.Fatalf("stop when already stopped: %v", err)
	}
}

func TestParsePorts(t *testing.T) {
	tests := []struct {
		name    string
		raw     []string
		want    []PortMapping
		wantErr bool
	}{
		{"single mapping", []string{"8080:80"}, []PortMapping{{Host: 8080, Container: 80}}, false},
		{"multiple", []string{"8080:80", "3000:3000"}, []PortMapping{{Host: 8080, Container: 80}, {Host: 3000, Container: 3000}}, false},
		{"empty", nil, []PortMapping{}, false},
		{"invalid format", []string{"bad"}, nil, true},
		{"non-numeric host", []string{"abc:80"}, nil, true},
		{"non-numeric container", []string{"80:abc"}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePorts(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d ports, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("port[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func buildDockerFrame(streamType byte, payload []byte) []byte {
	header := make([]byte, streamHeaderSize)
	header[0] = streamType
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
	return append(header, payload...)
}

func TestDemuxReader(t *testing.T) {
	t.Run("strips headers from mixed streams", func(t *testing.T) {
		var buf bytes.Buffer
		buf.Write(buildDockerFrame(1, []byte("hello ")))
		buf.Write(buildDockerFrame(2, []byte("world")))

		r := newDemuxReader(io.NopCloser(&buf))
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if string(got) != "hello world" {
			t.Errorf("got %q, want %q", got, "hello world")
		}
	})

	t.Run("empty stream", func(t *testing.T) {
		r := newDemuxReader(io.NopCloser(bytes.NewReader(nil)))
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %d bytes, want 0", len(got))
		}
	})
}

func TestLogs(t *testing.T) {
	t.Run("basic write and read", func(t *testing.T) {
		l := newLogs(10)
		_, _ = l.Write([]byte("line1\nline2\nline3\n"))
		lines := l.Lines()
		if len(lines) != 3 {
			t.Errorf("got %d lines, want 3", len(lines))
		}
		if lines[0] != "line1" {
			t.Errorf("lines[0] = %q, want %q", lines[0], "line1")
		}
	})

	t.Run("exceeds max lines", func(t *testing.T) {
		l := newLogs(3)
		_, _ = l.Write([]byte("a\nb\nc\nd\ne\n"))
		lines := l.Lines()
		if len(lines) != 3 {
			t.Errorf("got %d lines, want 3", len(lines))
		}
		if lines[0] != "c" {
			t.Errorf("oldest = %q, want %q", lines[0], "c")
		}
	})

	t.Run("partial line", func(t *testing.T) {
		l := newLogs(10)
		_, _ = l.Write([]byte("complete\npartial"))
		_, _ = l.Write([]byte(" continued\n"))
		lines := l.Lines()
		if len(lines) != 2 {
			t.Errorf("got %d lines, want 2", len(lines))
		}
		if lines[1] != "partial continued" {
			t.Errorf("lines[1] = %q, want %q", lines[1], "partial continued")
		}
	})
}

func TestContainerStartPullError(t *testing.T) {
	mock := &mockAPI{
		ImageExistsFn: func(_ context.Context, _ string) (bool, error) {
			return false, nil
		},
		PullImageFn: func(_ context.Context, _ string, _ func(string)) error {
			return fmt.Errorf("network timeout")
		},
	}

	c := NewContainer("web", config.Service{Image: "nginx"}, mock, func() {})
	if err := c.Start(); err == nil {
		t.Fatal("expected error from pull failure")
	}
	if c.IsRunning() {
		t.Error("expected not running after pull error")
	}
}

func TestContainerStartImageCheckError(t *testing.T) {
	mock := &mockAPI{
		ImageExistsFn: func(_ context.Context, _ string) (bool, error) {
			return false, fmt.Errorf("docker daemon unreachable")
		},
	}

	c := NewContainer("web", config.Service{Image: "nginx"}, mock, func() {})
	if err := c.Start(); err == nil {
		t.Fatal("expected error from image check failure")
	}
	if c.IsRunning() {
		t.Error("expected not running after image check error")
	}
}

func TestContainerStartContainerStartError(t *testing.T) {
	removed := false
	mock := &mockAPI{
		StartContainerFn: func(_ context.Context, _ string) error {
			return fmt.Errorf("permission denied")
		},
		RemoveContainerFn: func(_ context.Context, _ string) error {
			removed = true
			return nil
		},
	}

	c := NewContainer("web", config.Service{Image: "nginx"}, mock, func() {})
	if err := c.Start(); err == nil {
		t.Fatal("expected error from container start failure")
	}
	if c.IsRunning() {
		t.Error("expected not running after start error")
	}
	if !removed {
		t.Error("expected container cleanup (remove) after start error")
	}
}

func TestContainerStartWithEnv(t *testing.T) {
	var gotCfg ContainerConfig
	mock := &mockAPI{
		CreateContainerFn: func(_ context.Context, cfg ContainerConfig) (string, error) {
			gotCfg = cfg
			return "test-id", nil
		},
	}

	svc := config.Service{
		Image: "nginx",
		Env:   map[string]string{"NODE_ENV": "production", "PORT": "3000"},
	}
	c := NewContainer("web", svc, mock, func() {})
	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = c.Stop() }()

	if len(gotCfg.Env) != 2 {
		t.Errorf("env count = %d, want 2", len(gotCfg.Env))
	}
	if gotCfg.Env["NODE_ENV"] != "production" {
		t.Errorf("NODE_ENV = %q, want production", gotCfg.Env["NODE_ENV"])
	}
}

func TestContainerStartWithVolumes(t *testing.T) {
	var gotCfg ContainerConfig
	mock := &mockAPI{
		CreateContainerFn: func(_ context.Context, cfg ContainerConfig) (string, error) {
			gotCfg = cfg
			return "test-id", nil
		},
	}

	svc := config.Service{
		Image:   "postgres",
		Volumes: []string{"./data:/var/lib/postgresql/data"},
		Ports:   []string{"5432:5432"},
	}
	c := NewContainer("db", svc, mock, func() {})
	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = c.Stop() }()

	if len(gotCfg.Volumes) != 1 {
		t.Errorf("volumes count = %d, want 1", len(gotCfg.Volumes))
	}
	if gotCfg.Volumes[0] != "./data:/var/lib/postgresql/data" {
		t.Errorf("volume = %q, want ./data:/var/lib/postgresql/data", gotCfg.Volumes[0])
	}
	if len(gotCfg.Ports) != 1 || gotCfg.Ports[0].Host != 5432 {
		t.Errorf("ports = %+v, want [{5432 5432 }]", gotCfg.Ports)
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		s    state
		want string
	}{
		{stateStopped, "stopped"},
		{stateStarting, "starting"},
		{stateRunning, "running"},
		{stateStopping, "stopping"},
		{stateFailed, "failed"},
		{state(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("state(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}
