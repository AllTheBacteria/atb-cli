package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	idx "github.com/allthebacteria/atb-cli/internal/index"
)

// Issue #25: species queries that hit the SQLite fast path returned 0 results
// with no "Did you mean" hint, even though the suggester ran on the slower
// parquet path. A user typing the NCBI name ("Pseudomonas fluorescens") should
// be pointed at the GTDB name actually stored in the index
// ("Pseudomonas_E fluorescens").
func TestPrintSpeciesSuggestions(t *testing.T) {
	names := []string{
		"Escherichia coli",
		"Salmonella enterica",
		"Klebsiella pneumoniae",
		"Pseudomonas aeruginosa",
		"Pseudomonas_E fluorescens", // GTDB name carries a letter-clade suffix
	}

	var buf bytes.Buffer
	printSpeciesSuggestions(&buf, "Pseudomonas fluorescens", names)
	out := buf.String()

	if !strings.Contains(out, `No results for species "Pseudomonas fluorescens". Did you mean:`) {
		t.Errorf("missing header line, got:\n%s", out)
	}
	if !strings.Contains(out, "Pseudomonas_E fluorescens") {
		t.Fatalf("expected GTDB suggestion, got:\n%s", out)
	}
	// The closest GTDB match must outrank the other Pseudomonas species.
	if strings.Index(out, "Pseudomonas_E fluorescens") > strings.Index(out, "Pseudomonas aeruginosa") {
		t.Errorf("expected 'Pseudomonas_E fluorescens' ranked above 'Pseudomonas aeruginosa', got:\n%s", out)
	}
}

// With no candidate names there is nothing to suggest, so the helper must stay
// silent rather than print an empty "Did you mean" header.
func TestPrintSpeciesSuggestionsNoCandidates(t *testing.T) {
	var buf bytes.Buffer
	printSpeciesSuggestions(&buf, "Pseudomonas fluorescens", nil)
	if buf.Len() != 0 {
		t.Errorf("expected no output for empty candidate list, got:\n%s", buf.String())
	}
}

// End-to-end over the real SQLite index: a misspelled species yields zero rows
// on the fast path, and SpeciesList supplies the candidate names the suggester
// needs to recover the correct spelling. This guards the components the fast
// path wires together (issue #25).
func TestSpeciesListFeedsSuggesterOnEmptyResult(t *testing.T) {
	db := buildTestIndexCLI(t)

	rows, err := db.Query(idx.QueryParams{Species: "Escherichia colli"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows for misspelled species, got %d", len(rows))
	}

	counts, err := db.SpeciesList(0)
	if err != nil {
		t.Fatalf("SpeciesList: %v", err)
	}
	names := make([]string, len(counts))
	for i, c := range counts {
		names[i] = c.Species
	}

	var buf bytes.Buffer
	printSpeciesSuggestions(&buf, "Escherichia colli", names)
	if !strings.Contains(buf.String(), "Escherichia coli") {
		t.Errorf("expected suggestion 'Escherichia coli' for typo 'Escherichia colli', got:\n%s", buf.String())
	}
}

// buildTestIndexCLI builds a fresh SQLite index from the shared parquet
// fixtures in a temp dir and returns an opened handle. It mirrors the index
// package's own test harness so cli-level tests can exercise the fast path.
func buildTestIndexCLI(t *testing.T) *idx.DB {
	t.Helper()

	dir := t.TempDir()
	fixtures := filepath.Join("..", "..", "testdata", "fixtures")
	for _, name := range []string{"assembly.parquet", "assembly_stats.parquet", "checkm2.parquet", "mlst.parquet"} {
		data, err := os.ReadFile(filepath.Join(fixtures, name))
		if err != nil {
			t.Fatalf("reading fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatalf("writing fixture %s: %v", name, err)
		}
	}

	if err := idx.Build(dir, func(string, ...any) {}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	db, err := idx.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
