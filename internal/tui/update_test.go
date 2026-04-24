package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shahin-bayat/lokl/internal/types"
)

type restartFakeController struct {
	svcs         []types.ServiceInfo
	restartCalls int
}

func (f *restartFakeController) StartService(_ string) error { return nil }
func (f *restartFakeController) StopService(_ string) error  { return nil }
func (f *restartFakeController) RestartService(_ string) error {
	f.restartCalls++
	return nil
}
func (f *restartFakeController) ToggleProxy(_ string) (bool, error) { return false, nil }
func (f *restartFakeController) Services() []types.ServiceInfo      { return f.svcs }
func (f *restartFakeController) ServiceLogs(_ string) []string      { return nil }
func (f *restartFakeController) ProjectName() string                { return "test" }
func (f *restartFakeController) Subscribe() <-chan types.Event      { return make(chan types.Event) }

func TestRestartKeyNoopsOnProxyOnly(t *testing.T) {
	ctrl := &restartFakeController{
		svcs: []types.ServiceInfo{
			{Name: "console", Port: 9001, ProxyOnly: true, Running: true},
		},
	}
	m := newModel(ctrl)
	m.selectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if ctrl.restartCalls != 0 {
		t.Fatalf("RestartService should not be called on proxy-only; calls=%d", ctrl.restartCalls)
	}
	if !strings.Contains(m.copyMsg, "proxy-only") {
		t.Fatalf("expected toast about proxy-only; got %q", m.copyMsg)
	}
}

func TestRestartKeyCallsRestartOnNormalService(t *testing.T) {
	ctrl := &restartFakeController{
		svcs: []types.ServiceInfo{
			{Name: "api", Port: 3000, Running: true},
		},
	}
	m := newModel(ctrl)
	m.selectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	updated, _ := m.Update(msg)
	_ = updated.(Model)

	if ctrl.restartCalls != 1 {
		t.Fatalf("RestartService should be called once; got %d", ctrl.restartCalls)
	}
}
