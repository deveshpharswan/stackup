package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type ConfirmModel struct {
	action  ConfirmAction
	service string
	active  bool
}

func NewConfirmModel() ConfirmModel { return ConfirmModel{} }

func (m ConfirmModel) Request(action ConfirmAction, service string) ConfirmModel {
	m.action = action
	m.service = service
	m.active = true
	return m
}

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.active = false
			return m, func() tea.Msg {
				return ConfirmYesMsg{Action: m.action, Service: m.service}
			}
		case "n", "N", "esc":
			m.active = false
			return m, nil
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	var title, desc string
	switch m.action {
	case ConfirmRestart:
		title = fmt.Sprintf("Restart service %q?", m.service)
		desc = "This will restart the container."
	case ConfirmDelete:
		title = fmt.Sprintf("Stop service %q?", m.service)
		desc = "This will stop the container."
	case ConfirmStackDown:
		title = "Bring down the full stack?"
		desc = "This will stop and remove all containers."
	}

	content := styleBold.Render(title) + "\n\n" +
		styleDim.Render(desc) + "\n\n" +
		styleInfo.Render("[y]") + " Confirm   " + styleDim.Render("[n]") + " Cancel"

	return styleModal.Render(content)
}
