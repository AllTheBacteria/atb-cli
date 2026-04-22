package main

import (
	"fmt"
	"os"

	"github.com/allthebacteria/atb-cli/internal/cli"
)

// version is set via ldflags at build time.
var version = "dev"

func main() {
	cli.RootCmd.Version = version
	err := cli.RootCmd.Execute()

	// Let the background update check finish saving state (up to 2s).
	if cli.WaitForUpdateCheck != nil {
		cli.WaitForUpdateCheck()
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
