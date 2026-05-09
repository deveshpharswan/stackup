package tui

import tea "github.com/charmbracelet/bubbletea"

type HeaderModel struct{}

func NewHeaderModel() HeaderModel                                { return HeaderModel{} }
func (m HeaderModel) Update(msg tea.Msg) (HeaderModel, tea.Cmd) { return m, nil }
func (m HeaderModel) View(width int, active ViewType) string     { return "" }
