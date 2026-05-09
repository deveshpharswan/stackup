package tui

import tea "github.com/charmbracelet/bubbletea"

type InputMode int

const (
	ModeCommand InputMode = iota
	ModeFilter
)

type CommandModel struct {
	active bool
	mode   InputMode
	filter string
}

func NewCommandModel() CommandModel                                  { return CommandModel{} }
func (m CommandModel) Active() bool                                  { return m.active }
func (m CommandModel) Filter() string                                { return m.filter }
func (m *CommandModel) Activate(mode InputMode)                      { m.active = true; m.mode = mode }
func (m CommandModel) Update(msg tea.Msg) (CommandModel, tea.Cmd)    { return m, nil }
func (m CommandModel) View(width int) string                         { return "" }
