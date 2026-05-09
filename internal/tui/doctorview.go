package tui

import tea "github.com/charmbracelet/bubbletea"

type DoctorViewModel struct{}

func NewDoctorViewModel() DoctorViewModel                                      { return DoctorViewModel{} }
func (m DoctorViewModel) Update(msg tea.Msg) (DoctorViewModel, tea.Cmd)       { return m, nil }
func (m DoctorViewModel) View(width, height int) string                        { return "" }
