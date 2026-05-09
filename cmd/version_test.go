package cmd_test

import (
	"bytes"
	"testing"

	"github.com/deveshpharswan/stackup/cmd"
	"github.com/stretchr/testify/assert"
)

func TestVersionCommand(t *testing.T) {
	buf := new(bytes.Buffer)
	root := cmd.NewRootCmd("1.2.3", "abc1234", "2026-05-07")
	root.SetOut(buf)
	root.SetArgs([]string{"version"})
	err := root.Execute()
	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "1.2.3")
	assert.Contains(t, out, "abc1234")
	assert.Contains(t, out, "2026-05-07")
}
