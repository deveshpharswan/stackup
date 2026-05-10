package constants_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deveshpharswan/stackup/internal/constants"
)

func TestFindComposeFile_ComposeDotYaml(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("services: {}"), 0644); err != nil {
		t.Fatal(err)
	}
	got := constants.FindComposeFile(dir)
	if filepath.Base(got) != "compose.yaml" {
		t.Errorf("expected compose.yaml, got %q", got)
	}
}

func TestFindComposeFile_DockerComposeDotYml(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}"), 0644); err != nil {
		t.Fatal(err)
	}
	got := constants.FindComposeFile(dir)
	if filepath.Base(got) != "docker-compose.yml" {
		t.Errorf("expected docker-compose.yml, got %q", got)
	}
}

func TestFindComposeFile_PrecedenceComposeDotYamlWins(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("services: {}"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got := constants.FindComposeFile(dir)
	if filepath.Base(got) != "compose.yaml" {
		t.Errorf("expected compose.yaml to win precedence, got %q", got)
	}
}

func TestFindComposeFile_NoneFound(t *testing.T) {
	dir := t.TempDir()
	got := constants.FindComposeFile(dir)
	if got != "" {
		t.Errorf("expected empty string when no compose file found, got %q", got)
	}
}
