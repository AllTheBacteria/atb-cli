package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/cli"
)

// failingRoot builds a real command tree and an argument list that always
// fails during flag parsing, so the test does not depend on any data files.
func failingRoot() (*cobra.Command, *bytes.Buffer) {
	root := cli.NewRootCmd("test")
	root.SetArgs([]string{"query", "--no-such-flag"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	return root, &out
}

// Cobra reports the error when Execute returns and main reported it a second
// time, so the same sentence reached the user twice.
func TestRunReportsAnErrorOnce(t *testing.T) {
	root, out := failingRoot()

	code := run(root)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1", code)
	}
	if got := strings.Count(out.String(), "unknown flag: --no-such-flag"); got != 1 {
		t.Errorf("error reported %d time(s), want 1:\n%s", got, out.String())
	}
}

// atb writes progress and warnings to stderr as well, so the error keeps the
// conventional prefix that separates it from that output.
func TestRunPrefixesTheError(t *testing.T) {
	root, out := failingRoot()

	run(root)

	line := strings.TrimSpace(out.String())
	if !strings.HasPrefix(line, "Error: ") {
		t.Errorf("error line should start with %q, got:\n%s", "Error: ", line)
	}
}

// Cobra points the user at --help when the command name is not recognised. It
// writes that hint from the same branch as the error line, so suppressing the
// line to stop the duplicate also removed the hint.
func TestRunKeepsTheUnknownCommandHint(t *testing.T) {
	root := cli.NewRootCmd("test")
	root.SetArgs([]string{"frobnicate"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	code := run(root)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1", code)
	}
	if got := strings.Count(out.String(), `unknown command "frobnicate"`); got != 1 {
		t.Errorf("error reported %d time(s), want 1:\n%s", got, out.String())
	}
	if !strings.Contains(out.String(), "Run 'atb --help' for usage.") {
		t.Errorf("unknown command should point at --help, got:\n%s", out.String())
	}
}

func TestRunReportsNothingOnSuccess(t *testing.T) {
	root := cli.NewRootCmd("test")
	root.SetArgs([]string{"--help"})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	if code := run(root); code != 0 {
		t.Errorf("exit code: got %d, want 0", code)
	}
	if strings.Contains(out.String(), "Error:") {
		t.Errorf("--help should not report an error, got:\n%s", out.String())
	}
}
