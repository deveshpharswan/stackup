package tui

type HelpModel struct{}

func NewHelpModel() HelpModel                                            { return HelpModel{} }
func (m HelpModel) View(width, height int, active ViewType) string       { return "" }
