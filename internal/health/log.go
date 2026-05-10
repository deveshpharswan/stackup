package health

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// LogChecker watches container logs for a specific pattern string.
type LogChecker struct {
	cli      *dockerclient.Client
	service  string
	pattern  string
	timeout  time.Duration
	interval time.Duration
}

// NewLogChecker creates a health checker that scans container stdout/stderr for pattern.
func NewLogChecker(cli *dockerclient.Client, service, pattern string, timeout, interval time.Duration) *LogChecker {
	return &LogChecker{
		cli:      cli,
		service:  service,
		pattern:  pattern,
		timeout:  timeout,
		interval: interval,
	}
}

func (c *LogChecker) resolveContainerID(ctx context.Context) (string, error) {
	f := filters.NewArgs(filters.Arg("label", "com.docker.compose.service="+c.service))
	list, err := c.cli.ContainerList(ctx, container.ListOptions{Filters: f})
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no running container for service %q", c.service)
	}
	return list[0].ID, nil
}

func (c *LogChecker) Check(ctx context.Context) error {
	err := Poll(ctx, c.timeout, c.interval, func() error {
		found, err := c.scanLogs(ctx)
		if err != nil {
			return err
		}
		if found {
			return nil
		}
		return fmt.Errorf("pattern not found")
	})
	if err != nil && err != ctx.Err() {
		return fmt.Errorf("log pattern %q not found in %s after %s", c.pattern, c.service, c.timeout)
	}
	return err
}

func (c *LogChecker) scanLogs(ctx context.Context) (bool, error) {
	id, err := c.resolveContainerID(ctx)
	if err != nil {
		return false, err
	}
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
	}
	rc, err := c.cli.ContainerLogs(ctx, id, opts)
	if err != nil {
		return false, err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, rc); err != nil {
		return false, err
	}
	return ScanForPattern(&buf, c.pattern)
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
