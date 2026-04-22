package summarise_test

import (
	"testing"

	"github.com/allthebacteria/atb-cli/internal/query"
	"github.com/allthebacteria/atb-cli/internal/summarise"
)

func makeRows() []query.ResultRow {
	return []query.ResultRow{
		{"sample_accession": "S1", "hq_filter": "PASS", "sylph_species": "Escherichia coli", "dataset": "alpha"},
		{"sample_accession": "S2", "hq_filter": "PASS", "sylph_species": "Escherichia coli", "dataset": "alpha"},
		{"sample_accession": "S3", "hq_filter": "FAIL", "sylph_species": "Salmonella enterica", "dataset": "beta"},
		{"sample_accession": "S4", "hq_filter": "PASS", "sylph_species": "Klebsiella pneumoniae", "dataset": "alpha"},
	}
}

func TestDefaultSummary(t *testing.T) {
	rows := makeRows()
	s := summarise.DefaultSummary(rows)

	if s.Total != 4 {
		t.Errorf("Total: got %d, want 4", s.Total)
	}

	if s.HQCount != 3 {
		t.Errorf("HQCount: got %d, want 3", s.HQCount)
	}

	if len(s.TopSpecies) == 0 {
		t.Error("TopSpecies should not be empty")
	}

	// Escherichia coli should be first (2 occurrences)
	if s.TopSpecies[0].Value != "Escherichia coli" {
		t.Errorf("TopSpecies[0]: got %q, want %q", s.TopSpecies[0].Value, "Escherichia coli")
	}
	if s.TopSpecies[0].Count != 2 {
		t.Errorf("TopSpecies[0].Count: got %d, want 2", s.TopSpecies[0].Count)
	}

	if len(s.Datasets) == 0 {
		t.Error("Datasets should not be empty")
	}

	// alpha should be first (3 occurrences)
	if s.Datasets[0].Value != "alpha" {
		t.Errorf("Datasets[0]: got %q, want %q", s.Datasets[0].Value, "alpha")
	}
	if s.Datasets[0].Count != 3 {
		t.Errorf("Datasets[0].Count: got %d, want 3", s.Datasets[0].Count)
	}
}

func TestGroupBy(t *testing.T) {
	rows := makeRows()
	groups := summarise.GroupBy(rows, "sylph_species")

	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	// First group should have count 2
	if groups[0].Count != 2 {
		t.Errorf("groups[0].Count: got %d, want 2", groups[0].Count)
	}
	if groups[0].Value != "Escherichia coli" {
		t.Errorf("groups[0].Value: got %q, want %q", groups[0].Value, "Escherichia coli")
	}

	// Other two groups should have count 1
	if groups[1].Count != 1 || groups[2].Count != 1 {
		t.Errorf("expected remaining groups to have count 1")
	}
}

func TestGroupByEmpty(t *testing.T) {
	groups := summarise.GroupBy(nil, "sylph_species")
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for nil input, got %d", len(groups))
	}
}

func TestDefaultSummaryFromCSVData(t *testing.T) {
	// Simulate rows that would have been parsed from a CSV/TSV file.
	rows := []query.ResultRow{
		{"sample_accession": "SAMEA001", "hq_filter": "PASS", "sylph_species": "Klebsiella pneumoniae", "dataset": "gamma"},
		{"sample_accession": "SAMEA002", "hq_filter": "PASS", "sylph_species": "Klebsiella pneumoniae", "dataset": "gamma"},
		{"sample_accession": "SAMEA003", "hq_filter": "PASS", "sylph_species": "Klebsiella pneumoniae", "dataset": "delta"},
		{"sample_accession": "SAMEA004", "hq_filter": "FAIL", "sylph_species": "Staphylococcus aureus", "dataset": "gamma"},
		{"sample_accession": "SAMEA005", "hq_filter": "FAIL", "sylph_species": "Staphylococcus aureus", "dataset": "delta"},
	}

	s := summarise.DefaultSummary(rows)

	if s.Total != 5 {
		t.Errorf("Total: got %d, want 5", s.Total)
	}
	if s.HQCount != 3 {
		t.Errorf("HQCount: got %d, want 3", s.HQCount)
	}

	// Klebsiella pneumoniae appears 3 times — should be first
	if len(s.TopSpecies) == 0 {
		t.Fatal("TopSpecies should not be empty")
	}
	if s.TopSpecies[0].Value != "Klebsiella pneumoniae" {
		t.Errorf("TopSpecies[0]: got %q, want %q", s.TopSpecies[0].Value, "Klebsiella pneumoniae")
	}
	if s.TopSpecies[0].Count != 3 {
		t.Errorf("TopSpecies[0].Count: got %d, want 3", s.TopSpecies[0].Count)
	}

	// gamma dataset appears 3 times — should be first
	if len(s.Datasets) == 0 {
		t.Fatal("Datasets should not be empty")
	}
	if s.Datasets[0].Value != "gamma" {
		t.Errorf("Datasets[0]: got %q, want %q", s.Datasets[0].Value, "gamma")
	}
	if s.Datasets[0].Count != 3 {
		t.Errorf("Datasets[0].Count: got %d, want 3", s.Datasets[0].Count)
	}
}

func TestDefaultSummaryEmpty(t *testing.T) {
	s := summarise.DefaultSummary(nil)
	if s.Total != 0 {
		t.Errorf("Total: got %d, want 0", s.Total)
	}
	if s.HQCount != 0 {
		t.Errorf("HQCount: got %d, want 0", s.HQCount)
	}
}
