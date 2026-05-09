package tui

import tea "github.com/charmbracelet/bubbletea"

type GraphModel struct{}

func NewGraphModel() GraphModel                                   { return GraphModel{} }
func (m GraphModel) Update(msg tea.Msg) (GraphModel, tea.Cmd)    { return m, nil }
func (m GraphModel) View(width, height int) string                { return "" }
