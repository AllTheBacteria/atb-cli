package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/cli"
)

// version is set via ldflags at build time.
var version = "dev"

func main() {
	cli.RootCmd.Version = version
	os.Exit(run(cli.RootCmd))
}

// run executes root and returns the process exit code. Cobra has already
// written the error to stderr by the time Execute returns.
func run(root *cobra.Command) int {
	err := root.Execute()

	// Let the background update check finish saving state (up to 2s).
	if cli.WaitForUpdateCheck != nil {
		cli.WaitForUpdateCheck()
	}

	if err != nil {
		return 1
	}
	return 0
}
