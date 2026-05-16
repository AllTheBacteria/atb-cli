package index

import (
	"os"
	"path/filepath"
	"testing"
)

// buildTestIndex creates a fresh index from test fixtures in a temp dir.
func buildTestIndex(t *testing.T) *DB {
	t.Helper()

	dir := t.TempDir()
	fixtures := "../../testdata/fixtures"
	for _, name := range []string{"assembly.parquet", "assembly_stats.parquet", "checkm2.parquet", "mlst.parquet"} {
		data, err := os.ReadFile(filepath.Join(fixtures, name))
		if err != nil {
			t.Fatalf("reading fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatalf("writing fixture %s: %v", name, err)
		}
	}

	if err := Build(dir, func(string, ...any) {}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestQuerySpecies(t *testing.T) {
	db := buildTestIndex(t)

	rows, err := db.Query(QueryParams{Species: "Escherichia coli"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("expected 5 E. coli rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r["sylph_species"] != "Escherichia coli" {
			t.Errorf("unexpected species %q", r["sylph_species"])
		}
	}
}

func TestQueryHQOnly(t *testing.T) {
	db := buildTestIndex(t)

	rows, err := db.Query(QueryParams{HQOnly: true})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 17 {
		t.Errorf("expected 17 HQ rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r["hq_filter"] != "PASS" {
			t.Errorf("row %s has hq_filter=%q, expected PASS", r["sample_accession"], r["hq_filter"])
		}
	}
}

func TestQueryWithCompleteness(t *testing.T) {
	db := buildTestIndex(t)

	// completeness >= 99: SAMN00000001(99.5), SAMN00000002(99.2), SAMN00000006(99.0),
	// SAMN00000011(99.3), SAMN00000016(99.1), SAMN00000017(99.0), SAMN00000019(99.4) = 7
	rows, err := db.Query(QueryParams{MinCompleteness: 99})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 7 {
		t.Errorf("expected 7 rows with completeness >= 99, got %d", len(rows))
	}
}

func TestQueryWithN50(t *testing.T) {
	db := buildTestIndex(t)

	// N50 >= 240000: SAMN00000002(245000), SAMN00000007(260000), SAMN00000009(260000),
	// SAMN00000010(300000), SAMN00000011(250000), SAMN00000013(300000),
	// SAMN00000016(350000), SAMN00000017(350000), SAMN00000019(240000) = 9
	rows, err := db.Query(QueryParams{MinN50: 240000})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 9 {
		t.Errorf("expected 9 rows with N50 >= 240000, got %d", len(rows))
	}
}

func TestInfoRow(t *testing.T) {
	db := buildTestIndex(t)

	row, err := db.InfoRow("SAMN00000001")
	if err != nil {
		t.Fatalf("InfoRow: %v", err)
	}

	if row["sylph_species"] != "Escherichia coli" {
		t.Errorf("expected sylph_species=%q, got %q", "Escherichia coli", row["sylph_species"])
	}
	if row["hq_filter"] != "PASS" {
		t.Errorf("expected hq_filter=PASS, got %q", row["hq_filter"])
	}
}

func TestInfoRowNotFound(t *testing.T) {
	db := buildTestIndex(t)

	_, err := db.InfoRow("NONEXISTENT_SAMPLE")
	if err == nil {
		t.Fatal("expected error for nonexistent sample, got nil")
	}
}

func TestQueryMLSTColumns(t *testing.T) {
	db := buildTestIndex(t)

	rows, err := db.Query(QueryParams{
		Species: "Escherichia coli",
		Columns: []string{"sample_accession", "mlst_scheme", "mlst_st", "mlst_status", "mlst_score"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected 5 E. coli rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r["mlst_scheme"] != "ecoli_achtman_4" {
			t.Errorf("sample %s: expected mlst_scheme=ecoli_achtman_4, got %q", r["sample_accession"], r["mlst_scheme"])
		}
		if r["mlst_st"] == "" {
			t.Errorf("sample %s: mlst_st is empty", r["sample_accession"])
		}
		if r["mlst_status"] == "" {
			t.Errorf("sample %s: mlst_status is empty", r["sample_accession"])
		}
	}
}

func TestQueryMLSTFilterByST(t *testing.T) {
	db := buildTestIndex(t)

	// ST131 E. coli: SAMN1, SAMN2, SAMN19 (3 samples)
	rows, err := db.Query(QueryParams{SequenceType: "131"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 ST131 rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r["mlst_st"] != "131" {
			t.Errorf("expected mlst_st=131, got %q", r["mlst_st"])
		}
	}
}

func TestQueryMLSTFilterByScheme(t *testing.T) {
	db := buildTestIndex(t)

	rows, err := db.Query(QueryParams{Scheme: "salmonella"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	// Salmonella enterica samples: SAMN5, SAMN6, SAMN18 = 3
	if len(rows) != 3 {
		t.Errorf("expected 3 salmonella scheme rows, got %d", len(rows))
	}
}

func TestQueryMLSTFilterByStatus(t *testing.T) {
	db := buildTestIndex(t)

	rows, err := db.Query(QueryParams{MLSTStatus: "NONE"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	// NONE: M. tuberculosis (SAMN16, SAMN17) + unknown (SAMN20) = 3
	if len(rows) != 3 {
		t.Errorf("expected 3 NONE status rows, got %d", len(rows))
	}
}

func TestQueryMLSTOnlyExcludesUntyped(t *testing.T) {
	db := buildTestIndex(t)

	// Without MLSTOnly: SAMN16, SAMN17, SAMN20 have mlst_scheme='-' and show up.
	all, err := db.Query(QueryParams{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	var dashed int
	for _, r := range all {
		if r["mlst_scheme"] == "-" || r["mlst_scheme"] == "" {
			dashed++
		}
	}
	if dashed == 0 {
		t.Fatal("fixture precondition failed: expected rows with mlst_scheme in {'', '-'}")
	}

	// With MLSTOnly those rows must be filtered out.
	rows, err := db.Query(QueryParams{MLSTOnly: true})
	if err != nil {
		t.Fatalf("Query with MLSTOnly: %v", err)
	}
	if len(rows) != len(all)-dashed {
		t.Errorf("expected %d rows with MLSTOnly, got %d", len(all)-dashed, len(rows))
	}
	for _, r := range rows {
		if r["mlst_scheme"] == "" || r["mlst_scheme"] == "-" {
			t.Errorf("sample %s leaked through MLSTOnly filter: mlst_scheme=%q",
				r["sample_accession"], r["mlst_scheme"])
		}
	}
}

func TestMLSTForSample(t *testing.T) {
	db := buildTestIndex(t)

	mlst, err := db.MLSTForSample("SAMN00000001")
	if err != nil {
		t.Fatalf("MLSTForSample: %v", err)
	}
	if mlst == nil {
		t.Fatal("expected non-nil MLST result")
	}
	if mlst["mlst_scheme"] != "ecoli_achtman_4" {
		t.Errorf("expected mlst_scheme=ecoli_achtman_4, got %q", mlst["mlst_scheme"])
	}
}

func TestMLSTInInfoRow(t *testing.T) {
	db := buildTestIndex(t)

	row, err := db.InfoRow("SAMN00000001")
	if err != nil {
		t.Fatalf("InfoRow: %v", err)
	}
	if row["mlst_scheme"] != "ecoli_achtman_4" {
		t.Errorf("InfoRow: expected mlst_scheme=ecoli_achtman_4, got %q", row["mlst_scheme"])
	}
	if row["mlst_st"] == "" {
		t.Error("InfoRow: mlst_st is empty")
	}
}

// SAMN12 is the only fixture row with asm_fasta_on_osf=0. It's also E. coli +
// PASS + Incr_release.202408, so the published-only filter must reduce all
// three counters together: total 20→19, HQ 17→16, E. coli 5→4,
// Incr_release.202408 4→3.
func TestQueryStatsExcludesUnpublished(t *testing.T) {
	db := buildTestIndex(t)

	stats, err := db.QueryStats("", false)
	if err != nil {
		t.Fatalf("QueryStats: %v", err)
	}
	if stats.Total != 19 {
		t.Errorf("Total: expected 19 (20 minus SAMN12), got %d", stats.Total)
	}
	if stats.HQCount != 16 {
		t.Errorf("HQCount: expected 16 (17 PASS minus SAMN12), got %d", stats.HQCount)
	}
	if got := stats.Datasets["Incr_release.202408"]; got != 3 {
		t.Errorf("Datasets[Incr_release.202408]: expected 3, got %d", got)
	}

	var ecoli int
	for _, sc := range stats.TopSpecies {
		if sc.Species == "Escherichia coli" {
			ecoli = sc.Count
		}
	}
	if ecoli != 4 {
		t.Errorf("TopSpecies E. coli: expected 4, got %d", ecoli)
	}
}

// QueryStats keeps "unknown" in TopSpecies because it represents a real
// published category (assembled but unclassifiable by sylph). SpeciesList,
// in contrast, excludes it — see TestSpeciesListExcludesUnknown.
func TestQueryStatsTopSpeciesIncludesUnknown(t *testing.T) {
	db := buildTestIndex(t)

	stats, err := db.QueryStats("", false)
	if err != nil {
		t.Fatalf("QueryStats: %v", err)
	}

	var found bool
	for _, sc := range stats.TopSpecies {
		if sc.Species == "unknown" {
			found = true
			if sc.Count != 1 {
				t.Errorf("TopSpecies unknown: expected count 1, got %d", sc.Count)
			}
		}
	}
	if !found {
		t.Errorf("TopSpecies should include \"unknown\" (SAMN20), got %+v", stats.TopSpecies)
	}
}

func TestQueryStatsSpeciesFilter(t *testing.T) {
	db := buildTestIndex(t)

	// E. coli with the published-only filter: 5 rows minus SAMN12 = 4.
	stats, err := db.QueryStats("Escherichia coli", false)
	if err != nil {
		t.Fatalf("QueryStats: %v", err)
	}
	if stats.Total != 4 {
		t.Errorf("Total for E. coli: expected 4, got %d", stats.Total)
	}
	if stats.HQCount != 4 {
		t.Errorf("HQCount for E. coli: expected 4 (all PASS), got %d", stats.HQCount)
	}
}

func TestQueryStatsHQOnly(t *testing.T) {
	db := buildTestIndex(t)

	// HQOnly applies on top of the published filter: 17 PASS minus SAMN12 = 16.
	stats, err := db.QueryStats("", true)
	if err != nil {
		t.Fatalf("QueryStats: %v", err)
	}
	if stats.Total != 16 {
		t.Errorf("Total with HQOnly: expected 16, got %d", stats.Total)
	}
	if stats.HQCount != 16 {
		t.Errorf("HQCount with HQOnly: expected 16, got %d", stats.HQCount)
	}
}

func TestSpeciesListExcludesUnpublished(t *testing.T) {
	db := buildTestIndex(t)

	list, err := db.SpeciesList(0)
	if err != nil {
		t.Fatalf("SpeciesList: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("SpeciesList returned no rows")
	}
	// Ordered by count desc: E. coli is the unique top entry at 4
	// (S. aureus, S. enterica are tied at 3 below).
	if list[0].Species != "Escherichia coli" {
		t.Errorf("first species: expected Escherichia coli, got %q", list[0].Species)
	}
	if list[0].Count != 4 {
		t.Errorf("E. coli count: expected 4 (post-filter), got %d", list[0].Count)
	}

	// Sum of all named-species counts after the filter must equal 18
	// (19 published rows minus the 1 "unknown" SpeciesList drops).
	var sum int
	for _, sc := range list {
		sum += sc.Count
	}
	if sum != 18 {
		t.Errorf("sum of species counts: expected 18, got %d (rows=%+v)", sum, list)
	}
}

func TestSpeciesListExcludesUnknown(t *testing.T) {
	db := buildTestIndex(t)

	list, err := db.SpeciesList(0)
	if err != nil {
		t.Fatalf("SpeciesList: %v", err)
	}
	for _, sc := range list {
		if sc.Species == "unknown" || sc.Species == "" {
			t.Errorf("SpeciesList should not return %q rows, got %+v", sc.Species, sc)
		}
	}
}

func TestSpeciesListLimit(t *testing.T) {
	db := buildTestIndex(t)

	list, err := db.SpeciesList(3)
	if err != nil {
		t.Fatalf("SpeciesList: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected limit=3 to return 3 rows, got %d", len(list))
	}
	if list[0].Species != "Escherichia coli" {
		t.Errorf("first species: expected Escherichia coli, got %q", list[0].Species)
	}
}
