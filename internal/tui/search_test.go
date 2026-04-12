package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/shahin-bayat/lokl/internal/types"
)

func TestSlashKeyOpensSearchInLogView(t *testing.T) {
	m := newTestModel(nil)
	m.showLogs = true

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	nm := next.(Model)

	if !nm.showSearch {
		t.Error("pressing '/' with showLogs=true should open search")
	}
}

func TestSlashKeyNoopOutsideLogView(t *testing.T) {
	m := newTestModel(nil)
	m.showLogs = false

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	nm := next.(Model)

	if nm.showSearch {
		t.Error("pressing '/' outside log view should not open search")
	}
}

func TestEscInSearchModeClearsQueryAndExitsSearch(t *testing.T) {
	m := newTestModel(nil)
	m.showLogs = true
	m.showSearch = true
	m.searchQuery = "error"
	m.searchInput.SetValue("error")

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	nm := next.(Model)

	if nm.showSearch {
		t.Error("esc in search mode should set showSearch=false")
	}
	if nm.searchQuery != "" {
		t.Errorf("esc in search mode should clear searchQuery; got %q", nm.searchQuery)
	}
}

func TestEscInSearchModeDoesNotCloseLogView(t *testing.T) {
	m := newTestModel(nil)
	m.showLogs = true
	m.showSearch = true

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	nm := next.(Model)

	if !nm.showLogs {
		t.Error("esc in search mode should NOT close the log view")
	}
}

func TestEnterInSearchModeExitsSearchKeepsQuery(t *testing.T) {
	m := newTestModel(nil)
	m.showLogs = true
	m.showSearch = true
	m.searchQuery = "warn"
	m.searchInput.SetValue("warn")

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	nm := next.(Model)

	if nm.showSearch {
		t.Error("enter in search mode should set showSearch=false")
	}
	if nm.searchQuery != "warn" {
		t.Errorf("enter in search mode should keep searchQuery; got %q", nm.searchQuery)
	}
}

func TestTypingInSearchModeUpdatesQuery(t *testing.T) {
	m := newTestModel(nil)
	m.showLogs = true
	m.showSearch = true
	m.searchInput.Focus()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	nm := next.(Model)

	if nm.searchQuery != "e" {
		t.Errorf("typing in search mode should update searchQuery; got %q", nm.searchQuery)
	}
}

func TestTypingInSearchModeDoesNotScroll(t *testing.T) {
	svc := types.ServiceInfo{Name: "api", Running: true, Healthy: true}
	m := newTestModel([]types.ServiceInfo{svc})
	m.showLogs = true
	m.showSearch = true
	m.searchInput.Focus()
	m.logOffset = 5

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	nm := next.(Model)

	// Scroll handler would decrement logOffset (5→4). Verify it did not fire.
	if nm.logOffset == 4 {
		t.Error("'j' in search mode should not trigger scroll handler")
	}
	// Key should have gone to textinput instead.
	if nm.searchQuery != "j" {
		t.Errorf("'j' in search mode should update searchQuery; got %q", nm.searchQuery)
	}
}

func TestQueryChangeResetsLogOffset(t *testing.T) {
	m := newTestModel(nil)
	m.showLogs = true
	m.showSearch = true
	m.searchInput.Focus()
	m.logOffset = 10

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	nm := next.(Model)

	if nm.logOffset != 0 {
		t.Errorf("changing query should reset logOffset to 0; got %d", nm.logOffset)
	}
}

// --- helpers ---

type logFakeController struct {
	svcs []types.ServiceInfo
	logs []string
}

func (f *logFakeController) StartService(_ string) error        { return nil }
func (f *logFakeController) StopService(_ string) error         { return nil }
func (f *logFakeController) RestartService(_ string) error      { return nil }
func (f *logFakeController) ToggleProxy(_ string) (bool, error) { return false, nil }
func (f *logFakeController) Services() []types.ServiceInfo      { return f.svcs }
func (f *logFakeController) ServiceLogs(_ string) []string      { return f.logs }
func (f *logFakeController) ProjectName() string                { return "test" }
func (f *logFakeController) Subscribe() <-chan types.Event      { return make(chan types.Event) }

func newLogTestModel(svcs []types.ServiceInfo, logs []string) Model {
	ctrl := &logFakeController{svcs: svcs, logs: logs}
	m := newModel(ctrl)
	m.width = 120
	m.height = 40
	return m
}

// --- filter tests ---

func TestFilterLogsReturnsAllWhenNoQuery(t *testing.T) {
	lines := []string{"hello world", "error occurred", "done"}
	got := filterLogs(lines, "")
	if len(got) != 3 {
		t.Errorf("empty query should return all %d lines; got %d", len(lines), len(got))
	}
}

func TestFilterLogsCaseInsensitive(t *testing.T) {
	lines := []string{"INFO starting", "ERROR something failed", "WARN low memory"}
	got := filterLogs(lines, "error")
	if len(got) != 1 || got[0] != "ERROR something failed" {
		t.Errorf("case-insensitive match failed; got %v", got)
	}
}

func TestFilterLogsExcludesNonMatchingLines(t *testing.T) {
	lines := []string{"line one", "line two", "something else"}
	got := filterLogs(lines, "line")
	if len(got) != 2 {
		t.Errorf("expected 2 matching lines; got %d: %v", len(got), got)
	}
}

func TestFilterLogsMatchesSanitizedText(t *testing.T) {
	// Line has ANSI codes that sanitizeLog strips; query matches the visible text.
	lines := []string{"\x1b[31mERROR\x1b[0m something failed"}
	got := filterLogs(lines, "error")
	if len(got) != 1 {
		t.Errorf("filter should match sanitized (ANSI-stripped) text; got %v", got)
	}
}

// --- render tests ---

func TestRenderLogsShowsMatchCountWhenFiltering(t *testing.T) {
	svc := types.ServiceInfo{Name: "api", Running: true, Healthy: true, Port: 3000}
	logs := []string{"request start", "error occurred", "request end"}
	m := newLogTestModel([]types.ServiceInfo{svc}, logs)
	m.selectedIdx = 0
	m.showLogs = true
	m.searchQuery = "error"

	output := m.renderLogs(20)
	stripped := ansi.Strip(output)
	if !strings.Contains(stripped, "1/3 matching") {
		t.Errorf("renderLogs should show match count; got: %q", stripped)
	}
}

func TestRenderLogsNoCountWhenNoQuery(t *testing.T) {
	svc := types.ServiceInfo{Name: "api", Running: true, Healthy: true, Port: 3000}
	logs := []string{"request start", "request end"}
	m := newLogTestModel([]types.ServiceInfo{svc}, logs)
	m.selectedIdx = 0
	m.showLogs = true

	output := m.renderLogs(20)
	stripped := ansi.Strip(output)
	if strings.Contains(stripped, "matching") {
		t.Errorf("renderLogs should not show count when no query; got: %q", stripped)
	}
}

func TestRenderLogsShowsSearchBarWhenSearchActive(t *testing.T) {
	svc := types.ServiceInfo{Name: "api", Running: true, Healthy: true, Port: 3000}
	m := newLogTestModel([]types.ServiceInfo{svc}, []string{"a log line"})
	m.selectedIdx = 0
	m.showLogs = true
	m.showSearch = true
	m.searchInput.SetValue("abc")

	output := m.renderLogs(20)
	stripped := ansi.Strip(output)
	if !strings.Contains(stripped, "abc") {
		t.Errorf("renderLogs should render the search bar with current value; got: %q", stripped)
	}
}

func TestRenderLogsHidesNonMatchingLines(t *testing.T) {
	svc := types.ServiceInfo{Name: "api", Running: true, Healthy: true, Port: 3000}
	logs := []string{"match me", "skip this", "also match me"}
	m := newLogTestModel([]types.ServiceInfo{svc}, logs)
	m.selectedIdx = 0
	m.showLogs = true
	m.searchQuery = "match"

	output := m.renderLogs(30)
	stripped := ansi.Strip(output)
	if strings.Contains(stripped, "skip this") {
		t.Errorf("non-matching line should be hidden; got: %q", stripped)
	}
	if !strings.Contains(stripped, "match me") || !strings.Contains(stripped, "also match me") {
		t.Errorf("matching lines should appear; got: %q", stripped)
	}
}

func TestStatusBarShowsSlashHintInLogView(t *testing.T) {
	m := newTestModel(nil)
	m.width = 120
	m.showLogs = true

	bar := m.renderStatusBar()
	stripped := ansi.Strip(bar)
	if !strings.Contains(stripped, "search") {
		t.Errorf("status bar in log view should show '/' search hint; got: %q", stripped)
	}
}
