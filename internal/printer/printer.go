// Package printer provides formatted terminal output for Stackup operations.
package printer

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Printer writes structured, human-readable output to a writer.
type Printer struct {
	w io.Writer
}

// New creates a Printer that writes to w.
func New(w io.Writer) *Printer {
	return &Printer{w: w}
}

// Writer returns the underlying io.Writer.
func (p *Printer) Writer() io.Writer { return p.w }

// Phase prints a section header for the current operation.
func (p *Printer) Phase(name string) {
	fmt.Fprintf(p.w, "\n→ %s\n", name)
}

// EnvValid reports that environment validation passed.
func (p *Printer) EnvValid(keyCount int) {
	fmt.Fprintf(p.w, "  ✓ .env validated (%d keys, 0 missing)\n", keyCount)
}

// EnvKeyValid reports a single env key passed type validation.
func (p *Printer) EnvKeyValid(key, typ string) {
	fmt.Fprintf(p.w, "  ✓ %s — valid %s\n", key, typ)
}

// ServiceHealthy reports a service passed its health check.
func (p *Printer) ServiceHealthy(name, checkType string, duration time.Duration) {
	fmt.Fprintf(p.w, "  ✓ %-12s healthy  [%s]  %s\n", name, checkType, formatDuration(duration))
}

// ServiceWaiting reports a service is still being checked.
func (p *Printer) ServiceWaiting(name string, elapsed time.Duration) {
	fmt.Fprintf(p.w, "  ⠋ %-12s waiting... %s\n", name, formatDuration(elapsed))
}

// ServiceFailed reports a service health check failure.
func (p *Printer) ServiceFailed(name string, err error) {
	fmt.Fprintf(p.w, "  ✗ %-12s failed: %v\n", name, err)
}

// ValidationError reports an env validation failure.
func (p *Printer) ValidationError(key, message string) {
	fmt.Fprintf(p.w, "  ✗ %s: %s\n", key, message)
}

// Ready reports the stack is fully healthy with the total startup duration.
func (p *Printer) Ready(total time.Duration) {
	fmt.Fprintf(p.w, "\n✓ Stack ready in %s\n", formatDuration(total))
}

// ServiceLogs prints the tail of a service's container logs.
func (p *Printer) ServiceLogs(name string, logs string) {
	fmt.Fprintf(p.w, "  ┌── logs: %s ──\n", name)
	for _, line := range strings.Split(strings.TrimRight(logs, "\n"), "\n") {
		fmt.Fprintf(p.w, "  │ %s\n", line)
	}
	fmt.Fprintf(p.w, "  └────\n")
}

// CleanupSuggestion advises the user to run stackup down.
func (p *Printer) CleanupSuggestion(runningSvcs []string) {
	fmt.Fprintf(p.w, "\n  ⚠ Services still running: %s\n", strings.Join(runningSvcs, ", "))
	fmt.Fprintf(p.w, "    To clean up: stackup down\n")
}

// Hint prints actionable suggestions for the user.
func (p *Printer) Hint(lines ...string) {
	fmt.Fprintf(p.w, "  Try:\n")
	for _, l := range lines {
		fmt.Fprintf(p.w, "    • %s\n", l)
	}
}

// EnvDefault reports a schema default was injected for a missing key.
func (p *Printer) EnvDefault(key, val string) {
	fmt.Fprintf(p.w, "  ⚙ %s — using default: %s\n", key, val)
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
}
