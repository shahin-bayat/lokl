package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/shahin-bayat/lokl/internal/types"
)

func TestStyleProxyOffExists(t *testing.T) {
	want := lipgloss.Color("#F4A227")
	got := styleProxyOff.GetForeground()
	if got != want {
		t.Errorf("styleProxyOff foreground: want %q, got %q", want, got)
	}
}

func TestStyleLinkColorIsLightPurple(t *testing.T) {
	wantColor := lipgloss.Color("#A78BFA")
	if styleLink.GetForeground() != wantColor {
		t.Errorf("styleLink foreground: want %q, got %q", wantColor, styleLink.GetForeground())
	}
	if !styleLink.GetBold() {
		t.Error("styleLink: expected bold")
	}
}

func newTestModel(svcs []types.ServiceInfo) Model {
	ctrl := &polishFakeController{svcs: svcs}
	m := newModel(ctrl)
	m.width = 120
	return m
}

type polishFakeController struct{ svcs []types.ServiceInfo }

func (f *polishFakeController) StartService(_ string) error        { return nil }
func (f *polishFakeController) StopService(_ string) error         { return nil }
func (f *polishFakeController) RestartService(_ string) error      { return nil }
func (f *polishFakeController) ToggleProxy(_ string) (bool, error) { return false, nil }
func (f *polishFakeController) Services() []types.ServiceInfo      { return f.svcs }
func (f *polishFakeController) ServiceLogs(_ string) []string      { return nil }
func (f *polishFakeController) ProjectName() string                { return "test" }
func (f *polishFakeController) Subscribe() <-chan types.Event {
	return make(chan types.Event)
}

func TestUnhealthyRowNotSelectedHasTint(t *testing.T) {
	svc := types.ServiceInfo{Name: "api", Running: true, Healthy: false}
	m := newTestModel([]types.ServiceInfo{svc})

	row := m.renderServiceRow(svc, false, 16)
	rowNoTint := m.renderServiceRow(types.ServiceInfo{Name: "api", Running: true, Healthy: true}, false, 16)
	if row == rowNoTint {
		t.Error("unhealthy unselected row should differ from healthy row (tint not applied)")
	}
}

func TestUnhealthyRowSelectedNoTint(t *testing.T) {
	svc := types.ServiceInfo{Name: "api", Running: true, Healthy: false}
	m := newTestModel([]types.ServiceInfo{svc})

	selectedRow := m.renderServiceRow(svc, true, 16)
	unselectedRow := m.renderServiceRow(svc, false, 16)
	if selectedRow == unselectedRow {
		t.Error("selected and unselected rows should differ")
	}
}
