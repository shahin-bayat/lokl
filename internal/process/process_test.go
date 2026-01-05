package process

import "testing"

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
		state state
		want  string
	}{
		{stateStopped, "stopped"},
		{stateStarting, "starting"},
		{stateRunning, "running"},
		{stateStopping, "stopping"},
		{stateFailed, "failed"},
		{state(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("state(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
