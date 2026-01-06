package tui

import "github.com/shahin-bayat/lokl/internal/types"

// ServiceController defines what the TUI needs to control and display services.
type ServiceController interface {
	StartService(name string) error
	StopService(name string) error
	RestartService(name string) error
	Services() []types.ServiceInfo
	ServiceLogs(name string) []string
	ProjectName() string
}

// Model is the TUI state.
type Model struct {
	controller  ServiceController
	services    []types.ServiceInfo
	selectedIdx int
	showLogs    bool
	width       int
	height      int
	quitting    bool
}

func newModel(ctrl ServiceController) Model {
	m := Model{
		controller: ctrl,
	}
	m.refreshServices()
	return m
}

func (m *Model) refreshServices() {
	m.services = m.controller.Services()
}

func (m Model) selectedService() *types.ServiceInfo {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.services) {
		return &m.services[m.selectedIdx]
	}
	return nil
}
