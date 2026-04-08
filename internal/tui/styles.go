package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary = lipgloss.Color("#7D56F4")
	colorSuccess = lipgloss.Color("#73D216")
	colorError   = lipgloss.Color("#FF4757")
	colorMuted   = lipgloss.Color("#626262")
	colorAmber   = lipgloss.Color("#F4A227")

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	styleRunning  = lipgloss.NewStyle().Foreground(colorSuccess)
	styleStopped  = lipgloss.NewStyle().Foreground(colorMuted)
	styleFailed   = lipgloss.NewStyle().Foreground(colorError)
	styleProxyOff = lipgloss.NewStyle().Foreground(colorAmber)

	styleStatusBar = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleKeyHint = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleDomain = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleLink = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")).
			Bold(true)
)

func stateIndicator(running, healthy bool) string {
	if !running {
		return styleStopped.Render("○")
	}
	if healthy {
		return styleRunning.Render("●")
	}
	return styleFailed.Render("●")
}
