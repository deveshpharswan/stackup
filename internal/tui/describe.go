package tui

import tea "github.com/charmbracelet/bubbletea"

type DescribeModel struct{ service string }

func NewDescribeModel() DescribeModel                                    { return DescribeModel{} }
func (m DescribeModel) Update(msg tea.Msg) (DescribeModel, tea.Cmd)     { return m, nil }
func (m DescribeModel) View(width, height int) string                    { return "" }
func (m DescribeModel) ServiceName() string                             { return m.service }
