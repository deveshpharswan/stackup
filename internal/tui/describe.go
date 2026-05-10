package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/scaffold"
)

type DescribeModel struct {
	service  string
	viewport viewport.Model
	content  string
	ready    bool
}

func NewDescribeModel() DescribeModel {
	return DescribeModel{}
}

func (m DescribeModel) Start(service string, services []ServiceInfo, width, height int) (DescribeModel, tea.Cmd) {
	vp := viewport.New(width, height)
	m.service = service
	m.viewport = vp
	m.ready = true

	cfg, err := config.LoadOrEmpty(constants.DefaultConfigFile)
	if err != nil {
		cfg = &config.Config{}
	}
	m.content = m.buildContent(service, services, cfg)
	m.viewport.SetContent(m.content)
	return m, nil
}

func (m DescribeModel) Update(msg tea.Msg) (DescribeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 7
	}
	return m, nil
}

func (m DescribeModel) View(width, height int) string {
	if !m.ready {
		return styleDim.Render("  No service selected")
	}
	m.viewport.Width = width
	m.viewport.Height = height
	return m.viewport.View()
}

func (m DescribeModel) ServiceName() string { return m.service }

func (m DescribeModel) buildContent(name string, services []ServiceInfo, cfg *config.Config) string {
	var b strings.Builder

	var info ServiceInfo
	for _, s := range services {
		if s.Name == name {
			info = s
			break
		}
	}

	b.WriteString(styleBold.Render("  Service: ") + styleHealthy.Render(name) + "\n")
	b.WriteString(styleBold.Render("  State:   ") + m.stateStr(info) + "\n")
	if info.Ports != "" {
		b.WriteString(styleBold.Render("  Ports:   ") + info.Ports + "\n")
	}
	b.WriteString(styleBold.Render("  Uptime:  ") + formatUptime(info.Uptime) + "\n")
	b.WriteString("\n")

	if svcCfg, ok := cfg.Services[name]; ok && svcCfg.Health != nil {
		hc := svcCfg.Health
		b.WriteString(styleBold.Render("  Health Check:") + "\n")
		b.WriteString(fmt.Sprintf("    Type:     %s\n", hc.Type))
		if hc.URL != "" {
			b.WriteString(fmt.Sprintf("    URL:      %s\n", hc.URL))
		}
		if hc.Host != "" {
			b.WriteString(fmt.Sprintf("    Host:     %s:%d\n", hc.Host, hc.Port))
		}
		if hc.Pattern != "" {
			b.WriteString(fmt.Sprintf("    Pattern:  %s\n", hc.Pattern))
		}
		if hc.Interval != "" {
			b.WriteString(fmt.Sprintf("    Interval: %s\n", hc.Interval))
		}
		if hc.Timeout != "" {
			b.WriteString(fmt.Sprintf("    Timeout:  %s\n", hc.Timeout))
		}
		b.WriteString("\n")

		if svcCfg.Hooks != nil && len(svcCfg.Hooks.AfterStart) > 0 {
			b.WriteString(styleBold.Render("  Hooks:") + "\n")
			for _, h := range svcCfg.Hooks.AfterStart {
				b.WriteString(fmt.Sprintf("    after_start: %s\n", h.Run))
			}
			b.WriteString("\n")
		}
	}

	composePath := constants.FindComposeFile(".")
	if composePath == "" {
		composePath = constants.DefaultComposeFile
	}
	composeSvcs, err := scaffold.ParseServices(composePath)
	if err == nil {
		if deps, ok := composeSvcs[name]; ok && len(deps) > 0 {
			b.WriteString(styleBold.Render("  Depends On:") + "\n")
			for _, dep := range deps {
				b.WriteString(fmt.Sprintf("    %s\n", dep))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m DescribeModel) stateStr(info ServiceInfo) string {
	state := info.State
	if info.Health != "" && info.Health != "(none)" {
		state += " (" + info.Health + ")"
	}
	switch {
	case info.Health == "healthy":
		return styleHealthy.Render(state)
	case info.State == "running":
		return styleWarning.Render(state)
	default:
		return styleFailed.Render(state)
	}
}
