package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/shahin-bayat/lokl/internal/types"
)

const logPollInterval = 200 * time.Millisecond

type eventMsg types.Event
type logTickMsg struct{}

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
		return m, nil

	case eventMsg:
		m.refreshServices()
		return m, m.waitForEvent

	case logTickMsg:
		if m.showLogs {
			return m, logTick()
		}
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "j", "down":
		if m.selectedIdx < len(m.services)-1 {
			m.selectedIdx++
		}

	case "k", "up":
		if m.selectedIdx > 0 {
			m.selectedIdx--
		}

	case "s":
		if svc := m.selectedService(); svc != nil {
			_ = m.controller.StartService(svc.Name)
		}

	case "x":
		if svc := m.selectedService(); svc != nil {
			_ = m.controller.StopService(svc.Name)
		}

	case "r":
		if svc := m.selectedService(); svc != nil {
			_ = m.controller.RestartService(svc.Name)
		}

	case "p":
		if svc := m.selectedService(); svc != nil && svc.Domain != "" {
			_, _ = m.controller.ToggleProxy(svc.Name)
			m.refreshServices()
		}

	case "l":
		m.showLogs = !m.showLogs
		if m.showLogs {
			return m, logTick()
		}
	}

	return m, nil
}
