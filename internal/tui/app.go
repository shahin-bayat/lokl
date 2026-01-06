package tui

import tea "github.com/charmbracelet/bubbletea"

type App struct {
	model Model
}

func New(ctrl ServiceController) *App {
	return &App{
		model: newModel(ctrl),
	}
}

func (a *App) Run() error {
	p := tea.NewProgram(a.model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
