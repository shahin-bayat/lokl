package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors - Charm-style palette
	colorPrimary    = lipgloss.Color("#7D56F4")
	colorSuccess    = lipgloss.Color("#73D216")
	colorWarning    = lipgloss.Color("#F4C430")
	colorError      = lipgloss.Color("#FF4757")
	colorMuted      = lipgloss.Color("#626262")
	colorForeground = lipgloss.Color("#EEEEEE")
	colorHighlight  = lipgloss.Color("#3D3D5C")

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary)

	styleRunning  = lipgloss.NewStyle().Foreground(colorSuccess)
	styleStopped  = lipgloss.NewStyle().Foreground(colorMuted)
	styleFailed   = lipgloss.NewStyle().Foreground(colorError)
	styleStarting = lipgloss.NewStyle().Foreground(colorWarning)

	styleSelected = lipgloss.NewStyle().
			Background(colorHighlight).
			Bold(true)

	styleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted)

	styleStatusBar = lipgloss.NewStyle().
			Foreground(colorMuted)

	styleKeyHint = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleDomain = lipgloss.NewStyle().
			Foreground(colorMuted)
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
