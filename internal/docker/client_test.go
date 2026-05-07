package docker_test

import (
	"testing"

	"github.com/stackup-dev/stackup/internal/docker"
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
	_, err = c.ContainerIDByName("stackup-nonexistent-xyz")
	assert.Error(t, err)
}
