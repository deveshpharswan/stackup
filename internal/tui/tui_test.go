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

	// q should NOT quit when not on TabServices
	m2 := NewModel(nil)
	m2.width = 100
	m2.height = 30
	m2.activeTab = TabLogs
	newModel2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	model2 := newModel2.(Model)
	assert.False(t, model2.quitting, "q should not quit on non-Services tab")
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
