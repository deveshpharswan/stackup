package main

import (
	"os"

	"github.com/stackup-dev/stackup/cmd"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	root := cmd.NewRootCmd(version, commit, date)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
