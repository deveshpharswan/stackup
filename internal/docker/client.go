// Package docker wraps the Docker Engine API for container operations.
package docker

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/stdcopy"
	dockerclient "github.com/docker/docker/client"
)

// Client provides high-level container operations on top of the Docker SDK.
type Client struct {
	cli *dockerclient.Client
}

// NewClient connects to the Docker daemon using environment configuration.
func NewClient() (*Client, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}
	return &Client{cli: cli}, nil
}

// Close releases the underlying Docker client connection.
func (c *Client) Close() error { return c.cli.Close() }

// Raw returns the underlying Docker SDK client for advanced operations.
func (c *Client) Raw() *dockerclient.Client { return c.cli }

// ContainerIDByName finds a running container by its compose service label.
func (c *Client) ContainerIDByName(ctx context.Context, serviceName string) (string, error) {
	f := filters.NewArgs(filters.Arg("label", "com.docker.compose.service="+serviceName))
	list, err := c.cli.ContainerList(ctx, container.ListOptions{Filters: f})
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", fmt.Errorf("no container found for service %q", serviceName)
	}
	return list[0].ID, nil
}

// Status returns the health or running state of a container.
func (c *Client) Status(ctx context.Context, containerID string) (string, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}
	if info.State == nil {
		return "unknown", nil
	}
	if info.State.Health != nil {
		return info.State.Health.Status, nil
	}
	return info.State.Status, nil
}

// TailLogs writes the last N lines of container logs to w.
func (c *Client) TailLogs(ctx context.Context, containerID string, lines int, w io.Writer) error {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", lines),
	}
	rc, err := c.cli.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = stdcopy.StdCopy(w, w, rc)
	return err
}

// Logs streams container logs to w, optionally following new output.
func (c *Client) Logs(ctx context.Context, containerID string, follow bool, w io.Writer) error {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
	}
	rc, err := c.cli.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = stdcopy.StdCopy(w, w, rc)
	return err
}

// Restart stops and restarts a container.
func (c *Client) Restart(ctx context.Context, containerID string) error {
	return c.cli.ContainerRestart(ctx, containerID, container.StopOptions{})
}

// ExecShell opens an interactive shell (bash or sh) inside the container.
func (c *Client) ExecShell(ctx context.Context, containerID string, in io.Reader, out io.Writer) error {
	for _, shell := range []string{"bash", "sh"} {
		exec, err := c.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			Tty:          true,
			Cmd:          []string{shell},
		})
		if err != nil {
			continue
		}
		resp, err := c.cli.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{Tty: true})
		if err != nil {
			continue
		}
		defer resp.Close()
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(resp.Conn, in)
		}()
		_, _ = io.Copy(out, resp.Reader)
		wg.Wait()
		return nil
	}
	return fmt.Errorf("no shell found in container %s", containerID)
}

// ValidateServiceName checks that a service name conforms to Docker Compose naming rules.
func ValidateServiceName(name string) error {
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.') {
			return fmt.Errorf("invalid service name %q: contains invalid character %q", name, c)
		}
	}
	return nil
}
