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
	startCalls   int
	stopCalls    int
}

func (f *restartFakeController) StartService(_ string) error {
	f.startCalls++
	return nil
}
func (f *restartFakeController) StopService(_ string) error {
	f.stopCalls++
	return nil
}
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

func TestStartKeyNoopsOnProxyOnly(t *testing.T) {
	ctrl := &restartFakeController{
		svcs: []types.ServiceInfo{
			{Name: "console", Port: 9001, ProxyOnly: true, Running: false},
		},
	}
	m := newModel(ctrl)
	m.selectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if ctrl.startCalls != 0 {
		t.Fatalf("StartService should not be called on proxy-only; calls=%d", ctrl.startCalls)
	}
	if !strings.Contains(m.copyMsg, "proxy-only") {
		t.Fatalf("expected proxy-only toast; got %q", m.copyMsg)
	}
}

func TestStopKeyNoopsOnProxyOnly(t *testing.T) {
	ctrl := &restartFakeController{
		svcs: []types.ServiceInfo{
			{Name: "console", Port: 9001, ProxyOnly: true, Running: true},
		},
	}
	m := newModel(ctrl)
	m.selectedIdx = 0

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	updated, _ := m.Update(msg)
	m = updated.(Model)

	if ctrl.stopCalls != 0 {
		t.Fatalf("StopService should not be called on proxy-only; calls=%d", ctrl.stopCalls)
	}
	if !strings.Contains(m.copyMsg, "proxy-only") {
		t.Fatalf("expected proxy-only toast; got %q", m.copyMsg)
	}
}
