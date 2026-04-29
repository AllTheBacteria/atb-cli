package amr_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	parquetgo "github.com/parquet-go/parquet-go"

	"github.com/allthebacteria/atb-cli/internal/amr"
	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

type amrFixtureRow struct {
	species string
	genus   string
	count   int
}

func writeAMRFixture(t *testing.T, dir string, rows []amrFixtureRow) {
	t.Helper()
	path := filepath.Join(dir, amr.AMRFileName)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture parquet: %v", err)
	}
	defer f.Close()

	w := parquetgo.NewGenericWriter[pq.AMRRow](f)
	var idx int
	for _, r := range rows {
		for i := 0; i < r.count; i++ {
			row := pq.AMRRow{
				Name:           fmt.Sprintf("SAMN%08d", idx),
				GeneSymbol:     fmt.Sprintf("gene_%d", idx),
				ElementType:    "AMR",
				ElementSubtype: "AMR",
				Coverage:       100,
				Identity:       100,
				Method:         "EXACT",
				Class:          "BETA-LACTAM",
				Subclass:       "BETA-LACTAM",
				Species:        r.species,
				Genus:          r.genus,
			}
			if _, err := w.Write([]pq.AMRRow{row}); err != nil {
				t.Fatalf("write fixture row: %v", err)
			}
			idx++
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close fixture writer: %v", err)
	}
}

func fixturesDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "testdata", "fixtures")
}

func TestQueryAMRByGenus(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		Genera:      []string{"Escherichia"},
		ElementType: "AMR",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}
	for _, r := range results {
		if r.SampleAccession == "" {
			t.Error("expected non-empty SampleAccession")
		}
		if r.ElementType != "AMR" {
			t.Errorf("expected ElementType=AMR, got %q", r.ElementType)
		}
	}
}

func TestQueryAMRAllRows(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), amr.Filters{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	// 10 AMR + 5 STRESS = 15 rows total
	if len(results) != 15 {
		t.Errorf("expected 15 rows, got %d", len(results))
	}
	// Verify Genus field is populated
	first := results[0]
	if first.Genus == "" {
		t.Error("expected non-empty Genus")
	}
}

func TestQueryAMRFilterBySamples(t *testing.T) {
	wanted := map[string]struct{}{
		"SAMN00000001": {},
		"SAMN00000002": {},
	}
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		Samples: wanted,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	for _, r := range results {
		if _, ok := wanted[r.SampleAccession]; !ok {
			t.Errorf("unexpected sample %q in results", r.SampleAccession)
		}
	}
	if len(results) == 0 {
		t.Error("expected at least one result for filtered samples")
	}
}

func TestQueryAMRFilterByClass(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		Class: "EFFLUX",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for EFFLUX class")
	}
	for _, r := range results {
		if r.Class != "EFFLUX" {
			t.Errorf("expected Class=EFFLUX, got %q", r.Class)
		}
	}
}

func TestQueryAMRFilterByGenePattern(t *testing.T) {
	// Pattern "bla%" should match blaTEM-1, blaEC, blaOXA-1, blaCTX-M-15
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		GenePattern: "bla%",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results matching bla% pattern")
	}
	for _, r := range results {
		gene := r.GeneSymbol
		if len(gene) < 3 || gene[:3] != "bla" {
			t.Errorf("gene %q does not match bla%% pattern", gene)
		}
	}
}

func TestQueryAMRFilterByMinCoverage(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		MinCoverage: 99.0,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	for _, r := range results {
		if r.Coverage < 99.0 {
			t.Errorf("expected coverage >= 99.0, got %v for gene %q", r.Coverage, r.GeneSymbol)
		}
	}
}

func TestQueryStress(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		ElementType: "STRESS",
	})
	if err != nil {
		t.Fatalf("Query stress: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 stress results, got %d", len(results))
	}
	for _, r := range results {
		if r.ElementType != "STRESS" {
			t.Errorf("expected ElementType=STRESS, got %q", r.ElementType)
		}
	}
}

func TestQueryAll(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		ElementType: "all",
	})
	if err != nil {
		t.Fatalf("Query all: %v", err)
	}
	// "all" element type filter passes everything through
	if len(results) < 15 {
		t.Errorf("expected at least 15 results from 'all' query, got %d", len(results))
	}
}

func TestQueryFilterByGenus(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		Genera: []string{"Escherichia"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 9 {
		t.Errorf("expected 9 results for Escherichia, got %d", len(results))
	}
	for _, r := range results {
		if r.Genus != "Escherichia" {
			t.Errorf("expected Genus=Escherichia, got %q", r.Genus)
		}
	}
}

func TestQueryFilterByGenusNoMatch(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		Genera: []string{"Klebsiella"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for Klebsiella, got %d", len(results))
	}
}

func TestQueryWithLimit(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		Genera: []string{"Escherichia"},
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results with limit=3, got %d", len(results))
	}
}

func TestQueryWithLimitExceedingResults(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		Genera: []string{"Escherichia"},
		Limit:  100,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 9 {
		t.Errorf("expected 9 results (limit exceeds matches), got %d", len(results))
	}
}

func TestQueryWithZeroLimitReturnsAll(t *testing.T) {
	results, err := amr.Query(fixturesDir(t), amr.Filters{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 15 {
		t.Errorf("expected 15 rows with no limit, got %d", len(results))
	}
}

func TestQueryMultipleGenera(t *testing.T) {
	// First discover what genera exist in fixture data
	all, err := amr.Query(fixturesDir(t), amr.Filters{})
	if err != nil {
		t.Fatalf("Query all: %v", err)
	}
	genusCounts := make(map[string]int)
	for _, r := range all {
		genusCounts[r.Genus]++
	}

	// Query for Escherichia + Staphylococcus
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		Genera: []string{"Escherichia", "Staphylococcus"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	expected := genusCounts["Escherichia"] + genusCounts["Staphylococcus"]
	if len(results) != expected {
		t.Errorf("expected %d results for both genera, got %d", expected, len(results))
	}

	// Verify only requested genera are returned
	for _, r := range results {
		if r.Genus != "Escherichia" && r.Genus != "Staphylococcus" {
			t.Errorf("unexpected genus %q in results", r.Genus)
		}
	}
}

func TestQueryFilterBySpeciesIsNarrowerThanGenus(t *testing.T) {
	// Build a fixture with two species under the same genus so we can
	// confirm a species filter returns a strict subset of the genus filter.
	dir := t.TempDir()
	writeAMRFixture(t, dir, []amrFixtureRow{
		{species: "Acinetobacter baumannii", genus: "Acinetobacter", count: 5},
		{species: "Acinetobacter radioresistens", genus: "Acinetobacter", count: 3},
		{species: "Klebsiella pneumoniae", genus: "Klebsiella", count: 2},
	})

	genusResults, err := amr.Query(dir, amr.Filters{
		Genera: []string{"Acinetobacter"},
	})
	if err != nil {
		t.Fatalf("Query genus: %v", err)
	}
	if len(genusResults) != 8 {
		t.Fatalf("expected 8 genus results, got %d", len(genusResults))
	}

	speciesResults, err := amr.Query(dir, amr.Filters{
		Species: []string{"Acinetobacter baumannii"},
	})
	if err != nil {
		t.Fatalf("Query species: %v", err)
	}
	if len(speciesResults) != 5 {
		t.Errorf("expected 5 results for Acinetobacter baumannii, got %d", len(speciesResults))
	}
	for _, r := range speciesResults {
		if r.Species != "Acinetobacter baumannii" {
			t.Errorf("unexpected species %q", r.Species)
		}
	}

	multiSpecies, err := amr.Query(dir, amr.Filters{
		Species: []string{"Acinetobacter baumannii", "Klebsiella pneumoniae"},
	})
	if err != nil {
		t.Fatalf("Query multi species: %v", err)
	}
	if len(multiSpecies) != 7 {
		t.Errorf("expected 7 results for two species, got %d", len(multiSpecies))
	}
}

func TestQueryFilterBySpeciesCaseInsensitive(t *testing.T) {
	all, err := amr.Query(fixturesDir(t), amr.Filters{})
	if err != nil {
		t.Fatalf("Query all: %v", err)
	}
	if len(all) == 0 {
		t.Skip("empty fixture")
	}
	target := all[0].Species
	if target == "" {
		t.Skip("fixture row has empty species")
	}
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		Species: []string{strings.ToUpper(target)},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Errorf("expected case-insensitive species match to return results")
	}
}

func TestQueryNoGenusWithGeneFilter(t *testing.T) {
	// Search for a gene across all genera (no Genera filter)
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		GenePattern: "bla%",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for gene bla% across all genera")
	}
	for _, r := range results {
		if len(r.GeneSymbol) < 3 || r.GeneSymbol[:3] != "bla" {
			t.Errorf("gene %q does not match bla%% pattern", r.GeneSymbol)
		}
	}
}
