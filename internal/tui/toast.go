package tui

import tea "github.com/charmbracelet/bubbletea"

type ToastModel struct{ message string }

func NewToastModel() ToastModel                    { return ToastModel{} }
func (m ToastModel) Show(text string) ToastModel   { m.message = text; return m }
func (m ToastModel) Hide() ToastModel              { m.message = ""; return m }
func (m ToastModel) Message() string               { return m.message }
func (m ToastModel) Tick() tea.Cmd                 { return nil }
