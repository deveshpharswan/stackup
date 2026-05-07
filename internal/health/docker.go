package health

import (
	"context"
	"fmt"
	"time"

	dockerclient "github.com/docker/docker/client"
)

type DockerChecker struct {
	cli         *dockerclient.Client
	containerID string
	timeout     time.Duration
	interval    time.Duration
}

func NewDockerChecker(cli *dockerclient.Client, containerID string, timeout, interval time.Duration) *DockerChecker {
	return &DockerChecker{cli: cli, containerID: containerID, timeout: timeout, interval: interval}
}

func (c *DockerChecker) Check(ctx context.Context) error {
	deadline := time.Now().Add(c.timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		info, err := c.cli.ContainerInspect(ctx, c.containerID)
		if err == nil && info.State != nil && info.State.Health != nil && info.State.Health.Status == "healthy" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.interval):
		}
	}
	return fmt.Errorf("docker health check timed out after %s for container %s", c.timeout, c.containerID)
}
