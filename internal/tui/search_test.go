package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
	m.logOffset = 5

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	nm := next.(Model)

	if nm.logOffset != 5 {
		t.Errorf("'j' in search mode should not scroll; logOffset changed to %d", nm.logOffset)
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
