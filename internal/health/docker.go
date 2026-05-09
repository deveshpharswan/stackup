package health

import (
	"context"
	"fmt"
	"time"

	dockerclient "github.com/docker/docker/client"
)

// DockerChecker polls the Docker API for a container's built-in HEALTHCHECK status.
type DockerChecker struct {
	cli         *dockerclient.Client
	containerID string
	timeout     time.Duration
	interval    time.Duration
}

// NewDockerChecker creates a health checker that inspects the container's health state.
func NewDockerChecker(cli *dockerclient.Client, containerID string, timeout, interval time.Duration) *DockerChecker {
	return &DockerChecker{cli: cli, containerID: containerID, timeout: timeout, interval: interval}
}

// Check polls Docker until the container reports healthy or timeout.
func (c *DockerChecker) Check(ctx context.Context) error {
	err := Poll(ctx, c.timeout, c.interval, func() error {
		info, err := c.cli.ContainerInspect(ctx, c.containerID)
		if err != nil {
			return err
		}
		if info.State != nil && info.State.Health != nil && info.State.Health.Status == "healthy" {
			return nil
		}
		return fmt.Errorf("not healthy")
	})
	if err != nil && err != ctx.Err() {
		return fmt.Errorf("docker health check timed out after %s for container %s", c.timeout, c.containerID)
	}
	return err
}
