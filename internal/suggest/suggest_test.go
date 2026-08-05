package suggest

import (
	"strings"
	"testing"
)

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
	}

	for _, c := range cases {
		got := levenshtein(c.a, c.b)
		if got != c.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSuggest(t *testing.T) {
	species := []string{
		"Escherichia coli",
		"Staphylococcus aureus",
		"Salmonella enterica",
		"Klebsiella pneumoniae",
		"Pseudomonas aeruginosa",
	}

	cases := []struct {
		input string
		want  string
	}{
		{"Escherichia col", "Escherichia coli"},
		{"staphylococcus aureas", "Staphylococcus aureus"},
		{"salmonela enterica", "Salmonella enterica"},
	}

	for _, c := range cases {
		got := Suggest(c.input, species, 3)
		if len(got) == 0 {
			t.Errorf("Suggest(%q) returned no results", c.input)
			continue
		}
		if got[0] != c.want {
			t.Errorf("Suggest(%q)[0] = %q, want %q", c.input, got[0], c.want)
		}
	}
}

// Issue #25: an out-of-dataset query has no near-spelling among the candidates,
// so the suggester must return nothing rather than offer the nearest unrelated
// name. AllTheBacteria is bacteria-only, so an archaeal name like "Sulfolobus
// acidocaldarius" is absent; it shares only its epithet with the bacterium
// "Alicyclobacillus acidocaldarius" - far too distant to be a real suggestion.
func TestSuggestRejectsOutOfDataset(t *testing.T) {
	species := []string{
		"Escherichia coli",
		"Alicyclobacillus acidocaldarius",
		"Staphylococcus aureus",
	}

	got := Suggest("Sulfolobus acidocaldarius", species, 5)
	if len(got) != 0 {
		t.Errorf("Suggest(%q) = %v, want no suggestions for an out-of-dataset name",
			"Sulfolobus acidocaldarius", got)
	}
}

func TestIdentifiersSuggestsShortNameTypo(t *testing.T) {
	cols := []string{"N50", "N90", "total_length", "Contamination", "sample_accession"}

	got := Identifiers("N5O", cols, 3)

	if len(got) == 0 || got[0] != "N50" {
		t.Errorf("Identifiers(%q) = %v, want %q first", "N5O", got, "N50")
	}
}

func TestIdentifiersIsCaseInsensitive(t *testing.T) {
	got := Identifiers("n50", []string{"N50", "N90"}, 3)

	if len(got) == 0 || got[0] != "N50" {
		t.Errorf("Identifiers(%q) = %v, want %q first", "n50", got, "N50")
	}
}

func TestIdentifiersMatchesLongNameTypo(t *testing.T) {
	got := Identifiers("Contaminaton", []string{"Contamination", "Completeness_General"}, 3)

	if len(got) == 0 || got[0] != "Contamination" {
		t.Errorf("Identifiers(%q) = %v, want %q first", "Contaminaton", got, "Contamination")
	}
}

func TestIdentifiersMatchesGroupPrefix(t *testing.T) {
	got := Identifiers("mlst", []string{"mlst_scheme", "mlst_st", "N50"}, 3)

	if len(got) != 2 {
		t.Fatalf("Identifiers(%q) = %v, want the two mlst columns", "mlst", got)
	}
	for _, name := range got {
		if !strings.HasPrefix(name, "mlst_") {
			t.Errorf("Identifiers(%q) returned %q, which is not an mlst column", "mlst", name)
		}
	}
}

func TestIdentifiersRejectsUnrelatedInput(t *testing.T) {
	cols := []string{"N50", "N90", "total_length", "Contamination", "sample_accession"}

	if got := Identifiers("xyzzy", cols, 3); len(got) != 0 {
		t.Errorf("Identifiers(%q) = %v, want nothing: no column is close to it", "xyzzy", got)
	}
}

func TestIdentifiersLimitsResults(t *testing.T) {
	got := Identifiers("mlst", []string{"mlst_scheme", "mlst_st", "mlst_status", "mlst_score"}, 2)

	if len(got) != 2 {
		t.Errorf("Identifiers with n=2 returned %d results, want 2", len(got))
	}
}
