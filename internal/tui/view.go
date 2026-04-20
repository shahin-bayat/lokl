package tui

import (
	"fmt"
	"strings"
	"time"
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

func filterLogs(logs []string, query string) []string {
	if query == "" {
		return logs
	}
	q := strings.ToLower(query)
	out := make([]string, 0, len(logs))
	for _, line := range logs {
		if strings.Contains(strings.ToLower(sanitizeLog(line)), q) {
			out = append(out, line)
		}
	}
	return out
}

func (m Model) renderSearchBar() string {
	label := styleDomain.Render("/")
	return label + " " + m.searchInput.View()
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
	countStyle := styleStopped
	if runningCount > 0 {
		countStyle = styleRunning
	}
	right := fmt.Sprintf("%s %s", stateIndicator(runningCount > 0, true), countStyle.Render(fmt.Sprintf("%d running", runningCount)))

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

	nameWidth := 16
	for _, svc := range m.services {
		if n := len([]rune(svc.Name)); n > nameWidth {
			nameWidth = n
		}
	}

	var b strings.Builder
	for i, svc := range m.services {
		line := m.renderServiceRow(svc, i == m.selectedIdx, nameWidth)
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderServiceRow(svc types.ServiceInfo, selected bool, nameWidth int) string {
	cursor := "  "
	if selected {
		cursor = styleKeyHint.Render("❯ ")
	}

	indicator := stateIndicator(svc.Running, svc.Healthy)
	name := fmt.Sprintf("%-*s", nameWidth, svc.Name)

	var domain string
	if svc.Domain == "" {
		domain = "  " + styleDomain.Render(fmt.Sprintf("%-30s", "-"))
	} else {
		url := fmt.Sprintf("https://%s%s", svc.Domain, svc.PathPrefix)
		paddedURL := fmt.Sprintf("%-30s", url)
		if svc.ProxyEnabled {
			domain = "  " + styleLink.Render(paddedURL)
		} else {
			domain = "  " + styleDomain.Render(paddedURL)
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

	body := fmt.Sprintf("%s %s %s  %s  %s", indicator, name, domain, port, status)

	if svc.Running && !svc.Healthy && !selected {
		return cursor + lipgloss.NewStyle().Background(lipgloss.Color("#3D1E1E")).Render(body)
	}
	return cursor + body
}

func (m Model) renderLogs(available int) string {
	svc := m.selectedService()
	if svc == nil {
		return ""
	}

	logs := m.controller.ServiceLogs(svc.Name)
	filtered := filterLogs(logs, m.searchQuery)

	var prefix string
	if m.searchQuery != "" {
		prefix = fmt.Sprintf("─── Logs: %s (%d/%d matching) ", svc.Name, len(filtered), len(logs))
	} else {
		prefix = fmt.Sprintf("─── Logs: %s ", svc.Name)
	}
	fill := max(0, m.width-lipgloss.Width(prefix))
	headerStr := "\n" + styleDomain.Render(prefix+strings.Repeat("─", fill)) + "\n\n"

	searchBar := ""
	searchLines := 0
	if m.showSearch {
		searchBar = m.renderSearchBar() + "\n"
		searchLines = 1
	}

	if len(logs) == 0 {
		return headerStr + searchBar + styleStopped.Render("  No logs available") + "\n"
	}
	if len(filtered) == 0 {
		return headerStr + searchBar + styleStopped.Render("  No matching lines") + "\n"
	}

	maxLogLines := available - strings.Count(headerStr, "\n") - searchLines
	if maxLogLines < 1 {
		return ""
	}

	// cap offset so scrolling to the top always shows a full screenful
	maxOffset := len(filtered) - maxLogLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := m.logOffset
	if offset > maxOffset {
		offset = maxOffset
	}

	end := len(filtered) - offset
	if end < 0 {
		end = 0
	}
	start := end - maxLogLines
	if start < 0 {
		start = 0
	}

	logWidth := max(0, m.width-2)

	var b strings.Builder
	b.WriteString(headerStr)
	b.WriteString(searchBar)
	for _, line := range filtered[start:end] {
		runes := []rune(sanitizeLog(line))
		hStart := m.logHOffset
		if hStart > len(runes) {
			hStart = len(runes)
		}
		hEnd := hStart + logWidth
		if hEnd > len(runes) {
			hEnd = len(runes)
		}
		if hEnd < hStart {
			hEnd = hStart
		}
		b.WriteString("  ")
		b.WriteString(string(runes[hStart:hEnd]))
		b.WriteString("\n")
	}

	return b.String()
}

func (m Model) renderStatusBar() string {
	divider := styleDomain.Render(strings.Repeat("─", m.width))

	if m.copyMsg != "" && time.Now().Before(m.copyExpiry) {
		return divider + "\n" + styleStatusBar.Render(m.copyMsg)
	}

	var keys []string
	if m.showSearch {
		keys = []string{
			styleKeyHint.Render("enter") + " apply filter",
			styleKeyHint.Render("esc") + " cancel",
		}
	} else if m.showLogs {
		keys = []string{
			styleKeyHint.Render("k/j/↑/↓") + " scroll",
			styleKeyHint.Render("h/l/←/→") + " pan",
			styleKeyHint.Render("/") + " search",
			styleKeyHint.Render("esc") + " close",
			styleKeyHint.Render("c") + " copy",
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
			styleKeyHint.Render("c") + " copy",
			styleKeyHint.Render("?") + " help",
			styleKeyHint.Render("q") + " quit",
		}
	}

	return divider + "\n" + styleStatusBar.Render(strings.Join(keys, "  "))
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
		{"l", "Open log view"},
		{"/", "Search/filter logs"},
		{"c", "Copy logs to clipboard"},
		{"k/j", "Scroll logs up/down (↑/↓)"},
		{"h/l", "Pan logs left/right (←/→)"},
		{"esc", "Close log view / exit search"},
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
