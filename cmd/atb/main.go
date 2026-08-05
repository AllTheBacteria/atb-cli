package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/cli"
)

// version is set via ldflags at build time.
var version = "dev"

func main() {
	cli.RootCmd.Version = version
	os.Exit(run(cli.RootCmd, os.Stderr))
}

// run executes root and returns the process exit code, reporting any error on
// stderr.
func run(root *cobra.Command, stderr io.Writer) int {
	err := root.Execute()

	// Let the background update check finish saving state (up to 2s).
	if cli.WaitForUpdateCheck != nil {
		cli.WaitForUpdateCheck()
	}

	if err != nil {
		fmt.Fprintln(stderr, "Error:", err)
		return 1
	}
	return 0
}
