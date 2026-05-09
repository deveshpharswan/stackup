package health

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

// LogChecker watches container logs for a specific pattern string.
// This handles services that don't have reliable TCP/HTTP readiness signals.
type LogChecker struct {
	cli      *dockerclient.Client
	service  string
	pattern  string
	timeout  time.Duration
	interval time.Duration
}

// NewLogChecker creates a LogChecker that polls logs of the given service
// container, searching for pattern until timeout is exceeded.
func NewLogChecker(cli *dockerclient.Client, service, pattern string, timeout, interval time.Duration) *LogChecker {
	return &LogChecker{
		cli:      cli,
		service:  service,
		pattern:  pattern,
		timeout:  timeout,
		interval: interval,
	}
}

// Check polls the container logs until the pattern is found or the timeout expires.
func (c *LogChecker) Check(ctx context.Context) error {
	deadline := time.Now().Add(c.timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		found, err := c.scanLogs(ctx)
		if err == nil && found {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.interval):
		}
	}
	return fmt.Errorf("log pattern %q not found in %s after %s", c.pattern, c.service, c.timeout)
}

func (c *LogChecker) scanLogs(ctx context.Context) (bool, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
	}
	rc, err := c.cli.ContainerLogs(ctx, c.service, opts)
	if err != nil {
		return false, err
	}
	defer rc.Close()
	return ScanForPattern(rc, c.pattern)
}

// ScanForPattern reads lines from r looking for a line containing pattern.
func ScanForPattern(r io.Reader, pattern string) (bool, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), pattern) {
			return true, nil
		}
	}
	return false, scanner.Err()
}
