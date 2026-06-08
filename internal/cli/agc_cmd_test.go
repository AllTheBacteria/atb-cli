package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// withFakeAGC installs a POSIX-shell `agc` stub on PATH whose body is script,
// and returns. Skips on Windows where the stub cannot run.
func withFakeAGC(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "agc"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake agc: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestAGCHelpListsSubcommands(t *testing.T) {
	stdout, _, err := runCmd("agc", "--help")
	if err != nil {
		t.Fatalf("agc --help failed: %v", err)
	}
	for _, sub := range []string{"install", "ls", "info", "get"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("expected %q in agc --help output, got:\n%s", sub, stdout)
		}
	}
}

func TestAGCGetRequiresSelection(t *testing.T) {
	// No contigs, no --sample, no --all: validation must fail before any exec,
	// so no stub is needed.
	_, _, err := runCmd("agc", "get", "genomes.agc")
	if err == nil {
		t.Fatal("expected error when no selection is given, got nil")
	}
	if !strings.Contains(err.Error(), "--all") && !strings.Contains(err.Error(), "contig") {
		t.Errorf("error should explain the selection options, got: %v", err)
	}
}

func TestAGCGetRejectsAllWithContig(t *testing.T) {
	_, _, err := runCmd("agc", "get", "genomes.agc", "ctg1", "--all")
	if err == nil {
		t.Fatal("expected error combining --all with a contig argument, got nil")
	}
	if !strings.Contains(err.Error(), "--all") {
		t.Errorf("error should mention --all, got: %v", err)
	}
}

func TestAGCGetStreamsToFile(t *testing.T) {
	withFakeAGC(t, "#!/bin/sh\nprintf '>ctg1\\nACGT\\n'\n")
	out := filepath.Join(t.TempDir(), "out.fa")

	_, _, err := runCmd("agc", "get", "genomes.agc", "ctg1", "-o", out)
	if err != nil {
		t.Fatalf("agc get failed: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if !strings.Contains(string(data), ">ctg1") || !strings.Contains(string(data), "ACGT") {
		t.Errorf("output file = %q, want FASTA with >ctg1 / ACGT", data)
	}
}

func TestAGCListSamples(t *testing.T) {
	withFakeAGC(t, "#!/bin/sh\nprintf 'SAMPLE_A\\nSAMPLE_B\\n'\n")

	stdout, _, err := runCmd("agc", "ls", "genomes.agc")
	if err != nil {
		t.Fatalf("agc ls failed: %v", err)
	}
	if !strings.Contains(stdout, "SAMPLE_A") || !strings.Contains(stdout, "SAMPLE_B") {
		t.Errorf("agc ls output = %q, want both sample names", stdout)
	}
}
