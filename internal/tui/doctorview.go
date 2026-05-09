package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/doctor"
)

type DoctorViewModel struct {
	findings []doctor.Finding
	cursor   int
	expanded int
	loading  bool
}

func NewDoctorViewModel() DoctorViewModel {
	return DoctorViewModel{expanded: -1}
}

func (m DoctorViewModel) Init() tea.Cmd {
	return m.runChecks()
}

func (m DoctorViewModel) Update(msg tea.Msg) (DoctorViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case DoctorResultMsg:
		m.findings = msg.Findings
		m.loading = false
		m.cursor = 0
	case DoctorRunningMsg:
		m.loading = true
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.findings)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if m.expanded == m.cursor {
				m.expanded = -1
			} else {
				m.expanded = m.cursor
			}
		case "R":
			m.loading = true
			return m, m.runChecks()
		}
	}
	return m, nil
}

func (m DoctorViewModel) View(width, height int) string {
	if m.loading {
		return styleWarning.Render("  ⠋ Running diagnostics...")
	}
	if len(m.findings) == 0 {
		return styleHealthy.Render("  ✓ No issues found")
	}

	var b strings.Builder

	header := fmt.Sprintf("  %-4s %-18s %-12s %-30s %s",
		"SEV", "CHECK", "SERVICE", "FINDING", "FIX")
	b.WriteString(styleInfo.Bold(true).Render(header) + "\n")
	b.WriteString(styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")

	for i, f := range m.findings {
		if i >= height-4 {
			break
		}
		row := m.renderFinding(f, i == m.cursor, width)
		b.WriteString(row + "\n")
		if i == m.expanded {
			if f.Detail != "" {
				b.WriteString(styleDim.Render("    Detail: "+f.Detail) + "\n")
			}
			if f.Fix != "" {
				b.WriteString(styleInfo.Render("    Fix: "+f.Fix) + "\n")
			}
		}
	}

	errors, warnings, oks := 0, 0, 0
	for _, f := range m.findings {
		switch f.Severity {
		case doctor.SeverityError:
			errors++
		case doctor.SeverityWarning:
			warnings++
		case doctor.SeverityOK:
			oks++
		}
	}
	b.WriteString("\n" + styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")
	b.WriteString(fmt.Sprintf("  %s  %s  %s",
		styleFailed.Render(fmt.Sprintf("%d errors", errors)),
		styleWarning.Render(fmt.Sprintf("%d warnings", warnings)),
		styleHealthy.Render(fmt.Sprintf("%d ok", oks))))

	return b.String()
}

func (m DoctorViewModel) renderFinding(f doctor.Finding, selected bool, width int) string {
	var sevIcon string
	var sevStyle lipgloss.Style
	switch f.Severity {
	case doctor.SeverityError:
		sevIcon = "✗"
		sevStyle = styleFailed
	case doctor.SeverityWarning:
		sevIcon = "!"
		sevStyle = styleWarning
	case doctor.SeverityOK:
		sevIcon = "✓"
		sevStyle = styleHealthy
	}

	svc := f.Service
	if svc == "" {
		svc = "—"
	}
	fix := f.Fix
	if len(fix) > 30 {
		fix = fix[:27] + "..."
	}
	title := f.Title
	if len(title) > 30 {
		title = title[:27] + "..."
	}

	row := fmt.Sprintf("  %-4s %-18s %-12s %-30s %s",
		sevIcon, "", svc, title, fix)

	if selected {
		return styleSelected.Width(width).Render(row)
	}
	return sevStyle.Render(row)
}

func (m DoctorViewModel) runChecks() tea.Cmd {
	return func() tea.Msg {
		d := doctor.New()
		opts := &doctor.Options{
			ComposeFile: constants.DefaultComposeFile,
			EnvFile:     constants.DefaultEnvFile,
			ExampleFile: constants.DefaultExampleFile,
			ConfigFile:  constants.DefaultConfigFile,
		}
		findings := d.Run(context.Background(), opts)
		return DoctorResultMsg{Findings: findings}
	}
}
