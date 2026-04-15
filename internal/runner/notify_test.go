package runner

import (
	"testing"
)

func TestNotifySkipsWhenOptOut(t *testing.T) {
	t.Setenv("LOKL_NO_NOTIFICATIONS", "1")
	// Should return without panicking or doing anything.
	Notify("myproject", "api")
}

func TestNotifyNoopWhenNoTool(t *testing.T) {
	t.Setenv("LOKL_NO_NOTIFICATIONS", "")
	// Restrict PATH so neither osascript nor notify-send is found.
	t.Setenv("PATH", "/nonexistent")
	// Should return silently without panicking.
	Notify("myproject", "api")
}
