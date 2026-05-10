package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const maxToasts = 3

type toastLevel int

const (
	toastInfo toastLevel = iota
	toastSuccess
	toastWarning
	toastError
)

type toastEntry struct {
	text  string
	level toastLevel
}

type ToastModel struct {
	entries []toastEntry
}

func NewToastModel() ToastModel { return ToastModel{} }

func (m ToastModel) Show(text string) ToastModel {
	return m.ShowLevel(text, toastInfo)
}

func (m ToastModel) ShowLevel(text string, level toastLevel) ToastModel {
	m.entries = append(m.entries, toastEntry{text: text, level: level})
	if len(m.entries) > maxToasts {
		m.entries = m.entries[len(m.entries)-maxToasts:]
	}
	return m
}

func (m ToastModel) HideOldest() ToastModel {
	if len(m.entries) > 0 {
		m.entries = m.entries[1:]
	}
	return m
}

// Message returns the most recent toast text (for status bar compat).
func (m ToastModel) Message() string {
	if len(m.entries) == 0 {
		return ""
	}
	return m.entries[len(m.entries)-1].text
}

func (m ToastModel) Tick() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return ToastExpiredMsg{}
	})
}

// Hide removes the oldest toast (backward compat with existing callers).
func (m ToastModel) Hide() ToastModel {
	return m.HideOldest()
}
