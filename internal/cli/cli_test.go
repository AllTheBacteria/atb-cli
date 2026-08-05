package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	idx "github.com/allthebacteria/atb-cli/internal/index"
)

const fixtureDir = "../../testdata/fixtures"

func runCmd(args ...string) (string, string, error) {
	cmd := NewRootCmd("test")
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestQuerySpecies(t *testing.T) {
	stdout, _, err := runCmd("query", "--data-dir", fixtureDir, "--species", "Escherichia coli", "--format", "tsv")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if !strings.Contains(stdout, "SAMN00000001") {
		t.Errorf("expected SAMN00000001 in output, got:\n%s", stdout)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 6 {
		t.Errorf("expected 6 lines (header + 5 results), got %d lines:\n%s", len(lines), stdout)
	}
}

func TestQueryHQOnly(t *testing.T) {
	stdout, _, err := runCmd("query", "--data-dir", fixtureDir, "--hq-only", "--format", "tsv")
	if err != nil {
		t.Fatalf("query --hq-only failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	// 1 header + 17 HQ results = 18 lines
	dataRows := len(lines) - 1
	if dataRows != 17 {
		t.Errorf("expected 17 HQ results, got %d", dataRows)
	}
}

func TestQueryWithN50(t *testing.T) {
	stdout, _, err := runCmd("query", "--data-dir", fixtureDir, "--species", "Escherichia coli", "--min-n50", "240000", "--format", "tsv")
	if err != nil {
		t.Fatalf("query with min-n50 failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 4 {
		t.Errorf("expected 4 lines (header + 3 results), got %d lines:\n%s", len(lines), stdout)
	}
}

// TestQuerySampleFileUsesIndex guards against the index path ignoring
// --sample-file: accessions loaded from the file must restrict the SQLite
// query just like --samples does, instead of scanning the whole table.
func TestQuerySampleFileUsesIndex(t *testing.T) {
	dir := fixtureDirWithIndex(t)

	sampleFile := filepath.Join(t.TempDir(), "samples.txt")
	if err := os.WriteFile(sampleFile, []byte("SAMN00000001\nSAMN00000002\n"), 0o644); err != nil {
		t.Fatalf("writing sample file: %v", err)
	}

	stdout, stderr, err := runCmd("query", "--data-dir", dir, "--sample-file", sampleFile, "--format", "tsv")
	if err != nil {
		t.Fatalf("query --sample-file failed: %v\nstderr: %s", err, stderr)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if dataRows := len(lines) - 1; dataRows != 2 {
		t.Errorf("expected 2 rows from --sample-file via the index, got %d:\n%s", dataRows, stdout)
	}
	if !strings.Contains(stdout, "SAMN00000001") || !strings.Contains(stdout, "SAMN00000002") {
		t.Errorf("expected both requested samples in output, got:\n%s", stdout)
	}
}

func TestInfoCommand(t *testing.T) {
	stdout, _, err := runCmd("info", "--data-dir", fixtureDir, "SAMN00000001")
	if err != nil {
		t.Fatalf("info command failed: %v", err)
	}

	if !strings.Contains(stdout, "Escherichia coli") {
		t.Errorf("expected 'Escherichia coli' in output, got:\n%s", stdout)
	}

	if !strings.Contains(stdout, "PASS") {
		t.Errorf("expected 'PASS' in output, got:\n%s", stdout)
	}
}

func TestInfoNotFound(t *testing.T) {
	_, _, err := runCmd("info", "--data-dir", fixtureDir, "NONEXISTENT")
	if err == nil {
		t.Fatal("expected an error for non-existent sample, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error message, got: %v", err)
	}
}

func TestVersionCommand(t *testing.T) {
	stdout, _, err := runCmd("version")
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	if !strings.Contains(stdout, "atb") {
		t.Errorf("expected 'atb' in version output, got:\n%s", stdout)
	}
}

func TestSummariseFromTSV(t *testing.T) {
	stdout, _, err := runCmd("summarise", "--from", "../../testdata/sample_results.tsv")
	if err != nil {
		t.Fatalf("summarise --from tsv failed: %v", err)
	}

	if !strings.Contains(stdout, "Total genomes:") {
		t.Errorf("expected 'Total genomes:' in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "5") {
		t.Errorf("expected count 5 in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Klebsiella pneumoniae") {
		t.Errorf("expected 'Klebsiella pneumoniae' in output, got:\n%s", stdout)
	}
}

func TestSummariseFromCSV(t *testing.T) {
	stdout, _, err := runCmd("summarise", "--from", "../../testdata/sample_results.csv")
	if err != nil {
		t.Fatalf("summarise --from csv failed: %v", err)
	}

	if !strings.Contains(stdout, "Total genomes:") {
		t.Errorf("expected 'Total genomes:' in output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Klebsiella pneumoniae") {
		t.Errorf("expected 'Klebsiella pneumoniae' in output, got:\n%s", stdout)
	}
}

func TestSummariseDefault(t *testing.T) {
	stdout, _, err := runCmd("summarise", "--data-dir", fixtureDir)
	if err != nil {
		t.Fatalf("summarise (default) failed: %v", err)
	}

	if !strings.Contains(stdout, "Total genomes:") {
		t.Errorf("expected 'Total genomes:' in default summary output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Top species:") {
		t.Errorf("expected 'Top species:' in default summary output, got:\n%s", stdout)
	}

	// Fixture has 20 assembly rows but SAMN12 (E. coli) has asm_fasta_on_osf=0,
	// so the default summary must exclude it: 19 total, 4 E. coli (not 5).
	if !regexp.MustCompile(`Total genomes:\s+19\b`).MatchString(stdout) {
		t.Errorf("expected Total genomes of 19 after asm_fasta_on_osf filter, got:\n%s", stdout)
	}
	if !regexp.MustCompile(`Escherichia coli\s+4\b`).MatchString(stdout) {
		t.Errorf("expected Escherichia coli count of 4 after filter, got:\n%s", stdout)
	}
}

func TestSummariseRejectsPositionalArgs(t *testing.T) {
	_, _, err := runCmd("summarise", "--data-dir", fixtureDir, "foo")
	if err == nil {
		t.Fatal("expected error for stray positional argument, got nil")
	}
}

func TestSummariseFromNonExistent(t *testing.T) {
	_, _, err := runCmd("summarise", "--from", "/nonexistent/path/results.csv")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestQueryMissingDatabaseNonInteractive(t *testing.T) {
	// When stdin is not a terminal (test environment), should get error message
	dir := t.TempDir()
	_, stderr, err := runCmd("query", "--data-dir", dir, "--species", "E. coli")
	if err == nil {
		t.Error("expected error for missing database")
	}
	// Should mention "database" or "fetch" in the error
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	combined := stderr + errMsg
	if !strings.Contains(strings.ToLower(combined), "database") && !strings.Contains(strings.ToLower(combined), "fetch") {
		t.Errorf("error should mention database or fetch: %s / %s", errMsg, stderr)
	}
}

func TestConfigShow(t *testing.T) {
	stdout, _, err := runCmd("config", "show")
	if err != nil {
		t.Fatalf("config show failed: %v", err)
	}

	if !strings.Contains(stdout, "data_dir") {
		t.Errorf("expected 'data_dir' in config show output, got:\n%s", stdout)
	}
}

func TestMLSTHelp(t *testing.T) {
	stdout, _, err := runCmd("mlst", "--help")
	if err != nil {
		t.Fatalf("mlst --help failed: %v", err)
	}

	if !strings.Contains(stdout, "MLST") {
		t.Errorf("expected 'MLST' in mlst --help output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--species") {
		t.Errorf("expected '--species' flag in mlst --help output, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--st") {
		t.Errorf("expected '--st' flag in mlst --help output, got:\n%s", stdout)
	}
}

// fixtureDirWithIndex creates a temp directory containing all fixture parquet files
// and a freshly built SQLite index. This is required for commands that use the index
// (e.g. mlst), which do not fall back to parquet scanning.
func fixtureDirWithIndex(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("reading fixture dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(fixtureDir, e.Name()))
		if readErr != nil {
			t.Fatalf("reading fixture %s: %v", e.Name(), readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(dir, e.Name()), data, 0644); writeErr != nil {
			t.Fatalf("writing fixture %s: %v", e.Name(), writeErr)
		}
	}

	if err := idx.Build(dir, func(string, ...any) {}); err != nil {
		t.Fatalf("building index: %v", err)
	}
	return dir
}

func TestAMRDownloadDryRun(t *testing.T) {
	dir := fixtureDirWithIndex(t)

	stdout, stderr, err := runCmd("amr", "--data-dir", dir,
		"--species", "Escherichia coli", "--limit", "5",
		"--download", "--dry-run")
	if err != nil {
		t.Fatalf("amr --download --dry-run failed: %v\nstderr: %s", err, stderr)
	}

	// Normal query output should still appear on stdout. AMR output uses the
	// AMRFinderPlus v4.2.5 column headers verbatim ("Name", "Element symbol", ...).
	if !strings.Contains(stdout, "Element symbol") {
		t.Errorf("expected AMRFP tabular output on stdout, got:\n%s", stdout)
	}

	// Dry-run messages should appear on stderr
	if !strings.Contains(stderr, "Would download") {
		t.Errorf("expected dry-run message on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, ".fa.gz") {
		t.Errorf("expected .fa.gz URLs in dry-run output, got:\n%s", stderr)
	}
}

func TestMLSTDownloadDryRun(t *testing.T) {
	dir := fixtureDirWithIndex(t)

	stdout, stderr, err := runCmd("mlst", "--data-dir", dir,
		"--species", "Escherichia coli", "--limit", "5",
		"--download", "--dry-run")
	if err != nil {
		t.Fatalf("mlst --download --dry-run failed: %v\nstderr: %s", err, stderr)
	}

	if !strings.Contains(stdout, "sample_accession") {
		t.Errorf("expected tabular output on stdout, got:\n%s", stdout)
	}

	if !strings.Contains(stderr, "Would download") {
		t.Errorf("expected dry-run message on stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, ".fa.gz") {
		t.Errorf("expected .fa.gz URLs in dry-run output, got:\n%s", stderr)
	}
}

func TestAMRDownloadMaxSamples(t *testing.T) {
	dir := fixtureDirWithIndex(t)

	_, stderr, err := runCmd("amr", "--data-dir", dir,
		"--species", "Escherichia coli",
		"--download", "--dry-run", "--max-samples", "2")
	if err != nil {
		t.Fatalf("amr --download --max-samples failed: %v\nstderr: %s", err, stderr)
	}

	if !strings.Contains(stderr, "Capping to 2") {
		// Only expect capping message if there are more than 2 unique samples
		// Count .fa.gz lines to verify cap was applied
		count := strings.Count(stderr, ".fa.gz")
		if count > 2 {
			t.Errorf("expected at most 2 URLs in dry-run output, got %d\nstderr: %s", count, stderr)
		}
	}
}

// speciesColumn returns the values of the species column from TSV output.
func speciesColumn(t *testing.T, tsv string) []string {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(tsv), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected a header and at least one data row, got:\n%s", tsv)
	}

	col := -1
	for i, name := range strings.Split(lines[0], "\t") {
		if name == "species" {
			col = i
		}
	}
	if col < 0 {
		t.Fatalf("no species column in header %q", lines[0])
	}

	var values []string
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if col >= len(fields) {
			t.Fatalf("row has no species field: %q", line)
		}
		values = append(values, fields[col])
	}
	return values
}

func TestAMRSpeciesLike(t *testing.T) {
	dir := fixtureDirWithIndex(t)

	stdout, stderr, err := runCmd("amr", "--data-dir", dir,
		"--species-like", "Escherichia co%", "--format", "tsv", "--limit", "20")
	if err != nil {
		t.Fatalf("amr --species-like failed: %v\nstderr: %s", err, stderr)
	}

	// A species pattern is a filter, so it must not trigger the full-scan prompt.
	if strings.Contains(stderr, "--yes") {
		t.Errorf("expected no full-scan confirmation, got stderr:\n%s", stderr)
	}

	for _, species := range speciesColumn(t, stdout) {
		if !strings.HasPrefix(species, "Escherichia co") {
			t.Errorf("species %q does not match pattern %q", species, "Escherichia co%")
		}
	}
}

func TestAMRSpeciesLikeUnderscoreIsLiteral(t *testing.T) {
	dir := fixtureDirWithIndex(t)

	// The fixture species is "Escherichia coli" with a space, so a literal
	// underscore must not stand in for it.
	stdout, stderr, err := runCmd("amr", "--data-dir", dir,
		"--species-like", "Escherichia_coli", "--format", "tsv")
	if err != nil {
		t.Fatalf("amr --species-like failed: %v\nstderr: %s", err, stderr)
	}
	if strings.Contains(stdout, "Escherichia coli") {
		t.Errorf("expected no rows for a literal underscore pattern, got:\n%s", stdout)
	}
}

func TestAMRSpeciesLikeLeadingWildcardNote(t *testing.T) {
	dir := fixtureDirWithIndex(t)

	t.Run("leading wildcard explains the full scan", func(t *testing.T) {
		_, stderr, err := runCmd("amr", "--data-dir", dir,
			"--species-like", "%coli", "--format", "tsv", "--limit", "5")
		if err != nil {
			t.Fatalf("amr --species-like failed: %v\nstderr: %s", err, stderr)
		}
		if !strings.Contains(stderr, "starts with a wildcard") {
			t.Errorf("expected a note about the full scan, got stderr:\n%s", stderr)
		}
	})

	t.Run("anchored pattern is quiet", func(t *testing.T) {
		_, stderr, err := runCmd("amr", "--data-dir", dir,
			"--species-like", "Escherichia%", "--format", "tsv", "--limit", "5")
		if err != nil {
			t.Fatalf("amr --species-like failed: %v\nstderr: %s", err, stderr)
		}
		if strings.Contains(stderr, "starts with a wildcard") {
			t.Errorf("unexpected full-scan note for an anchored pattern, got stderr:\n%s", stderr)
		}
	})
}
