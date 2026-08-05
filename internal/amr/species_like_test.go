package amr_test

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/amr"
)

func TestPartitionsForSpeciesPattern(t *testing.T) {
	dir := t.TempDir()
	partDir := filepath.Join(dir, amr.PartitionDir)
	if err := os.MkdirAll(partDir, 0o755); err != nil {
		t.Fatalf("mkdir partition dir: %v", err)
	}
	for _, name := range []string{
		"Campylobacter_D.parquet",
		"Campylobacter_E.parquet",
		"Escherichia.parquet",
		"Streptococcus.parquet",
		"Streptococcus_A.parquet",
		"_other.parquet",
	} {
		if err := os.WriteFile(filepath.Join(partDir, name), nil, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	tests := []struct {
		name    string
		pattern string
		want    []string
	}{
		{
			name:    "literal prefix spanning the genus pins one partition",
			pattern: "Campylobacter_D jej%",
			want:    []string{"Campylobacter_D"},
		},
		{
			name:    "exact species name pins one partition",
			pattern: "Escherichia coli",
			want:    []string{"Escherichia"},
		},
		{
			name:    "pinned genus without a partition falls back to _other",
			pattern: "Klebsiella pneu%",
			want:    []string{"_other"},
		},
		{
			name:    "genus prefix keeps every matching partition and _other",
			pattern: "Streptococcus%",
			want:    []string{"Streptococcus", "Streptococcus_A", "_other"},
		},
		{
			name:    "genus prefix excludes non-matching clades",
			pattern: "Streptococcus_A%",
			want:    []string{"Streptococcus_A", "_other"},
		},
		{
			name:    "interior wildcard narrows on the literal genus prefix",
			pattern: "Campylobacter%jejuni",
			want:    []string{"Campylobacter_D", "Campylobacter_E", "_other"},
		},
		{
			name:    "leading wildcard cannot narrow",
			pattern: "%coli",
			want:    nil,
		},
		{
			name:    "empty pattern cannot narrow",
			pattern: "",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := amr.PartitionsForSpeciesPattern(dir, tt.pattern)
			if !equalStrings(got, tt.want) {
				t.Errorf("PartitionsForSpeciesPattern(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestPartitionsForSpeciesPatternWithoutPartitions(t *testing.T) {
	got := amr.PartitionsForSpeciesPattern(t.TempDir(), "Escherichia coli")
	if got != nil {
		t.Errorf("expected no partitions when none are built, got %v", got)
	}
}

func TestQuerySpeciesLikeFiltersRows(t *testing.T) {
	dir := t.TempDir()
	writeAMRFixture(t, dir, []amrFixtureRow{
		{species: "Campylobacter_D jejuni", genus: "Campylobacter_D", count: 4},
		{species: "Campylobacter_D jejuni_A", genus: "Campylobacter_D", count: 2},
		{species: "Campylobacter_D coli", genus: "Campylobacter_D", count: 3},
		{species: "Campylobacter-D jejuni", genus: "Campylobacter-D", count: 5},
		{species: "Enterococcus_B faecium", genus: "Enterococcus_B", count: 1},
	})

	tests := []struct {
		name    string
		pattern string
		want    int
	}{
		{"prefix within a genus", "Campylobacter_D jej%", 6},
		{"underscore is literal, not a wildcard", "Campylobacter_D coli", 3},
		{"interior wildcard", "Enterococcus%faecium", 1},
		{"trailing wildcard on the whole genus", "Campylobacter_D%", 9},
		{"leading wildcard", "%jejuni", 9},
		{"no match", "Salmonella%", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := amr.Query(dir, amr.Filters{SpeciesLike: tt.pattern})
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			if len(results) != tt.want {
				t.Errorf("SpeciesLike %q returned %d rows, want %d", tt.pattern, len(results), tt.want)
			}
		})
	}
}

// TestQuerySpeciesLikeReadsOnlyNeededPartitions corrupts the partitions a
// pattern must not touch, so reading one fails loudly instead of returning the
// right rows from the wrong file.
func TestQuerySpeciesLikeReadsOnlyNeededPartitions(t *testing.T) {
	dir := t.TempDir()
	promoted := amr.PartitionThreshold
	writeAMRFixture(t, dir, []amrFixtureRow{
		{species: "Campylobacter_D jejuni", genus: "Campylobacter_D", count: promoted - 500},
		{species: "Campylobacter_D coli", genus: "Campylobacter_D", count: 500},
		{species: "Escherichia coli", genus: "Escherichia", count: promoted},
		{species: "Campylobacter_E hepaticus", genus: "Campylobacter_E", count: 50},
	})

	if err := amr.BuildPartitions(dir, nil); err != nil {
		t.Fatalf("BuildPartitions: %v", err)
	}

	partDir := filepath.Join(dir, amr.PartitionDir)
	if err := os.WriteFile(filepath.Join(partDir, "Escherichia.parquet"), []byte("not parquet"), 0o644); err != nil {
		t.Fatalf("corrupt Escherichia partition: %v", err)
	}
	if err := os.Remove(filepath.Join(partDir, "Escherichia.sqlite")); err != nil {
		t.Fatalf("remove Escherichia index: %v", err)
	}

	t.Run("pinned genus skips other partitions", func(t *testing.T) {
		results, err := amr.Query(dir, amr.Filters{SpeciesLike: "Campylobacter_D jej%"})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(results) != promoted-500 {
			t.Errorf("expected %d rows, got %d", promoted-500, len(results))
		}
		for _, r := range results {
			if r.Species != "Campylobacter_D jejuni" {
				t.Fatalf("unexpected species %q", r.Species)
			}
		}
	})

	t.Run("genus prefix also reads _other", func(t *testing.T) {
		results, err := amr.Query(dir, amr.Filters{SpeciesLike: "Campylobacter%"})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(results) != promoted+50 {
			t.Errorf("expected %d rows, got %d", promoted+50, len(results))
		}
		var hepaticus int
		for _, r := range results {
			if r.Species == "Campylobacter_E hepaticus" {
				hepaticus++
			}
		}
		if hepaticus != 50 {
			t.Errorf("expected 50 rows from _other, got %d", hepaticus)
		}
	})

	t.Run("limit is applied across partitions", func(t *testing.T) {
		results, err := amr.Query(dir, amr.Filters{SpeciesLike: "Campylobacter%", Limit: 10})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(results) != 10 {
			t.Errorf("expected 10 rows, got %d", len(results))
		}
	})
}

func TestQuerySpeciesLikeSQLiteMatchesParquet(t *testing.T) {
	rows := []amrFixtureRow{
		{species: "Enterococcus_B faecium", genus: "Enterococcus_B", count: 3},
		{species: "Enterococcus faecalis", genus: "Enterococcus", count: 2},
		{species: "Enterococcus_A faecium", genus: "Enterococcus_A", count: 1},
		{species: "Escherichia coli", genus: "Escherichia", count: 4},
	}

	parquetDir := t.TempDir()
	writeAMRFixture(t, parquetDir, rows)

	sqliteDir := t.TempDir()
	writeAMRFixture(t, sqliteDir, rows)
	if err := amr.BuildPartitions(sqliteDir, nil); err != nil {
		t.Fatalf("BuildPartitions: %v", err)
	}
	if amr.IndexPath(sqliteDir, "_other") == "" {
		t.Fatal("expected an _other SQLite index")
	}

	patterns := []string{
		"Enterococcus%faecium",
		"Enterococcus_B%",
		"Enterococcus_B faecium",
		"Escherichia coli",
		"%faecium",
		"%faec%",
		"Enterococcus%",
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			fromParquet, err := amr.Query(parquetDir, amr.Filters{SpeciesLike: pattern})
			if err != nil {
				t.Fatalf("parquet query: %v", err)
			}
			fromSQLite, err := amr.Query(sqliteDir, amr.Filters{SpeciesLike: pattern})
			if err != nil {
				t.Fatalf("sqlite query: %v", err)
			}
			if got, want := speciesCounts(fromSQLite), speciesCounts(fromParquet); got != want {
				t.Errorf("sqlite returned %s, parquet returned %s", got, want)
			}
		})
	}
}

func TestQuerySpeciesLikeCombinesWithGenus(t *testing.T) {
	dir := t.TempDir()
	writeAMRFixture(t, dir, []amrFixtureRow{
		{species: "Campylobacter_D jejuni", genus: "Campylobacter_D", count: 4},
		{species: "Campylobacter_E jejuni", genus: "Campylobacter_E", count: 3},
	})

	results, err := amr.Query(dir, amr.Filters{
		Genera:      []string{"Campylobacter_D"},
		SpeciesLike: "%jejuni",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 4 {
		t.Errorf("expected 4 rows, got %d", len(results))
	}
}

func speciesCounts(results []amr.Result) string {
	counts := make(map[string]int, len(results))
	for _, r := range results {
		counts[r.Species]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + strconv.Itoa(counts[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
