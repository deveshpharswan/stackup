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
