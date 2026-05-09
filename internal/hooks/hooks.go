// Package hooks executes lifecycle hook commands inside Docker containers.
package hooks

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/deveshpharswan/stackup/internal/config"
)

// Executor runs lifecycle hooks for services.
type Executor struct {
	w io.Writer
}

// NewExecutor creates a hook executor that writes output to w.
func NewExecutor(w io.Writer) *Executor {
	return &Executor{w: w}
}

// RunAfterStart executes hook actions sequentially.
// Each hook runs `docker compose exec <service> <command>`.
// parentService is the service whose after_start hooks are being run,
// used as default target when a hook omits the Service field.
func (e *Executor) RunAfterStart(ctx context.Context, parentService string, actions []config.HookAction) error {
	for _, action := range actions {
		target := action.Service
		if target == "" {
			target = parentService
		}
		name := action.Name
		if name == "" {
			name = action.Run
		}

		fmt.Fprintf(e.w, "    → hook: %s\n", name)

		parts := strings.Fields(action.Run)
		cmdArgs := append([]string{"compose", "exec", "-T", target}, parts...)
		cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
		cmd.Stdout = e.w
		cmd.Stderr = e.w
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("hook %q failed on %s: %w", name, target, err)
		}
		fmt.Fprintf(e.w, "    ✓ %s\n", name)
	}
	return nil
}
