package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestModel_InitialTab(t *testing.T) {
	m := NewModel(nil)
	assert.Equal(t, TabServices, m.activeTab)
}

func TestModel_TabSwitch(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 30
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	model := newModel.(Model)
	assert.Equal(t, TabDoctor, model.activeTab)
}

func TestModel_EscReturnsToServices(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 30
	m.activeTab = TabLogs
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := newModel.(Model)
	assert.Equal(t, TabServices, model.activeTab)
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
