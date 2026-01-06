package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const tickInterval = 200 * time.Millisecond

type tickMsg time.Time

func doTick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return doTick()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.refreshServices()
		return m, doTick()
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
	}

	return m, nil
}
