package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var (
	stackupBin string
	hasDocker  bool
)

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "stackup-e2e-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	stackupBin = filepath.Join(tmp, "stackup")
	if runtime.GOOS == "windows" {
		stackupBin += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", stackupBin, "github.com/deveshpharswan/stackup")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("build failed: %s\n%s", err, out))
	}

	hasDocker = exec.Command("docker", "compose", "version").Run() == nil

	os.Exit(m.Run())
}
