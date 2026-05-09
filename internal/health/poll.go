package health

import (
	"context"
	"fmt"
	"time"
)

// Poll retries fn until it returns nil or the timeout is exceeded.
// Between attempts it waits for interval, respecting context cancellation.
func Poll(ctx context.Context, timeout, interval time.Duration, fn func() error) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := fn(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	return fmt.Errorf("timed out after %s", timeout)
}
