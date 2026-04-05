package tui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/shahin-bayat/lokl/internal/types"
)

func sanitizeLog(s string) string {
	s = ansi.Strip(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case !unicode.IsControl(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (m Model) View() string {
	if m.quitting {
		return "Shutting down...\n"
	}

	if m.showHelp {
		return m.renderHelp()
	}

	header := m.renderHeader()
	services := m.renderServices()
	statusBar := m.renderStatusBar()

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")
	b.WriteString(services)

	if m.showLogs {
		shell := header + "\n\n" + services + "\n" + statusBar
		available := m.height - lipgloss.Height(shell)
		b.WriteString(m.renderLogs(available))
	}

	b.WriteString("\n")
	b.WriteString(statusBar)

	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, b.String())
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
		domain = "  " + styleDomain.Render(fmt.Sprintf("%-30s", "-"))
	} else {
		url := fmt.Sprintf("https://%s%s", svc.Domain, svc.PathPrefix)
		paddedURL := fmt.Sprintf("%-30s", url)
		if svc.ProxyEnabled {
			domain = "  " + styleLink.Render(paddedURL)
		} else {
			domain = styleFailed.Render("↗") + " " + styleDomain.Render(paddedURL)
		}
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

func (m Model) renderLogs(available int) string {
	svc := m.selectedService()
	if svc == nil {
		return ""
	}

	headerStr := "\n" +
		styleDomain.Render(fmt.Sprintf("─── Logs: %s ", svc.Name)) +
		styleDomain.Render(strings.Repeat("─", 40)) +
		"\n\n"

	logs := m.controller.ServiceLogs(svc.Name)
	if len(logs) == 0 {
		return headerStr + styleStopped.Render("  No logs available") + "\n"
	}

	maxLogLines := available - strings.Count(headerStr, "\n")
	if maxLogLines < 1 {
		return ""
	}

	// cap offset so scrolling to the top always shows a full screenful
	maxOffset := len(logs) - maxLogLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := m.logOffset
	if offset > maxOffset {
		offset = maxOffset
	}

	end := len(logs) - offset
	if end < 0 {
		end = 0
	}
	start := end - maxLogLines
	if start < 0 {
		start = 0
	}

	logWidth := m.width - 2

	var b strings.Builder
	b.WriteString(headerStr)
	for _, line := range logs[start:end] {
		line = sanitizeLog(line)
		b.WriteString("  ")
		b.WriteString(ansi.Truncate(line, logWidth, ""))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderStatusBar() string {
	var keys []string
	if m.showLogs {
		keys = []string{
			styleKeyHint.Render("k") + " scroll up",
			styleKeyHint.Render("j") + " scroll down",
			styleKeyHint.Render("l/esc") + " close logs",
			styleKeyHint.Render("q") + " quit",
		}
	} else {
		keys = []string{
			styleKeyHint.Render("j/k") + " navigate",
			styleKeyHint.Render("s") + " start",
			styleKeyHint.Render("x") + " stop",
			styleKeyHint.Render("r") + " restart",
			styleKeyHint.Render("p") + " toggle",
			styleKeyHint.Render("l") + " logs",
			styleKeyHint.Render("?") + " help",
			styleKeyHint.Render("q") + " quit",
		}
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
