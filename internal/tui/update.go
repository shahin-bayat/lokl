package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shahin-bayat/lokl/internal/types"
)

const logPollInterval = 200 * time.Millisecond
const copyFlashDuration = 1500 * time.Millisecond

type eventMsg types.Event
type logTickMsg struct{}
type copyDoneMsg struct{}

func copyFlashTimeout() tea.Cmd {
	return tea.Tick(copyFlashDuration, func(time.Time) tea.Msg {
		return copyDoneMsg{}
	})
}

func (m Model) waitForEvent() tea.Msg {
	return eventMsg(<-m.events)
}

func logTick() tea.Cmd {
	return tea.Tick(logPollInterval, func(time.Time) tea.Msg {
		return logTickMsg{}
	})
}

func (m Model) Init() tea.Cmd {
	return m.waitForEvent
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.searchInput.Width = max(0, msg.Width-4)
		return m, nil

	case eventMsg:
		m.refreshServices()
		return m, m.waitForEvent

	case logTickMsg:
		if m.showLogs {
			return m, logTick()
		}
		return m, nil

	case copyDoneMsg:
		if time.Now().After(m.copyExpiry) {
			m.copyMsg = ""
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Intercept before normal handlers so search input captures j/k/q/etc.
	if m.showSearch {
		switch msg.String() {
		case "esc":
			m.showSearch = false
			m.searchQuery = ""
			m.searchInput.SetValue("")
			m.searchInput.Blur()
			return m, nil
		case "enter":
			m.showSearch = false
			m.searchInput.Blur()
			return m, nil
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
		prev := m.searchQuery
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.searchQuery = m.searchInput.Value()
		if m.searchQuery != prev {
			m.logOffset = 0
		}
		return m, cmd
	}

	if m.showHelp {
		switch msg.String() {
		case "?", "esc", "q":
			m.showHelp = false
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "?":
		m.showHelp = true
		return m, nil

	case "j", "down":
		if m.showLogs {
			if m.logOffset > 0 {
				m.logOffset--
			}
		} else {
			if m.selectedIdx < len(m.services)-1 {
				m.selectedIdx++
				m.logOffset = 0
			}
		}

	case "k", "up":
		if m.showLogs {
			m.logOffset++
		} else {
			if m.selectedIdx > 0 {
				m.selectedIdx--
				m.logOffset = 0
			}
		}

	case "h", "left":
		if m.showLogs && m.logHOffset > 0 {
			m.logHOffset -= 4
			if m.logHOffset < 0 {
				m.logHOffset = 0
			}
		}

	case "l", "right":
		if m.showLogs {
			m.logHOffset += 4
		} else {
			m.showLogs = true
			m.logOffset = 0
			m.logHOffset = 0
			return m, logTick()
		}

	case "s":
		if svc := m.selectedService(); svc != nil {
			if svc.ProxyOnly {
				m.copyMsg = "proxy-only services have no process to start"
				m.copyExpiry = time.Now().Add(copyFlashDuration)
				return m, copyFlashTimeout()
			}
			_ = m.controller.StartService(svc.Name)
		}

	case "x":
		if svc := m.selectedService(); svc != nil {
			if svc.ProxyOnly {
				m.copyMsg = "proxy-only services have no process to stop"
				m.copyExpiry = time.Now().Add(copyFlashDuration)
				return m, copyFlashTimeout()
			}
			_ = m.controller.StopService(svc.Name)
		}

	case "r":
		if svc := m.selectedService(); svc != nil {
			if svc.ProxyOnly {
				m.copyMsg = "proxy-only services have no process to restart"
				m.copyExpiry = time.Now().Add(copyFlashDuration)
				return m, copyFlashTimeout()
			}
			_ = m.controller.RestartService(svc.Name)
		}

	case "p":
		if svc := m.selectedService(); svc != nil && svc.Domain != "" {
			_, _ = m.controller.ToggleProxy(svc.Name)
			m.refreshServices()
		}

	case "c":
		if svc := m.selectedService(); svc != nil {
			logs := filterLogs(m.controller.ServiceLogs(svc.Name), m.searchQuery)
			if err := copyToClipboard(logs); err != nil {
				m.copyMsg = "Copy failed"
			} else {
				m.copyMsg = "Logs copied"
			}
			m.copyExpiry = time.Now().Add(copyFlashDuration)
			return m, copyFlashTimeout()
		}

	case "/":
		if m.showLogs {
			m.showSearch = true
			m.searchInput.Focus()
			return m, nil
		}

	case "esc":
		if m.showLogs {
			m.showLogs = false
			m.logOffset = 0
			m.logHOffset = 0
		}
	}

	return m, nil
}
