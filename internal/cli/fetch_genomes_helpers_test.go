package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestReadAccessionsTSVHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "q.tsv")
	os.WriteFile(path, []byte("sample_accession\tspecies\nACC1\tEcoli\nACC2\tKpneu\n"), 0o644)

	got, err := readAccessionsFromFile(path)
	if err != nil {
		t.Fatalf("readAccessionsFromFile: %v", err)
	}
	if !slices.Equal(got, []string{"ACC1", "ACC2"}) {
		t.Errorf("got %v, want [ACC1 ACC2]", got)
	}
}

func TestReadAccessionsCSVHeaderReordered(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "q.csv")
	os.WriteFile(path, []byte("species,sample_accession\nEcoli,ACC1\nKpneu,ACC2\n"), 0o644)

	got, err := readAccessionsFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"ACC1", "ACC2"}) {
		t.Errorf("got %v, want [ACC1 ACC2]", got)
	}
}

func TestReadAccessionsBareLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "list.txt")
	os.WriteFile(path, []byte("ACC1\n# a comment\n\nACC2  trailing junk\n"), 0o644)

	got, err := readAccessionsFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"ACC1", "ACC2"}) {
		t.Errorf("got %v, want [ACC1 ACC2] (first token of each line)", got)
	}
}

func TestDedupeStrings(t *testing.T) {
	got := dedupeStrings([]string{"A", "B", "A", "C", "B"})
	if !slices.Equal(got, []string{"A", "B", "C"}) {
		t.Errorf("got %v, want [A B C]", got)
	}
}

func TestCountSamples(t *testing.T) {
	groups := map[string][]string{"x": {"A", "B"}, "y": {"C"}}
	if got := countSamples(groups); got != 3 {
		t.Errorf("countSamples = %d, want 3", got)
	}
}

func TestReadAccessionsCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "q.tsv")
	// Query results exported on Windows use CRLF line endings; the
	// sample_accession column must still parse cleanly.
	os.WriteFile(path, []byte("sample_accession\tspecies\r\nACC1\tEcoli\r\nACC2\tKpneu\r\n"), 0o644)

	got, err := readAccessionsFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"ACC1", "ACC2"}) {
		t.Errorf("got %v, want [ACC1 ACC2] (CRLF must be stripped)", got)
	}
}
