package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/shahin-bayat/lokl/internal/types"
)

func (m Model) View() string {
	if m.quitting {
		return "Shutting down...\n"
	}

	if m.showHelp {
		return m.renderHelp()
	}

	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")
	b.WriteString(m.renderServices())

	if m.showLogs {
		b.WriteString(m.renderLogs())
	}

	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())

	return b.String()
}

func (m Model) renderHeader() string {
	name := "lokl"
	if pn := m.controller.ProjectName(); pn != "" {
		name = fmt.Sprintf("lokl - %s", pn)
	}

	runningCount := 0
	for _, svc := range m.services {
		if svc.Running {
			runningCount++
		}
	}

	left := styleHeader.Render(name)
	right := fmt.Sprintf("%s %d running", stateIndicator(runningCount > 0, true), runningCount)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + right
}

func (m Model) renderServices() string {
	if len(m.services) == 0 {
		return styleStopped.Render("  No services configured")
	}

	var b strings.Builder

	for i, svc := range m.services {
		line := m.renderServiceRow(svc, i == m.selectedIdx)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderServiceRow(svc types.ServiceInfo, selected bool) string {
	cursor := "  "
	if selected {
		cursor = styleKeyHint.Render("▸ ")
	}

	indicator := stateIndicator(svc.Running, svc.Healthy)
	name := fmt.Sprintf("%-16s", svc.Name)

	var domain string
	if svc.Domain == "" {
		domain = styleDomain.Render(fmt.Sprintf("%-36s", "-"))
	} else if svc.ProxyEnabled {
		url := fmt.Sprintf("https://%s", svc.Domain)
		domain = "  " + styleLink.Render(url)
	} else {
		url := fmt.Sprintf("https://%s", svc.Domain)
		domain = styleFailed.Render("↗") + " " + styleDomain.Render(url)
	}

	port := fmt.Sprintf(":%d", svc.Port)

	status := "stopped"
	statusStyle := styleStopped
	if svc.Running {
		if svc.Healthy {
			status = "healthy"
			statusStyle = styleRunning
		} else {
			status = "unhealthy"
			statusStyle = styleFailed
		}
	}
	status = statusStyle.Render(status)

	row := fmt.Sprintf("%s%s %s %s  %s  %s", cursor, indicator, name, domain, port, status)

	if selected {
		row = styleSelected.Render(row)
	}

	return row
}

func (m Model) renderLogs() string {
	svc := m.selectedService()
	if svc == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleDomain.Render(fmt.Sprintf("─── Logs: %s ", svc.Name)))
	b.WriteString(styleDomain.Render(strings.Repeat("─", 40)))
	b.WriteString("\n\n")

	logs := m.controller.ServiceLogs(svc.Name)
	if len(logs) == 0 {
		b.WriteString(styleStopped.Render("  No logs available"))
		b.WriteString("\n")
		return b.String()
	}

	maxLogLines := m.height / 2
	if maxLogLines < 3 {
		maxLogLines = 3
	}

	start := 0
	if len(logs) > maxLogLines {
		start = len(logs) - maxLogLines
	}
	for _, line := range logs[start:] {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderStatusBar() string {
	keys := []string{
		styleKeyHint.Render("j/k") + " navigate",
		styleKeyHint.Render("s") + " start",
		styleKeyHint.Render("x") + " stop",
		styleKeyHint.Render("r") + " restart",
		styleKeyHint.Render("p") + " toggle",
		styleKeyHint.Render("l") + " logs",
		styleKeyHint.Render("?") + " help",
		styleKeyHint.Render("q") + " quit",
	}

	return styleStatusBar.Render(strings.Join(keys, "  "))
}

func (m Model) renderHelp() string {
	title := styleHeader.Render("Keyboard Shortcuts")

	bindings := []struct {
		key  string
		desc string
	}{
		{"j / ↓", "Move selection down"},
		{"k / ↑", "Move selection up"},
		{"s", "Start selected service"},
		{"x", "Stop selected service"},
		{"r", "Restart selected service"},
		{"p", "Toggle proxy (local/remote)"},
		{"l", "Toggle log view"},
		{"?", "Show/hide this help"},
		{"q", "Quit lokl"},
	}

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")

	for _, bind := range bindings {
		key := styleKeyHint.Render(fmt.Sprintf("%-8s", bind.key))
		b.WriteString(fmt.Sprintf("  %s %s\n", key, bind.desc))
	}

	b.WriteString("\n")
	b.WriteString(styleDomain.Render("Press ? or esc to close"))

	content := b.String()

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2)

	box := boxStyle.Render(content)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box)
}
