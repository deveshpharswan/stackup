// Package doctor runs automated diagnostics on the development stack.
package doctor

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

// Severity represents the importance level of a diagnostic finding.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityOK
)

// Finding represents a single diagnostic result.
type Finding struct {
	Severity Severity
	Title    string
	Detail   string
	Fix      string
	Service  string
}

// CheckFunc is the signature for individual diagnostic checks.
type CheckFunc func(ctx context.Context, opts *Options) []Finding

// Options holds file paths and configuration for diagnostic checks.
type Options struct {
	ComposeFile string
	EnvFile     string
	ExampleFile string
	ConfigFile  string
}

// Doctor orchestrates diagnostic checks.
type Doctor struct {
	checks []namedCheck
}

type namedCheck struct {
	name string
	fn   CheckFunc
}

// New creates a Doctor with all built-in checks registered.
func New() *Doctor {
	d := &Doctor{}
	d.Register("port-conflicts", CheckPortConflicts)
	d.Register("crash-loops", CheckCrashLoops)
	d.Register("env-drift", CheckEnvDrift)
	d.Register("container-status", CheckContainerStatus)
	d.Register("localhost-misuse", CheckLocalhostMisuse)
	return d
}

// Register adds a named check to the doctor.
func (d *Doctor) Register(name string, fn CheckFunc) {
	d.checks = append(d.checks, namedCheck{name: name, fn: fn})
}

// Run executes all registered checks and collects findings.
func (d *Doctor) Run(ctx context.Context, opts *Options) []Finding {
	var all []Finding
	for _, c := range d.checks {
		findings := c.fn(ctx, opts)
		all = append(all, findings...)
	}
	return all
}

// PrintFindings formats findings for terminal output with color support.
func PrintFindings(w io.Writer, findings []Finding) {
	var errors, warnings, oks int
	for _, f := range findings {
		switch f.Severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		case SeverityOK:
			oks++
		}
	}

	// Detect TTY for color support
	tty := false
	if f, ok := w.(*os.File); ok {
		tty = isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	bold := color.New(color.Bold)
	dim := color.New(color.FgHiBlack)
	if !tty {
		green.DisableColor()
		red.DisableColor()
		yellow.DisableColor()
		bold.DisableColor()
		dim.DisableColor()
	}

	fmt.Fprintf(w, "%s\n", bold.Sprint("Stackup Doctor"))
	fmt.Fprintf(w, "%s\n\n", dim.Sprint(strings.Repeat("─", 40)))

	for _, f := range findings {
		icon := severityIconColor(f.Severity, green, red, yellow)
		label := ""
		if f.Service != "" {
			label = fmt.Sprintf(" %s", dim.Sprintf("[%s]", f.Service))
		}
		fmt.Fprintf(w, "%s %s%s\n", icon, f.Title, label)
		if f.Detail != "" {
			fmt.Fprintf(w, "  %s\n", dim.Sprint(f.Detail))
		}
		if f.Fix != "" {
			fmt.Fprintf(w, "  %s %s\n", dim.Sprint("Fix:"), f.Fix)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "%s\n", dim.Sprint(strings.Repeat("─", 40)))
	summary := fmt.Sprintf("Summary: %s, %s, %s",
		red.Sprintf("%d error(s)", errors),
		yellow.Sprintf("%d warning(s)", warnings),
		green.Sprintf("%d ok", oks),
	)
	fmt.Fprintf(w, "%s\n", summary)
}

func severityIconColor(s Severity, green, red, yellow *color.Color) string {
	switch s {
	case SeverityError:
		return red.Sprint("✗")
	case SeverityWarning:
		return yellow.Sprint("!")
	case SeverityOK:
		return green.Sprint("✓")
	default:
		return "?"
	}
}

func severityIcon(s Severity) string {
	switch s {
	case SeverityError:
		return "✗"
	case SeverityWarning:
		return "!"
	case SeverityOK:
		return "✓"
	default:
		return "?"
	}
}
