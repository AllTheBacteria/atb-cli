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

func TestAGCDownloadHelpListsFlags(t *testing.T) {
	stdout, _, err := runCmd("agc", "download", "--help")
	if err != nil {
		t.Fatalf("agc download --help: %v", err)
	}
	for _, want := range []string{"--from", "--combine", "--archive-dir", "--dry-run", "--species", "--agc-index"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in --help, got:\n%s", want, stdout)
		}
	}
}

// TestTopLevelFetchGenomesRemoved guards the rename: the command moved under the
// agc group, so the old top-level path must no longer resolve.
func TestTopLevelFetchGenomesRemoved(t *testing.T) {
	_, _, err := runCmd("fetch-genomes", "--help")
	if err == nil {
		t.Fatal("expected top-level 'fetch-genomes' to be gone after the move to 'agc download'")
	}
}

// agcIndexTSV is the 6-column separate AGC index (atb_agc_files.tsv layout): it
// reuses the master-index columns so osf.ParseIndex round-trips it. Tests feed it
// via --agc-index to exercise Mode A without any network I/O.
const agcIndexTSV = "project\tproject_id\tfilename\turl\tmd5\tsize_mb\n" +
	"Acinetobacter_baylyi\tz7q5y\tAcinetobacter_baylyi_global_ordered_0001.agc\thttps://osf.io/download/aaa/\tmd5aaa\t3.890000\n" +
	"Acinetobacter_baylyi\tz7q5y\tAcinetobacter_baylyi_global_ordered_0002.agc\thttps://osf.io/download/bbb/\tmd5bbb\t4.100000\n" +
	"Streptococcus_suis_AA\tz7q5y\tStreptococcus_suis_AA_global_ordered_0001.agc\thttps://osf.io/download/ccc/\tmd5ccc\t10.400000\n"

func writeAGCIndex(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "atb_agc_files.tsv")
	if err := os.WriteFile(path, []byte(agcIndexTSV), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAGCDownloadSpeciesDryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	idxPath := writeAGCIndex(t)

	stdout, stderr, err := runCmd("agc", "download", "--data-dir", dir,
		"--species", "Acinetobacter baylyi", "--agc-index", idxPath, "--dry-run")
	if err != nil {
		t.Fatalf("species dry-run failed: %v\nstderr: %s", err, stderr)
	}
	combined := stdout + stderr
	// Both A. baylyi batches must be listed; the unrelated species must not.
	for _, want := range []string{
		"Acinetobacter_baylyi_global_ordered_0001",
		"Acinetobacter_baylyi_global_ordered_0002",
	} {
		if !strings.Contains(combined, want) {
			t.Errorf("dry-run should list %q, got:\n%s", want, combined)
		}
	}
	if strings.Contains(combined, "Streptococcus") {
		t.Errorf("dry-run must not list batches of other species, got:\n%s", combined)
	}
	if !strings.Contains(strings.ToLower(combined), "dry-run") {
		t.Errorf("dry-run should announce itself, got:\n%s", combined)
	}
	// Nothing may be downloaded in a dry-run.
	if _, err := os.Stat(filepath.Join(dir, sources.AGCArchiveSubdir, "Acinetobacter_baylyi_global_ordered_0001.agc")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not download archives")
	}
	// stdout is reserved for FASTA payload; diagnostics belong on stderr.
	if stdout != "" {
		t.Errorf("dry-run must keep stdout empty, got:\n%q", stdout)
	}
}

func TestAGCDownloadSpeciesNoMatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	idxPath := writeAGCIndex(t)

	_, stderr, err := runCmd("agc", "download", "--data-dir", dir,
		"--species", "Escherichia coli", "--agc-index", idxPath, "--dry-run")
	if err == nil {
		t.Fatal("expected an error when no batch matches the species")
	}
	msg := err.Error() + stderr
	if !strings.Contains(msg, "Escherichia coli") {
		t.Errorf("error should name the species, got: %v / %s", err, stderr)
	}
}

func TestAGCDownloadSpeciesRejectsAccessions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	idxPath := writeAGCIndex(t)

	// --species is its own access mode; mixing it with accessions is ambiguous.
	_, _, err := runCmd("agc", "download", "--data-dir", dir,
		"--species", "Acinetobacter baylyi", "--agc-index", idxPath, "SAMD00000344")
	if err == nil {
		t.Fatal("expected an error when --species is combined with accession arguments")
	}
	if !strings.Contains(err.Error(), "--species") {
		t.Errorf("error should mention --species, got: %v", err)
	}
}

func TestAGCDownloadDryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	// Install a fake agc so FindBinary passes.
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 batch_a\nACC2 batch_a\n")

	stdout, stderr, err := runCmd("agc", "download", "--data-dir", dir, "--dry-run", "ACC1", "ACC2")
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

func TestAGCDownloadUnresolvedWarns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 batch_a\n")

	_, stderr, err := runCmd("agc", "download", "--data-dir", dir, "--dry-run", "ACC1", "GHOST")
	if err == nil {
		t.Fatal("expected non-zero exit when an accession is unresolved")
	}
	if !strings.Contains(stderr, "GHOST") {
		t.Errorf("expected a warning naming GHOST, got:\n%s", stderr)
	}
}

func TestAGCDownloadMissingBinary(t *testing.T) {
	dir := t.TempDir()
	seedArchiveMap(t, dir, "ACC1 batch_a\n")
	// Empty PATH so FindBinary cannot locate agc, and the test binary's own
	// directory has no agc either.
	t.Setenv("PATH", t.TempDir())

	_, _, err := runCmd("agc", "download", "--data-dir", dir, "ACC1")
	if err == nil {
		t.Fatal("expected error when agc is not installed")
	}
	if !strings.Contains(err.Error(), "agc install") {
		t.Errorf("error should point at 'atb agc install', got: %v", err)
	}
}

func TestAGCDownloadDryRunMultipleArchives(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agc stub requires a POSIX shell")
	}
	dir := t.TempDir()
	withFakeAGC(t, "#!/bin/sh\nexit 0\n")
	seedArchiveMap(t, dir, "ACC1 batch_a\nACC2 batch_b\nACC3 batch_a\n")

	stdout, stderr, err := runCmd("agc", "download", "--data-dir", dir, "--dry-run", "ACC1", "ACC2", "ACC3")
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

func TestAGCDownloadDedupesArgsAndFile(t *testing.T) {
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
	_, stderr, err := runCmd("agc", "download", "--data-dir", dir, "--dry-run", "ACC1", "ACC2", "--from", from)
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
