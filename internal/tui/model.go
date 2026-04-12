package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/shahin-bayat/lokl/internal/types"
)

// ServiceController defines what the TUI needs to control and display services.
type ServiceController interface {
	StartService(name string) error
	StopService(name string) error
	RestartService(name string) error
	ToggleProxy(name string) (bool, error)
	Services() []types.ServiceInfo
	ServiceLogs(name string) []string
	ProjectName() string
	Subscribe() <-chan types.Event
}

// Model is the TUI state.
type Model struct {
	controller  ServiceController
	events      <-chan types.Event
	services    []types.ServiceInfo
	selectedIdx int
	showLogs    bool
	logOffset   int // 0 = pinned to latest; >0 = scrolled up by N lines
	logHOffset  int // horizontal scroll offset in runes
	showHelp    bool
	width       int
	height      int
	quitting    bool
	copyMsg     string
	copyExpiry  time.Time
	showSearch  bool
	searchQuery string
	searchInput textinput.Model
}

func newModel(ctrl ServiceController) Model {
	ti := textinput.New()
	ti.Placeholder = "search logs..."
	ti.CharLimit = 256
	ti.Width = 76 // updated on WindowSizeMsg

	m := Model{
		controller:  ctrl,
		events:      ctrl.Subscribe(),
		width:       80,
		height:      24,
		searchInput: ti,
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
