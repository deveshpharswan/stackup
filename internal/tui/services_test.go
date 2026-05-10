package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestServicesModel_UpdateWithServices(t *testing.T) {
	m := NewServicesModel(nil)
	msg := ServiceUpdateMsg{
		Services: []ServiceInfo{
			{Name: "api", State: "running", Health: "healthy", Ports: "8080/tcp", Tier: 2, Uptime: 5 * time.Minute},
			{Name: "postgres", State: "running", Health: "healthy", Ports: "5432/tcp", Tier: 1, Uptime: 6 * time.Minute},
			{Name: "worker", State: "running", Health: "starting", Tier: 3},
		},
	}
	m, _ = m.Update(msg)
	assert.Equal(t, 3, m.Count())
	assert.Equal(t, "api", m.Selected())
}

func TestServicesModel_Navigation(t *testing.T) {
	m := NewServicesModel(nil)
	msg := ServiceUpdateMsg{
		Services: []ServiceInfo{
			{Name: "api", State: "running", Health: "healthy"},
			{Name: "postgres", State: "running", Health: "healthy"},
		},
	}
	m, _ = m.Update(msg)

	m, _ = m.Update(keyMsg("j"))
	assert.Equal(t, "postgres", m.Selected())

	m, _ = m.Update(keyMsg("k"))
	assert.Equal(t, "api", m.Selected())
}

func TestServicesModel_Filter(t *testing.T) {
	m := NewServicesModel(nil)
	msg := ServiceUpdateMsg{
		Services: []ServiceInfo{
			{Name: "api", State: "running", Health: "healthy"},
			{Name: "postgres", State: "running", Health: "healthy"},
			{Name: "redis", State: "running", Health: "healthy"},
		},
	}
	m, _ = m.Update(msg)
	m = m.SetFilter("post")
	assert.Equal(t, 1, m.Count())
}

func keyMsg(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func TestParseUptime(t *testing.T) {
	cases := []struct {
		input string
		want  time.Duration
	}{
		{"Up About a minute", time.Minute},
		{"Up about a minute", time.Minute},
		{"Up About an hour", time.Hour},
		{"Up about an hour", time.Hour},
		{"Up 37 seconds", 37 * time.Second},
		{"Up 2 minutes", 2 * time.Minute},
		{"Up 3 hours", 3 * time.Hour},
		{"Up 5 days", 5 * 24 * time.Hour},
		{"Exited (1) 2 minutes ago", 0},
		{"", 0},
	}
	for _, tc := range cases {
		got := parseUptime(tc.input)
		if got != tc.want {
			t.Errorf("parseUptime(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
