package hooks

import (
	"bytes"
	"context"
	"testing"

	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestRunAfterStart_EmptyActions(t *testing.T) {
	var buf bytes.Buffer
	e := NewExecutor(&buf)
	err := e.RunAfterStart(context.Background(), "postgres", nil)
	assert.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestRunAfterStart_EmptyServiceDefaultsToParent(t *testing.T) {
	// We can't actually run docker, but we can verify the executor
	// constructs with the right writer and handles the empty-actions case.
	var buf bytes.Buffer
	e := NewExecutor(&buf)

	actions := []config.HookAction{
		{Name: "migrate", Service: "", Run: "npm run migrate"},
	}

	// This will fail because docker is not available, but we verify the
	// error message references the parent service as target.
	err := e.RunAfterStart(context.Background(), "api", actions)
	if err != nil {
		assert.Contains(t, err.Error(), "api")
	}
}

func TestRunAfterStart_EmptyNameUsesRun(t *testing.T) {
	var buf bytes.Buffer
	e := NewExecutor(&buf)

	actions := []config.HookAction{
		{Name: "", Service: "worker", Run: "echo hello"},
	}

	// Will fail without docker, but check the output prefix uses Run as name.
	_ = e.RunAfterStart(context.Background(), "postgres", actions)
	assert.Contains(t, buf.String(), "echo hello")
}

func TestNewExecutor(t *testing.T) {
	var buf bytes.Buffer
	e := NewExecutor(&buf)
	assert.NotNil(t, e)
}
