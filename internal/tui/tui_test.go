package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestModel_InitialView(t *testing.T) {
	m := NewModel(nil)
	assert.Equal(t, ViewServices, m.activeView)
}

func TestModel_ViewStack(t *testing.T) {
	m := NewModel(nil)
	m = m.pushView(ViewDoctor)
	assert.Equal(t, ViewDoctor, m.activeView)
	assert.Len(t, m.viewStack, 2)

	m = m.popView()
	assert.Equal(t, ViewServices, m.activeView)
	assert.Len(t, m.viewStack, 1)
}

func TestModel_PopViewAtRoot(t *testing.T) {
	m := NewModel(nil)
	m = m.popView()
	assert.Equal(t, ViewServices, m.activeView)
	assert.Len(t, m.viewStack, 1)
}

func TestModel_TerminalTooSmall(t *testing.T) {
	m := NewModel(nil)
	m.width = 40
	m.height = 10
	view := m.View()
	assert.Contains(t, view, "Terminal too small")
}

func TestModel_QuitOnQ(t *testing.T) {
	m := NewModel(nil)
	m.width = 100
	m.height = 30
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	model := newModel.(Model)
	assert.True(t, model.quitting)
	assert.NotNil(t, cmd)
}

func TestModel_HelpToggle(t *testing.T) {
	m := NewModel(nil)
	m.width = 100
	m.height = 30
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	model := newModel.(Model)
	assert.True(t, model.showHelp)

	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	model = newModel.(Model)
	assert.False(t, model.showHelp)
}

func TestModel_IsWide(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	assert.False(t, m.isWide())
	m.width = 100
	assert.True(t, m.isWide())
	m.width = 120
	assert.True(t, m.isWide())
}

func TestModel_TwoPanelRenderNoPanic(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 30
	// Should not panic with no services loaded
	view := m.View()
	assert.NotEmpty(t, view)
	assert.NotContains(t, view, "Terminal too small")
}

func TestModel_NarrowFallback(t *testing.T) {
	m := NewModel(nil)
	m.width = 90
	m.height = 30
	// Narrow mode should render without panic
	view := m.View()
	assert.NotEmpty(t, view)
}

func TestModel_SidebarNavInWideMode(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 30
	// Load some services into sidebar
	m.sidebar = m.sidebar.SetServices([]ServiceInfo{
		{Name: "db", State: "running", Health: "healthy"},
		{Name: "api", State: "running", Health: "healthy"},
	})
	assert.Equal(t, "db", m.sidebar.Selected())

	// Press j to move cursor down
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	model := newModel.(Model)
	assert.Equal(t, "api", model.sidebar.Selected())
}

func TestModel_SelectedServiceNarrow(t *testing.T) {
	m := NewModel(nil)
	m.width = 80
	m.height = 30
	// In narrow mode, selectedService uses services model
	assert.Equal(t, "", m.selectedService())
}

func TestModel_SelectedServiceWide(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 30
	m.sidebar = m.sidebar.SetServices([]ServiceInfo{
		{Name: "web", State: "running", Health: "healthy"},
	})
	assert.Equal(t, "web", m.selectedService())
}

func TestDetailModel_NilService(t *testing.T) {
	d := NewDetailModel()
	view := d.View(80, 30, "")
	assert.Contains(t, view, "Select a service")
}

func TestDetailModel_WithService(t *testing.T) {
	d := NewDetailModel()
	svc := ServiceInfo{Name: "web", State: "running", Health: "healthy"}
	d = d.SetService(&svc, nil)
	view := d.View(80, 30, "")
	assert.Contains(t, view, "web")
	assert.Contains(t, view, "Logs")
}

func TestDetailModel_WithStats(t *testing.T) {
	d := NewDetailModel()
	svc := ServiceInfo{Name: "api", State: "running", Health: "healthy", CPU: 12.5, Memory: 40.0}
	history := map[string]*StatsHistory{
		"api": {cpu: []float64{10, 11, 12}, mem: []float64{38, 39, 40}},
	}
	d = d.SetService(&svc, history)
	view := d.View(80, 30, "some log line")
	assert.Contains(t, view, "api")
	assert.Contains(t, view, "some log line")
}

func TestDetailModel_WithLogTail(t *testing.T) {
	d := NewDetailModel()
	svc := ServiceInfo{Name: "db", State: "running"}
	d = d.SetService(&svc, nil)
	view := d.View(80, 30, "line1\nline2\nline3")
	assert.Contains(t, view, "line1")
}

func TestComputeLayout_Narrow(t *testing.T) {
	l := ComputeLayout(80, 24)
	assert.False(t, l.HasSidebar)
	assert.Equal(t, 80, l.DetailWidth)
	assert.Equal(t, 22, l.ContentHeight)
}

func TestComputeLayout_Wide(t *testing.T) {
	l := ComputeLayout(120, 30)
	assert.True(t, l.HasSidebar)
	assert.Equal(t, 24, l.SidebarWidth)
	assert.Equal(t, 96, l.DetailWidth)
	assert.Equal(t, 28, l.ContentHeight)
}
