package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/x/ansi"
)

func TestCopyFlashShowsInStatusBar(t *testing.T) {
	m := newTestModel(nil)
	m.width = 80
	m.copyMsg = "Logs copied"
	m.copyExpiry = time.Now().Add(2 * time.Second)

	bar := m.renderStatusBar()
	stripped := ansi.Strip(bar)
	if !strings.Contains(stripped, "Logs copied") {
		t.Errorf("status bar should show flash message; got: %q", stripped)
	}
}

func TestCopyFlashHiddenAfterExpiry(t *testing.T) {
	m := newTestModel(nil)
	m.width = 80
	m.copyMsg = "Logs copied"
	m.copyExpiry = time.Now().Add(-1 * time.Second) // already expired

	bar := m.renderStatusBar()
	stripped := ansi.Strip(bar)
	if strings.Contains(stripped, "Logs copied") {
		t.Error("expired flash should not appear in status bar")
	}
}

func TestCopyDoneMsgClearsFlashWhenExpired(t *testing.T) {
	m := newTestModel(nil)
	m.copyMsg = "Logs copied"
	m.copyExpiry = time.Now().Add(-1 * time.Millisecond) // just expired

	next, _ := m.Update(copyDoneMsg{})
	nm := next.(Model)

	if nm.copyMsg != "" {
		t.Errorf("copyDoneMsg should clear copyMsg after expiry; got %q", nm.copyMsg)
	}
}

func TestCopyDoneMsgKeepsFlashWhenNotExpired(t *testing.T) {
	// simulates stale first timer after a second press reset the expiry
	m := newTestModel(nil)
	m.copyMsg = "Logs copied"
	m.copyExpiry = time.Now().Add(2 * time.Second)

	next, _ := m.Update(copyDoneMsg{})
	nm := next.(Model)

	if nm.copyMsg == "" {
		t.Error("stale copyDoneMsg should not clear copyMsg when expiry is still active")
	}
}

func TestCopyKeyWithNoServiceIsNoop(t *testing.T) {
	m := newTestModel(nil) // no services → selectedService() = nil

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	nm := next.(Model)

	if nm.copyMsg != "" {
		t.Error("pressing 'c' with no service selected should not set copyMsg")
	}
	if cmd != nil {
		t.Error("pressing 'c' with no service selected should return nil cmd")
	}
}

func TestStatusBarShowsNormalKeysWhenNoFlash(t *testing.T) {
	m := newTestModel(nil)
	m.width = 80
	// copyMsg is "" — no flash

	bar := m.renderStatusBar()
	stripped := ansi.Strip(bar)
	if !strings.Contains(stripped, "navigate") {
		t.Errorf("status bar should show normal keys when no flash; got: %q", stripped)
	}
}
