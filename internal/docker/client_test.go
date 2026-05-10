package docker_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/deveshpharswan/stackup/internal/docker"
	"github.com/stretchr/testify/assert"
)

func TestNewClient_ConnectsToDocker(t *testing.T) {
	c, err := docker.NewClient()
	if err != nil {
		t.Skip("Docker daemon not available:", err)
	}
	defer c.Close()
	assert.NotNil(t, c)
}

func TestClient_ContainerIDByName_NotFound(t *testing.T) {
	c, err := docker.NewClient()
	if err != nil {
		t.Skip("Docker daemon not available:", err)
	}
	defer c.Close()
	_, err = c.ContainerIDByName(context.Background(), "stackup-nonexistent-xyz")
	assert.Error(t, err)
}

func TestClient_TailLogs_InvalidContainer(t *testing.T) {
	c, err := docker.NewClient()
	if err != nil {
		t.Skip("Docker daemon not available:", err)
	}
	defer c.Close()
	var buf bytes.Buffer
	err = c.TailLogs(context.Background(), "nonexistent-container-id", 20, &buf)
	assert.Error(t, err)
}
