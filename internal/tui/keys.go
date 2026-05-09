package tui

import "github.com/charmbracelet/bubbles/key"

type ServicesKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Restart key.Binding
	Shell   key.Binding
	Logs    key.Binding
	Delete  key.Binding
	Desc    key.Binding
	Doctor  key.Binding
	Graph   key.Binding
	Filter  key.Binding
	Command key.Binding
	Help    key.Binding
	Quit    key.Binding
}

var ServicesKeys = ServicesKeyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Restart: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")),
	Shell:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "shell")),
	Logs:    key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "logs")),
	Delete:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop")),
	Desc:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "describe")),
	Doctor:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "doctor")),
	Graph:   key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "graph")),
	Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Command: key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:    key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
}

type LogsKeyMap struct {
	Back       key.Binding
	Timestamps key.Binding
	Wrap       key.Binding
	Search     key.Binding
	Clear      key.Binding
	Top        key.Binding
	Bottom     key.Binding
}

var LogsKeys = LogsKeyMap{
	Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Timestamps: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "timestamps")),
	Wrap:       key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "wrap")),
	Search:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Clear:      key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clear")),
	Top:        key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
	Bottom:     key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
}

type DoctorKeyMap struct {
	Back   key.Binding
	Expand key.Binding
	Rescan key.Binding
}

var DoctorKeys = DoctorKeyMap{
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Expand: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
	Rescan: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "re-scan")),
}

type GraphKeyMap struct {
	Back   key.Binding
	Select key.Binding
}

var GraphKeys = GraphKeyMap{
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
}
