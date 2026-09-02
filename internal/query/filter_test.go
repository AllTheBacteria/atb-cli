package query

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFilterTOML(t *testing.T) {
	content := `
[filter]
species = "Salmonella enterica"
species_like = "Salmonella%"
genus = "Salmonella"
samples = ["SAMEA1", "SAMEA2"]
sample_file = "/tmp/samples.txt"
runs = ["ERR1", "SRR2"]
run_file = "/tmp/runs.txt"
hq_only = true
min_completeness = 95.5
max_contamination = 2.0
min_n50 = 50000
dataset = "v2"
has_assembly = true
country = "United Kingdom"
platform = "Illumina"
collection_date_from = "2020-01-01"
collection_date_to = "2023-12-31"

[output]
columns = ["sample_id", "species"]
sort_by = "species"
sort_desc = true
limit = 100
offset = 10
format = "tsv"
output = "results.tsv"
`

	dir := t.TempDir()
	path := filepath.Join(dir, "filter.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	ff, err := LoadFilterFile(path)
	if err != nil {
		t.Fatalf("LoadFilterFile returned error: %v", err)
	}

	f := ff.Filter
	if f.Species != "Salmonella enterica" {
		t.Errorf("Species: got %q, want %q", f.Species, "Salmonella enterica")
	}
	if f.SpeciesLike != "Salmonella%" {
		t.Errorf("SpeciesLike: got %q, want %q", f.SpeciesLike, "Salmonella%")
	}
	if f.Genus != "Salmonella" {
		t.Errorf("Genus: got %q, want %q", f.Genus, "Salmonella")
	}
	if len(f.Samples) != 2 || f.Samples[0] != "SAMEA1" || f.Samples[1] != "SAMEA2" {
		t.Errorf("Samples: got %v, want [SAMEA1 SAMEA2]", f.Samples)
	}
	if f.SampleFile != "/tmp/samples.txt" {
		t.Errorf("SampleFile: got %q, want %q", f.SampleFile, "/tmp/samples.txt")
	}
	if len(f.Runs) != 2 || f.Runs[0] != "ERR1" || f.Runs[1] != "SRR2" {
		t.Errorf("Runs: got %v, want [ERR1 SRR2]", f.Runs)
	}
	if f.RunFile != "/tmp/runs.txt" {
		t.Errorf("RunFile: got %q, want %q", f.RunFile, "/tmp/runs.txt")
	}
	if !f.HQOnly {
		t.Error("HQOnly: expected true")
	}
	if f.MinCompleteness != 95.5 {
		t.Errorf("MinCompleteness: got %v, want 95.5", f.MinCompleteness)
	}
	if f.MaxContamination != 2.0 {
		t.Errorf("MaxContamination: got %v, want 2.0", f.MaxContamination)
	}
	if f.MinN50 != 50000 {
		t.Errorf("MinN50: got %v, want 50000", f.MinN50)
	}
	if f.Dataset != "v2" {
		t.Errorf("Dataset: got %q, want %q", f.Dataset, "v2")
	}
	if !f.HasAssembly {
		t.Error("HasAssembly: expected true")
	}
	if f.Country != "United Kingdom" {
		t.Errorf("Country: got %q, want %q", f.Country, "United Kingdom")
	}
	if f.Platform != "Illumina" {
		t.Errorf("Platform: got %q, want %q", f.Platform, "Illumina")
	}
	if f.CollectionDateFrom != "2020-01-01" {
		t.Errorf("CollectionDateFrom: got %q, want %q", f.CollectionDateFrom, "2020-01-01")
	}
	if f.CollectionDateTo != "2023-12-31" {
		t.Errorf("CollectionDateTo: got %q, want %q", f.CollectionDateTo, "2023-12-31")
	}

	o := ff.Output
	if len(o.Columns) != 2 || o.Columns[0] != "sample_id" || o.Columns[1] != "species" {
		t.Errorf("Output.Columns: got %v", o.Columns)
	}
	if o.SortBy != "species" {
		t.Errorf("Output.SortBy: got %q, want %q", o.SortBy, "species")
	}
	if !o.SortDesc {
		t.Error("Output.SortDesc: expected true")
	}
	if o.Limit != 100 {
		t.Errorf("Output.Limit: got %d, want 100", o.Limit)
	}
	if o.Offset != 10 {
		t.Errorf("Output.Offset: got %d, want 10", o.Offset)
	}
	if o.Format != "tsv" {
		t.Errorf("Output.Format: got %q, want %q", o.Format, "tsv")
	}
	if o.Output != "results.tsv" {
		t.Errorf("Output.Output: got %q, want %q", o.Output, "results.tsv")
	}
}

func TestNeedsCheckM2(t *testing.T) {
	cases := []struct {
		name string
		f    Filters
		want bool
	}{
		{"empty", Filters{}, false},
		{"min_completeness", Filters{MinCompleteness: 95.0}, true},
		{"max_contamination", Filters{MaxContamination: 2.0}, true},
		{"species_only", Filters{Species: "Salmonella enterica"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.f.NeedsCheckM2()
			if got != tc.want {
				t.Errorf("NeedsCheckM2() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNeedsAssemblyStats(t *testing.T) {
	cases := []struct {
		name string
		f    Filters
		want bool
	}{
		{"empty", Filters{}, false},
		{"min_n50", Filters{MinN50: 50000}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.f.NeedsAssemblyStats()
			if got != tc.want {
				t.Errorf("NeedsAssemblyStats() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNeedsENA(t *testing.T) {
	cases := []struct {
		name string
		f    Filters
		want bool
	}{
		{"empty", Filters{}, false},
		{"country", Filters{Country: "United Kingdom"}, true},
		{"platform", Filters{Platform: "Illumina"}, true},
		{"date_from", Filters{CollectionDateFrom: "2020-01-01"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.f.NeedsENA()
			if got != tc.want {
				t.Errorf("NeedsENA() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMatchesSpecies(t *testing.T) {
	cases := []struct {
		name    string
		filter  string
		species string
		want    bool
	}{
		{"empty filter matches all", "", "Salmonella enterica", true},
		{"exact match", "Salmonella enterica", "Salmonella enterica", true},
		{"case insensitive match", "salmonella enterica", "Salmonella enterica", true},
		{"no match", "E. coli", "Salmonella enterica", false},
		{"ncbi query matches gtdb genus suffix", "Enterococcus faecium", "Enterococcus_A faecium", true},
		{"ncbi query matches gtdb epithet suffix", "Campylobacter jejunii", "Campylobacter jejunii_A", true},
		{"different species still no match", "Enterococcus faecium", "Enterococcus_A faecalis", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := Filters{Species: tc.filter}
			got := f.MatchesSpecies(tc.species)
			if got != tc.want {
				t.Errorf("MatchesSpecies(%q) with filter %q = %v, want %v", tc.species, tc.filter, got, tc.want)
			}
		})
	}
}

func TestMatchesGenus(t *testing.T) {
	cases := []struct {
		name    string
		filter  string
		species string
		want    bool
	}{
		{"empty filter matches all", "", "Salmonella enterica", true},
		{"genus match", "Salmonella", "Salmonella enterica", true},
		{"case insensitive", "salmonella", "Salmonella enterica", true},
		{"no match", "Escherichia", "Salmonella enterica", false},
		{"ncbi genus matches gtdb genus suffix", "Enterococcus", "Enterococcus_A faecium", true},
		{"different genus still no match", "Enterococcus", "Streptococcus_A pyogenes", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := Filters{Genus: tc.filter}
			if got := f.MatchesGenus(tc.species); got != tc.want {
				t.Errorf("MatchesGenus(%q) with filter %q = %v, want %v", tc.species, tc.filter, got, tc.want)
			}
		})
	}
}

func TestMatchesSpeciesLike(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		species string
		want    bool
	}{
		{"empty filter matches all", "", "Salmonella enterica", true},
		{"prefix wildcard", "Salmonella%", "Salmonella enterica", true},
		{"prefix wildcard no match", "Salmonella%", "E. coli", false},
		{"suffix wildcard", "%coli", "E. coli", true},
		{"suffix wildcard no match", "%coli", "Salmonella enterica", false},
		{"contains wildcard", "%aureus%", "S. aureus", true},
		{"contains wildcard no match", "%aureus%", "E. coli", false},
		{"case insensitive prefix", "salmonella%", "Salmonella enterica", true},
		{"case insensitive suffix", "%COLI", "E. coli", true},
		{"interior wildcard", "Enterococcus%faecium", "Enterococcus_B faecium", true},
		{"interior wildcard no match", "Enterococcus%faecium", "Enterococcus faecalis", false},
		{"underscore is literal", "Streptococcus_A%", "Streptococcus_A pyogenes", true},
		{"underscore does not match other characters", "Streptococcus_A%", "Streptococcus pyogenes", false},
		{"exact match without wildcards", "Salmonella enterica", "Salmonella enterica", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := Filters{SpeciesLike: tc.pattern}
			got := f.MatchesSpeciesLike(tc.species)
			if got != tc.want {
				t.Errorf("MatchesSpeciesLike(%q) with pattern %q = %v, want %v", tc.species, tc.pattern, got, tc.want)
			}
		})
	}
}

func TestLoadSampleFile(t *testing.T) {
	content := `SAMEA1
# this is a comment
SAMEA2

SAMEA3
`
	dir := t.TempDir()
	path := filepath.Join(dir, "samples.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	f := &Filters{SampleFile: path}
	if err := f.LoadSampleFile(); err != nil {
		t.Fatalf("LoadSampleFile returned error: %v", err)
	}

	set := f.SampleSet()
	expected := []string{"SAMEA1", "SAMEA2", "SAMEA3"}
	for _, s := range expected {
		if _, ok := set[s]; !ok {
			t.Errorf("expected %q in sample set", s)
		}
	}
	if len(set) != len(expected) {
		t.Errorf("SampleSet length: got %d, want %d", len(set), len(expected))
	}
}

func TestHasSampleFilter(t *testing.T) {
	t.Run("no samples", func(t *testing.T) {
		f := &Filters{}
		if f.HasSampleFilter() {
			t.Error("expected HasSampleFilter() = false with no samples")
		}
	})

	t.Run("with samples slice", func(t *testing.T) {
		f := &Filters{Samples: []string{"SAMEA1"}}
		if !f.HasSampleFilter() {
			t.Error("expected HasSampleFilter() = true with samples slice")
		}
	})

	t.Run("with sample file loaded", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "samples.txt")
		if err := os.WriteFile(path, []byte("SAMEA1\n"), 0o644); err != nil {
			t.Fatalf("failed to write sample file: %v", err)
		}

		f := &Filters{SampleFile: path}
		if err := f.LoadSampleFile(); err != nil {
			t.Fatalf("LoadSampleFile returned error: %v", err)
		}
		if !f.HasSampleFilter() {
			t.Error("expected HasSampleFilter() = true after loading sample file")
		}
	})
}

func TestSampleSet(t *testing.T) {
	f := &Filters{Samples: []string{"SAMEA1", "SAMEA2", "SAMEA1"}}
	set := f.SampleSet()
	if len(set) != 2 {
		t.Errorf("SampleSet: got %d unique entries, want 2", len(set))
	}
	if _, ok := set["SAMEA1"]; !ok {
		t.Error("SampleSet missing SAMEA1")
	}
	if _, ok := set["SAMEA2"]; !ok {
		t.Error("SampleSet missing SAMEA2")
	}
}

func TestSampleAccessions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "samples.txt")
	if err := os.WriteFile(path, []byte("SAMEA2\nSAMEA3\n"), 0o644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	f := &Filters{Samples: []string{"SAMEA1", "SAMEA2", "SAMEA1"}, SampleFile: path}
	if err := f.LoadSampleFile(); err != nil {
		t.Fatalf("LoadSampleFile returned error: %v", err)
	}

	// Both flags merged, duplicates removed, first-seen order preserved.
	got := f.SampleAccessions()
	want := []string{"SAMEA1", "SAMEA2", "SAMEA3"}
	if len(got) != len(want) {
		t.Fatalf("SampleAccessions() length: got %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SampleAccessions()[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
