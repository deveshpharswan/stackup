package tui

import tea "github.com/charmbracelet/bubbletea"

type LogsModel struct{ service string }

func NewLogsModel() LogsModel                               { return LogsModel{} }
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) { return m, nil }
func (m LogsModel) View(width, height int) string            { return "" }
func (m LogsModel) ServiceName() string                     { return m.service }
