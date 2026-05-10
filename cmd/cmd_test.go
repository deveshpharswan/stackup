package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/deveshpharswan/stackup/cmd"
	"github.com/stretchr/testify/assert"
)

func TestValidateCommand_MissingEnv(t *testing.T) {
	t.Chdir(t.TempDir())

	buf := new(bytes.Buffer)
	root := cmd.NewRootCmd("0.0.0", "test", "now")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"validate"})
	err := root.Execute()
	assert.Error(t, err)
	output := buf.String()
	assert.Contains(t, output, "could not read")
}

func TestValidateCommand_Valid(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Create a .env file with a variable
	os.WriteFile(filepath.Join(dir, ".env"), []byte("FOO=bar\nBAZ=qux\n"), 0644)
	// Create a matching .env.example
	os.WriteFile(filepath.Join(dir, ".env.example"), []byte("FOO=\nBAZ=\n"), 0644)

	buf := new(bytes.Buffer)
	root := cmd.NewRootCmd("0.0.0", "test", "now")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"validate"})
	err := root.Execute()
	assert.NoError(t, err)
}

func TestInitCommand_GeneratesConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Create a minimal docker-compose.yml
	compose := "version: '3'\nservices:\n  redis:\n    image: redis:7\n  api:\n    image: myapp:latest\n"
	os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0644)

	buf := new(bytes.Buffer)
	root := cmd.NewRootCmd("0.0.0", "test", "now")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"init"})
	err := root.Execute()
	assert.NoError(t, err)

	// Verify stackup.yml was created
	_, statErr := os.Stat(filepath.Join(dir, "stackup.yml"))
	assert.NoError(t, statErr)

	// Verify output mentions the file was generated
	assert.Contains(t, buf.String(), "stackup.yml generated")
}

func TestInitCommand_WontOverwrite(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Create a docker-compose.yml so init has something to work with
	compose := "version: '3'\nservices:\n  app:\n    image: node:20\n"
	os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0644)

	// Pre-create stackup.yml to trigger the "already exists" path
	os.WriteFile(filepath.Join(dir, "stackup.yml"), []byte("existing config"), 0644)

	buf := new(bytes.Buffer)
	root := cmd.NewRootCmd("0.0.0", "test", "now")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"init"})
	err := root.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCheckCommand_NoDocker(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Force Docker client to fail by pointing to a non-existent host
	origHost := os.Getenv("DOCKER_HOST")
	os.Setenv("DOCKER_HOST", "tcp://127.0.0.1:1")
	t.Cleanup(func() {
		if origHost == "" {
			os.Unsetenv("DOCKER_HOST")
		} else {
			os.Setenv("DOCKER_HOST", origHost)
		}
	})

	// Create a stackup.yml with a service so the check actually tries something
	cfg := "version: \"1\"\nservices:\n  db:\n    health:\n      type: tcp\n      host: 127.0.0.1\n      port: 59999\n"
	os.WriteFile(filepath.Join(dir, "stackup.yml"), []byte(cfg), 0644)

	buf := new(bytes.Buffer)
	root := cmd.NewRootCmd("0.0.0", "test", "now")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"check"})
	err := root.Execute()
	// The command should error — either Docker connection fails or health check fails
	assert.Error(t, err)
}

func TestDoctorCommand_Runs(t *testing.T) {
	t.Chdir(t.TempDir())

	buf := new(bytes.Buffer)
	root := cmd.NewRootCmd("0.0.0", "test", "now")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"doctor"})

	// doctor should complete without panicking, even with no files present
	assert.NotPanics(t, func() {
		_ = root.Execute()
	})
}

func TestRootCommand_Help(t *testing.T) {
	buf := new(bytes.Buffer)
	root := cmd.NewRootCmd("0.0.0", "test", "now")
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"--help"})
	err := root.Execute()
	assert.NoError(t, err)

	out := buf.String()
	// Verify all subcommands appear in help output
	assert.Contains(t, out, "version")
	assert.Contains(t, out, "up")
	assert.Contains(t, out, "down")
	assert.Contains(t, out, "validate")
	assert.Contains(t, out, "status")
	assert.Contains(t, out, "init")
	assert.Contains(t, out, "logs")
	assert.Contains(t, out, "shell")
	assert.Contains(t, out, "restart")
	assert.Contains(t, out, "run")
	assert.Contains(t, out, "doctor")
	assert.Contains(t, out, "check")
}
