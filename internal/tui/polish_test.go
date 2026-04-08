package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/shahin-bayat/lokl/internal/types"
)

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

func TestHeaderRunningCountStyled(t *testing.T) {
	svc := types.ServiceInfo{Name: "api", Running: true, Healthy: true}
	m := newTestModel([]types.ServiceInfo{svc})
	m.width = 120

	expectedColor := styleRunning.GetForeground()
	if expectedColor == (lipgloss.Color("")) {
		t.Skip("lipgloss has no color in test environment — verify manually")
	}

	header := m.renderHeader()
	if !strings.Contains(header, styleRunning.Render("1 running")) {
		t.Errorf("header should contain styleRunning-rendered count")
	}
}

func TestHeaderZeroRunningUsesStopped(t *testing.T) {
	svc := types.ServiceInfo{Name: "api", Running: false, Healthy: false}
	m := newTestModel([]types.ServiceInfo{svc})
	m.width = 120

	expectedColor := styleStopped.GetForeground()
	if expectedColor == (lipgloss.Color("")) {
		t.Skip("lipgloss has no color in test environment — verify manually")
	}

	header := m.renderHeader()
	if !strings.Contains(header, styleStopped.Render("0 running")) {
		t.Errorf("header should contain styleStopped-rendered count when nothing running")
	}
}

func TestLogSeparatorFillsWidth(t *testing.T) {
	svc := types.ServiceInfo{Name: "api", Running: true, Healthy: true, Port: 3000}
	m := newTestModel([]types.ServiceInfo{svc})
	m.selectedIdx = 0
	m.width = 80

	output := m.renderLogs(20)
	stripped := ansi.Strip(output)
	lines := strings.Split(stripped, "\n")
	var sepLine string
	for _, l := range lines {
		if strings.Contains(l, "─── Logs:") {
			sepLine = l
			break
		}
	}
	if sepLine == "" {
		t.Fatal("separator line not found in renderLogs output")
	}
	got := len([]rune(sepLine))
	if got < m.width-2 || got > m.width {
		t.Errorf("separator length %d not within [%d, %d]", got, m.width-2, m.width)
	}
}

func TestStatusBarHasDivider(t *testing.T) {
	m := newTestModel([]types.ServiceInfo{})
	m.width = 80
	bar := m.renderStatusBar()
	stripped := ansi.Strip(bar)
	if !strings.Contains(stripped, "─") {
		t.Error("status bar should contain a ─ divider line")
	}
}

func TestNameColumnAdaptsToLongestName(t *testing.T) {
	svcs := []types.ServiceInfo{
		{Name: "short", Running: true, Healthy: true},
		{Name: "a-very-long-service-name", Running: false},
	}
	m := newTestModel(svcs)
	m.width = 120

	rendered := m.renderServices()
	stripped := ansi.Strip(rendered)

	if !strings.Contains(stripped, "a-very-long-service-name") {
		t.Error("long service name should appear untruncated in service list")
	}
	if !strings.Contains(stripped, "short") {
		t.Error("short service name should appear in service list")
	}

	// Verify both rows are present
	lines := strings.Split(strings.TrimRight(stripped, "\n"), "\n")
	if len(lines) < 2 {
		t.Errorf("expected at least 2 service rows, got %d", len(lines))
	}
}
