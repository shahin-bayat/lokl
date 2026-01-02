package process

import (
	"strings"
	"testing"

	"github.com/shahin-bayat/lokl/internal/config"
)

func TestSortByDependency(t *testing.T) {
	tests := []struct {
		name     string
		services map[string]config.Service
		wantErr  string
		validate func([]string) bool
	}{
		{
			name: "no dependencies",
			services: map[string]config.Service{
				"a": {Command: "x"},
				"b": {Command: "x"},
			},
			validate: func(order []string) bool {
				return len(order) == 2
			},
		},
		{
			name: "linear chain",
			services: map[string]config.Service{
				"api": {Command: "x", DependsOn: []string{"db"}},
				"db":  {Command: "x"},
			},
			validate: func(order []string) bool {
				return indexOf(order, "db") < indexOf(order, "api")
			},
		},
		{
			name: "multiple dependencies",
			services: map[string]config.Service{
				"api":   {Command: "x", DependsOn: []string{"db", "redis"}},
				"db":    {Command: "x"},
				"redis": {Command: "x"},
			},
			validate: func(order []string) bool {
				apiIdx := indexOf(order, "api")
				return indexOf(order, "db") < apiIdx && indexOf(order, "redis") < apiIdx
			},
		},
		{
			name: "circular dependency",
			services: map[string]config.Service{
				"a": {Command: "x", DependsOn: []string{"b"}},
				"b": {Command: "x", DependsOn: []string{"a"}},
			},
			wantErr: "circular dependency",
		},
		{
			name: "unknown dependency",
			services: map[string]config.Service{
				"a": {Command: "x", DependsOn: []string{"unknown"}},
			},
			wantErr: "unknown service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order, err := SortByDependency(tt.services)
			if tt.wantErr != "" {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.validate(order) {
				t.Errorf("invalid order: %v", order)
			}
		})
	}
}

func TestLineBuffer(t *testing.T) {
	t.Run("basic write and read", func(t *testing.T) {
		buf := newLineBuffer(10)
		_, _ = buf.Write([]byte("line1\nline2\nline3\n"))

		lines := buf.Lines()
		if len(lines) != 3 {
			t.Errorf("got %d lines, want 3", len(lines))
		}
		if lines[0] != "line1" {
			t.Errorf("lines[0] = %q, want %q", lines[0], "line1")
		}
	})

	t.Run("exceeds max lines", func(t *testing.T) {
		buf := newLineBuffer(3)
		_, _ = buf.Write([]byte("a\nb\nc\nd\ne\n"))

		lines := buf.Lines()
		if len(lines) != 3 {
			t.Errorf("got %d lines, want 3", len(lines))
		}
		if lines[0] != "c" {
			t.Errorf("oldest line should be 'c', got %q", lines[0])
		}
	})

	t.Run("partial line", func(t *testing.T) {
		buf := newLineBuffer(10)
		_, _ = buf.Write([]byte("complete\npartial"))
		_, _ = buf.Write([]byte(" continued\n"))

		lines := buf.Lines()
		if len(lines) != 2 {
			t.Errorf("got %d lines, want 2", len(lines))
		}
		if lines[1] != "partial continued" {
			t.Errorf("lines[1] = %q, want %q", lines[1], "partial continued")
		}
	})
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateStopped, "stopped"},
		{StateStarting, "starting"},
		{StateRunning, "running"},
		{StateStopping, "stopping"},
		{StateFailed, "failed"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
