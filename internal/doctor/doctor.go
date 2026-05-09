package doctor

import (
	"context"
	"fmt"
	"io"
	"strings"
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

// PrintFindings formats findings for terminal output.
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

	fmt.Fprintf(w, "Stackup Doctor\n")
	fmt.Fprintf(w, "%s\n\n", strings.Repeat("=", 40))

	for _, f := range findings {
		icon := severityIcon(f.Severity)
		label := ""
		if f.Service != "" {
			label = fmt.Sprintf(" [%s]", f.Service)
		}
		fmt.Fprintf(w, "%s %s%s\n", icon, f.Title, label)
		if f.Detail != "" {
			fmt.Fprintf(w, "  %s\n", f.Detail)
		}
		if f.Fix != "" {
			fmt.Fprintf(w, "  Fix: %s\n", f.Fix)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "%s\n", strings.Repeat("-", 40))
	fmt.Fprintf(w, "Summary: %d error(s), %d warning(s), %d ok\n", errors, warnings, oks)
}

func severityIcon(s Severity) string {
	switch s {
	case SeverityError:
		return "✗" // ✗
	case SeverityWarning:
		return "!"
	case SeverityOK:
		return "✓" // ✓
	default:
		return "?"
	}
}
