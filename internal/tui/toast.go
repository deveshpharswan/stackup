package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type ToastModel struct {
	message string
	visible bool
}

func NewToastModel() ToastModel { return ToastModel{} }

func (m ToastModel) Show(text string) ToastModel {
	m.message = text
	m.visible = true
	return m
}

func (m ToastModel) Hide() ToastModel {
	m.message = ""
	m.visible = false
	return m
}

func (m ToastModel) Message() string {
	if m.visible {
		return m.message
	}
	return ""
}

func (m ToastModel) Tick() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return ToastExpiredMsg{}
	})
}
