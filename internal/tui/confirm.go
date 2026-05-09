package tui

import tea "github.com/charmbracelet/bubbletea"

type ConfirmModel struct {
	action  ConfirmAction
	service string
}

func NewConfirmModel() ConfirmModel { return ConfirmModel{} }
func (m ConfirmModel) Request(action ConfirmAction, service string) ConfirmModel {
	m.action = action
	m.service = service
	return m
}
func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) { return m, nil }
func (m ConfirmModel) View() string                               { return "" }
