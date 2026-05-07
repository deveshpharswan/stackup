package printer

import (
	"fmt"
	"io"
	"time"
)

type Printer struct {
	w io.Writer
}

func New(w io.Writer) *Printer {
	return &Printer{w: w}
}

func (p *Printer) Writer() io.Writer { return p.w }

func (p *Printer) Phase(name string) {
	fmt.Fprintf(p.w, "\n→ %s\n", name)
}

func (p *Printer) EnvValid(keyCount int) {
	fmt.Fprintf(p.w, "  ✓ .env validated (%d keys, 0 missing)\n", keyCount)
}

func (p *Printer) EnvKeyValid(key, typ string) {
	fmt.Fprintf(p.w, "  ✓ %s — valid %s\n", key, typ)
}

func (p *Printer) ServiceHealthy(name, checkType string, duration time.Duration) {
	fmt.Fprintf(p.w, "  ✓ %-12s healthy  [%s]  %s\n", name, checkType, formatDuration(duration))
}

func (p *Printer) ServiceFailed(name string, err error) {
	fmt.Fprintf(p.w, "  ✗ %-12s failed: %v\n", name, err)
}

func (p *Printer) ValidationError(key, message string) {
	fmt.Fprintf(p.w, "  ✗ %s: %s\n", key, message)
}

func (p *Printer) Ready(total time.Duration) {
	fmt.Fprintf(p.w, "\n✓ Stack ready in %s\n", formatDuration(total))
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
}
