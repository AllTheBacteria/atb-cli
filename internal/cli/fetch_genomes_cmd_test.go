package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

// seedArchiveMap writes a cached archive map under <dataDir>/agc so the command
// resolves accessions with no network I/O.
func seedArchiveMap(t *testing.T, dataDir, body string) {
	t.Helper()
	cacheDir := filepath.Join(dataDir, sources.AGCArchiveSubdir)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, sources.AGCArchiveMapFilename), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFetchGenomesHelpListsFlags(t *testing.T) {
	stdout, _, err := runCmd("fetch-genomes", "--help")
	if err != nil {
		t.Fatalf("fetch-genomes --help: %v", err)
	}
	for _, want := range []string{"--from", "--combine", "--archive-dir", "--dry-run"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in --help, got:\n%s", want, stdout)
		}
	}
}

func TestFetchGenomesDryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	// Install a fake agc so FindBinary passes.
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 batch_a\nACC2 batch_a\n")

	stdout, stderr, err := runCmd("fetch-genomes", "--data-dir", dir, "--dry-run", "ACC1", "ACC2")
	if err != nil {
		t.Fatalf("dry-run failed: %v\nstderr: %s", err, stderr)
	}
	combined := stdout + stderr
	if !strings.Contains(combined, "batch_a") {
		t.Errorf("dry-run should name the archive, got:\n%s", combined)
	}
	if !strings.Contains(strings.ToLower(combined), "dry-run") {
		t.Errorf("dry-run should announce itself, got:\n%s", combined)
	}
	// No archive should have been downloaded.
	if _, err := os.Stat(filepath.Join(dir, sources.AGCArchiveSubdir, "batch_a.agc")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not download archives")
	}
}

func TestFetchGenomesUnresolvedWarns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 batch_a\n")

	_, stderr, err := runCmd("fetch-genomes", "--data-dir", dir, "--dry-run", "ACC1", "GHOST")
	if err == nil {
		t.Fatal("expected non-zero exit when an accession is unresolved")
	}
	if !strings.Contains(stderr, "GHOST") {
		t.Errorf("expected a warning naming GHOST, got:\n%s", stderr)
	}
}

func TestFetchGenomesMissingBinary(t *testing.T) {
	dir := t.TempDir()
	seedArchiveMap(t, dir, "ACC1 batch_a\n")
	// Empty PATH so FindBinary cannot locate agc, and the test binary's own
	// directory has no agc either.
	t.Setenv("PATH", t.TempDir())

	_, _, err := runCmd("fetch-genomes", "--data-dir", dir, "ACC1")
	if err == nil {
		t.Fatal("expected error when agc is not installed")
	}
	if !strings.Contains(err.Error(), "agc install") {
		t.Errorf("error should point at 'atb agc install', got: %v", err)
	}
}

func TestFetchGenomesDryRunMultipleArchives(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 batch_a\nACC2 batch_b\nACC3 batch_a\n")

	stdout, stderr, err := runCmd("fetch-genomes", "--data-dir", dir, "--dry-run", "ACC1", "ACC2", "ACC3")
	if err != nil {
		t.Fatalf("dry-run failed: %v\nstderr: %s", err, stderr)
	}
	// Both archives must be listed.
	for _, want := range []string{"batch_a", "batch_b"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("dry-run should list %q, got:\n%s", want, stderr)
		}
	}
	// 3 samples spread across 2 archives.
	if !strings.Contains(stderr, "2 archive(s) for 3 sample(s)") {
		t.Errorf("dry-run summary wrong, got:\n%s", stderr)
	}
	// Diagnostics must never reach stdout, which is reserved for FASTA payload
	// (e.g. --combine piped to another tool).
	if stdout != "" {
		t.Errorf("dry-run must keep stdout empty, got:\n%q", stdout)
	}
}

func TestFetchGenomesDedupesArgsAndFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 batch_a\nACC2 batch_a\n")

	from := filepath.Join(t.TempDir(), "more.txt")
	if err := os.WriteFile(from, []byte("ACC2\nACC3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// args [ACC1 ACC2] + --from [ACC2 ACC3]: ACC2 is duplicated and must
	// collapse to one sample; ACC3 is unresolved and must warn + exit non-zero.
	_, stderr, err := runCmd("fetch-genomes", "--data-dir", dir, "--dry-run", "ACC1", "ACC2", "--from", from)
	if err == nil {
		t.Fatal("expected non-zero exit for unresolved ACC3")
	}
	// batch_a holds the deduped ACC1+ACC2 = 2 samples, not 3.
	if !strings.Contains(stderr, "1 archive(s) for 2 sample(s)") {
		t.Errorf("expected deduped count 2 in batch_a, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "ACC3") {
		t.Errorf("expected ACC3 unresolved warning, got:\n%s", stderr)
	}
}
