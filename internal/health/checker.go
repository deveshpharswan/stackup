// Package health provides health check implementations for Docker services.
package health

import "context"

// Checker polls a service until it is healthy or the timeout is exceeded.
type Checker interface {
	Check(ctx context.Context) error
}

// Named pairs a Checker with a human-readable label for terminal output.
type Named struct {
	Checker Checker
	Label   string
}
