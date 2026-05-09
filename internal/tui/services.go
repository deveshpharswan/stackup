package tui

import tea "github.com/charmbracelet/bubbletea"

type ServicesModel struct{ count int }

func NewServicesModel() ServicesModel                                 { return ServicesModel{} }
func (m ServicesModel) Init() tea.Cmd                                 { return nil }
func (m ServicesModel) Update(msg tea.Msg) (ServicesModel, tea.Cmd)   { return m, nil }
func (m ServicesModel) View(width, height int) string                  { return "  No services loaded" }
func (m ServicesModel) Count() int                                    { return m.count }
func (m ServicesModel) Selected() string                              { return "" }
