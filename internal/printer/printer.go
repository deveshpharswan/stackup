// Package printer provides formatted terminal output for Stackup operations.
package printer

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

// ServiceResult records the outcome of a health check for summary display.
type ServiceResult struct {
	Name    string
	Label   string
	Elapsed time.Duration
	Err     error
}

// Printer writes structured, human-readable output to a writer.
type Printer struct {
	w     io.Writer
	isTTY bool

	green  *color.Color
	red    *color.Color
	yellow *color.Color
	cyan   *color.Color
	dim    *color.Color
	bold   *color.Color
}

// New creates a Printer that writes to w with auto-detected TTY color support.
func New(w io.Writer) *Printer {
	tty := isTTY(w)
	p := &Printer{
		w:      w,
		isTTY:  tty,
		green:  color.New(color.FgGreen),
		red:    color.New(color.FgRed),
		yellow: color.New(color.FgYellow),
		cyan:   color.New(color.FgCyan),
		dim:    color.New(color.FgHiBlack),
		bold:   color.New(color.Bold),
	}
	if !tty {
		p.green.DisableColor()
		p.red.DisableColor()
		p.yellow.DisableColor()
		p.cyan.DisableColor()
		p.dim.DisableColor()
		p.bold.DisableColor()
	}
	return p
}

// Writer returns the underlying io.Writer.
func (p *Printer) Writer() io.Writer { return p.w }

// IsTTY returns whether the output is an interactive terminal.
func (p *Printer) IsTTY() bool { return p.isTTY }

// Phase prints a section header for the current operation.
func (p *Printer) Phase(name string) {
	fmt.Fprintf(p.w, "\n%s %s\n", p.cyan.Sprint("→"), p.bold.Sprint(name))
}

// EnvValid reports that environment validation passed.
func (p *Printer) EnvValid(keyCount int) {
	fmt.Fprintf(p.w, "  %s .env validated (%d keys, 0 missing)\n", p.green.Sprint("✓"), keyCount)
}

// EnvKeyValid reports a single env key passed type validation.
func (p *Printer) EnvKeyValid(key, typ string) {
	fmt.Fprintf(p.w, "  %s %s — valid %s\n", p.green.Sprint("✓"), key, p.dim.Sprint(typ))
}

// ServiceHealthy reports a service passed its health check.
func (p *Printer) ServiceHealthy(name, checkType string, duration time.Duration) {
	fmt.Fprintf(p.w, "  %s %-12s %s  %s  %s\n",
		p.green.Sprint("✓"),
		p.green.Sprint(name),
		p.dim.Sprint("healthy"),
		p.dim.Sprintf("[%s]", checkType),
		p.dim.Sprint(formatDuration(duration)),
	)
}

// ServiceWaiting reports a service is still being checked.
func (p *Printer) ServiceWaiting(name string, elapsed time.Duration) {
	fmt.Fprintf(p.w, "  %s %-12s waiting... %s\n", p.yellow.Sprint("⠋"), name, p.dim.Sprint(formatDuration(elapsed)))
}

// ServiceFailed reports a service health check failure.
func (p *Printer) ServiceFailed(name string, err error) {
	fmt.Fprintf(p.w, "  %s %-12s %s\n", p.red.Sprint("✗"), p.red.Sprint(name), p.red.Sprint(err))
}

// ValidationError reports an env validation failure.
func (p *Printer) ValidationError(key, message string) {
	fmt.Fprintf(p.w, "  %s %s: %s\n", p.red.Sprint("✗"), key, p.red.Sprint(message))
}

// Ready reports the stack is fully healthy with the total startup duration.
func (p *Printer) Ready(total time.Duration) {
	fmt.Fprintf(p.w, "\n%s %s\n", p.green.Sprint("✓"), p.bold.Sprintf("Stack ready in %s", formatDuration(total)))
}

// ServiceLogs prints the tail of a service's container logs.
func (p *Printer) ServiceLogs(name string, logs string) {
	fmt.Fprintf(p.w, "  %s logs: %s %s\n", p.dim.Sprint("┌──"), p.red.Sprint(name), p.dim.Sprint("──"))
	for _, line := range strings.Split(strings.TrimRight(logs, "\n"), "\n") {
		fmt.Fprintf(p.w, "  %s %s\n", p.dim.Sprint("│"), line)
	}
	fmt.Fprintf(p.w, "  %s\n", p.dim.Sprint("└────"))
}

// CleanupSuggestion advises the user to run stackup down.
func (p *Printer) CleanupSuggestion(runningSvcs []string) {
	fmt.Fprintf(p.w, "\n  %s Services still running: %s\n", p.yellow.Sprint("⚠"), strings.Join(runningSvcs, ", "))
	fmt.Fprintf(p.w, "    To clean up: %s\n", p.dim.Sprint("stackup down"))
}

// Hint prints actionable suggestions for the user.
func (p *Printer) Hint(lines ...string) {
	fmt.Fprintf(p.w, "  %s\n", p.dim.Sprint("Try:"))
	for _, l := range lines {
		fmt.Fprintf(p.w, "    %s %s\n", p.dim.Sprint("•"), p.dim.Sprint(l))
	}
}

// EnvDefault reports a schema default was injected for a missing key.
func (p *Printer) EnvDefault(key, val string) {
	fmt.Fprintf(p.w, "  %s %s — using default: %s\n", p.yellow.Sprint("⚙"), key, p.dim.Sprint(val))
}

// ClearScreen sends ANSI escape sequences to move cursor home and clear the screen.
// No-op when output is not a TTY.
func (p *Printer) ClearScreen() {
	if !p.isTTY {
		return
	}
	fmt.Fprint(p.w, "\033[H\033[2J")
}

// Green returns text colored green (or plain if no TTY).
func (p *Printer) Green(s string) string { return p.green.Sprint(s) }

// Red returns text colored red (or plain if no TTY).
func (p *Printer) Red(s string) string { return p.red.Sprint(s) }

// Yellow returns text colored yellow (or plain if no TTY).
func (p *Printer) Yellow(s string) string { return p.yellow.Sprint(s) }

// Dim returns text in dim gray (or plain if no TTY).
func (p *Printer) Dim(s string) string { return p.dim.Sprint(s) }

// Bold returns text in bold (or plain if no TTY).
func (p *Printer) Bold(s string) string { return p.bold.Sprint(s) }

// SummaryTable prints a bordered table of service health check results.
func (p *Printer) SummaryTable(results []ServiceResult, total time.Duration) {
	if len(results) == 0 {
		return
	}

	// Calculate column widths
	nameW, labelW := 7, 5 // min: "Service", "Check"
	for _, r := range results {
		if len(r.Name) > nameW {
			nameW = len(r.Name)
		}
		if len(r.Label) > labelW {
			labelW = len(r.Label)
		}
	}
	nameW += 2
	labelW += 2
	statusW := 11 // "healthy" + padding
	timeW := 8

	totalW := nameW + statusW + labelW + timeW + 5 // 5 = separators

	// Header
	fmt.Fprintf(p.w, "\n  %s%s%s\n", p.dim.Sprint("┌"), p.dim.Sprint(strings.Repeat("─", totalW)), p.dim.Sprint("┐"))

	healthy := 0
	for _, r := range results {
		if r.Err == nil {
			healthy++
		}
	}
	header := fmt.Sprintf("  ✓ Stack ready in %s  (%d/%d services)", formatDuration(total), healthy, len(results))
	pad := totalW + 2 - len([]rune(header))
	if pad < 0 {
		pad = 0
	}
	fmt.Fprintf(p.w, "  %s %s%s%s\n",
		p.dim.Sprint("│"),
		p.green.Sprint(header),
		strings.Repeat(" ", pad),
		p.dim.Sprint("│"),
	)

	// Column headers
	fmt.Fprintf(p.w, "  %s%s%s\n", p.dim.Sprint("├"), p.dim.Sprint(strings.Repeat("─", totalW)), p.dim.Sprint("┤"))
	fmt.Fprintf(p.w, "  %s %-*s%-*s%-*s%-*s%s\n",
		p.dim.Sprint("│"),
		nameW, p.bold.Sprint("Service"),
		statusW, p.bold.Sprint("Status"),
		labelW, p.bold.Sprint("Check"),
		timeW, p.bold.Sprint("Time"),
		p.dim.Sprint("│"),
	)
	fmt.Fprintf(p.w, "  %s%s%s\n", p.dim.Sprint("├"), p.dim.Sprint(strings.Repeat("─", totalW)), p.dim.Sprint("┤"))

	// Rows
	for _, r := range results {
		var status, statusColor string
		if r.Err == nil {
			status = "healthy"
			statusColor = p.green.Sprint(status)
		} else {
			status = "failed"
			statusColor = p.red.Sprint(status)
		}
		_ = status
		fmt.Fprintf(p.w, "  %s %-*s%-*s%-*s%-*s%s\n",
			p.dim.Sprint("│"),
			nameW, r.Name,
			statusW, statusColor,
			labelW, p.dim.Sprint(r.Label),
			timeW, p.dim.Sprint(formatDuration(r.Elapsed)),
			p.dim.Sprint("│"),
		)
	}

	// Footer
	fmt.Fprintf(p.w, "  %s%s%s\n", p.dim.Sprint("└"), p.dim.Sprint(strings.Repeat("─", totalW)), p.dim.Sprint("┘"))
}

// Spinner provides an animated progress indicator for long-running operations.
type Spinner struct {
	w       io.Writer
	frames  []string
	message string
	mu      sync.Mutex
	stop    chan struct{}
	done    chan struct{}
	active  bool
	isTTY   bool
	yellow  *color.Color
}

// NewSpinner creates a spinner that writes to the given writer.
func NewSpinner(w io.Writer, isTTY bool) *Spinner {
	return &Spinner{
		w:      w,
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		isTTY:  isTTY,
		yellow: color.New(color.FgYellow),
	}
}

// Start begins the spinner animation with the given message.
func (s *Spinner) Start(message string) {
	if !s.isTTY {
		fmt.Fprintf(s.w, "  ⠋ %s\n", message)
		return
	}
	s.mu.Lock()
	s.message = message
	s.active = true
	s.mu.Unlock()

	go func() {
		defer close(s.done)
		i := 0
		for {
			select {
			case <-s.stop:
				// Clear the spinner line
				fmt.Fprintf(s.w, "\r%s\r", strings.Repeat(" ", len(s.message)+10))
				return
			default:
				s.mu.Lock()
				msg := s.message
				s.mu.Unlock()
				frame := s.frames[i%len(s.frames)]
				fmt.Fprintf(s.w, "\r  %s %s", s.yellow.Sprint(frame), msg)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

// Stop halts the spinner animation.
func (s *Spinner) Stop() {
	if !s.isTTY || !s.active {
		return
	}
	close(s.stop)
	<-s.done
	s.active = false
}

func isTTY(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
	}
	return false
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
}
