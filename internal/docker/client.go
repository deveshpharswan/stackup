package docker

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
)

type Client struct {
	cli *dockerclient.Client
}

func NewClient() (*Client, error) {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error { return c.cli.Close() }

func (c *Client) Raw() *dockerclient.Client { return c.cli }

func (c *Client) ContainerIDByName(serviceName string) (string, error) {
	ctx := context.Background()
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
	_, err = io.Copy(w, rc)
	return err
}

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
	_, err = io.Copy(w, rc)
	return err
}

func (c *Client) Restart(ctx context.Context, containerID string) error {
	return c.cli.ContainerRestart(ctx, containerID, container.StopOptions{})
}

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
			io.Copy(resp.Conn, in)
		}()
		io.Copy(out, resp.Reader)
		wg.Wait()
		return nil
	}
	return fmt.Errorf("no shell found in container %s", containerID)
}
