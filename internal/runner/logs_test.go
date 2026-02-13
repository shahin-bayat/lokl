package runner

import "testing"

func TestLogs(t *testing.T) {
	t.Run("basic write and read", func(t *testing.T) {
		l := NewLogs(10)
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
		l := NewLogs(3)
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
		l := NewLogs(10)
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
