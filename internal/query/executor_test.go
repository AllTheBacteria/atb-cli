package query

import (
	"strconv"
	"testing"
	"time"
)

const fixturesDir = "../../testdata/fixtures"

func TestExecuteSpeciesFilter(t *testing.T) {
	filters := Filters{Species: "Escherichia coli"}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("expected 5 results for E.coli, got %d", len(rows))
	}
}

func TestExecuteHQOnly(t *testing.T) {
	filters := Filters{HQOnly: true}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 17 {
		t.Errorf("expected 17 HQ results, got %d", len(rows))
	}
}

func TestExecuteSpeciesAndHQ(t *testing.T) {
	filters := Filters{Species: "Escherichia coli", HQOnly: true}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("expected 5 results for E.coli + HQ, got %d", len(rows))
	}
}

func TestExecuteWithCheckM2Join(t *testing.T) {
	filters := Filters{
		Species:         "Escherichia coli",
		HQOnly:          true,
		MinCompleteness: 99.0,
	}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 4 {
		t.Errorf("expected 4 results (SAMN12 excluded with completeness=97.0), got %d", len(rows))
	}
}

func TestExecuteWithN50Join(t *testing.T) {
	filters := Filters{
		Species: "Escherichia coli",
		MinN50:  240000,
	}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 results (SAMN2=245k, SAMN11=250k, SAMN19=240k), got %d", len(rows))
	}
}

func TestExecuteWithColumns(t *testing.T) {
	filters := Filters{Species: "Escherichia coli", HQOnly: true}
	cols := []string{"sample_accession", "sylph_species", "N50"}
	rows, err := Execute(fixturesDir, filters, cols)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 5 {
		t.Errorf("expected 5 results, got %d", len(rows))
	}
	for _, row := range rows {
		for _, col := range cols {
			if _, ok := row[col]; !ok {
				t.Errorf("expected column %q in result row, but it was missing", col)
			}
		}
		// Ensure no extra columns
		if len(row) != len(cols) {
			t.Errorf("expected row to have exactly %d columns, got %d", len(cols), len(row))
		}
	}
}

func TestExecuteWithSylphJoin(t *testing.T) {
	filters := Filters{Species: "Escherichia coli"}
	cols := []string{"sample_accession", "Adjusted_ANI", "Taxonomic_abundance", "Sequence_abundance", "Median_cov"}
	rows, err := Execute(fixturesDir, filters, cols)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	got := make(map[string]ResultRow, len(rows))
	for _, r := range rows {
		got[r["sample_accession"]] = r
	}

	// SAMN00000001 has two sylph rows for its assigned species. The one with
	// the higher Taxonomic_abundance wins, so ANI is 95 rather than 94.
	first, ok := got["SAMN00000001"]
	if !ok {
		t.Fatal("SAMN00000001 missing from results")
	}
	for _, tc := range []struct{ col, want string }{
		{"Adjusted_ANI", "95"},
		{"Taxonomic_abundance", "90"},
		{"Sequence_abundance", "88"},
		{"Median_cov", "10"},
	} {
		if first[tc.col] != tc.want {
			t.Errorf("SAMN00000001 %s = %q, want %q", tc.col, first[tc.col], tc.want)
		}
	}

	// SAMN00000002 has a Klebsiella pneumoniae row at abundance 99.9, higher
	// than its own assigned species. Rows are selected by species, so the
	// Escherichia coli row wins and ANI is 95.25 rather than 99.9.
	second, ok := got["SAMN00000002"]
	if !ok {
		t.Fatal("SAMN00000002 missing from results")
	}
	if second["Adjusted_ANI"] != "95.25" {
		t.Errorf("SAMN00000002 Adjusted_ANI = %q, want %q (assigned species, not the higher-abundance decoy)", second["Adjusted_ANI"], "95.25")
	}
}

func TestExecuteWithMLSTJoin(t *testing.T) {
	filters := Filters{Species: "Escherichia coli"}
	// mean_length comes from assembly_stats. Requesting it alongside the mlst
	// columns keeps both groups populated in one result row.
	cols := []string{"sample_accession", "mlst_scheme", "mlst_st", "mlst_status", "mlst_score", "mlst_alleles", "mean_length"}
	rows, err := Execute(fixturesDir, filters, cols)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var first ResultRow
	for _, r := range rows {
		if r["sample_accession"] == "SAMN00000001" {
			first = r
		}
	}
	if first == nil {
		t.Fatal("SAMN00000001 missing from results")
	}

	for _, tc := range []struct{ col, want string }{
		{"mlst_scheme", "ecoli_achtman_4"},
		{"mlst_st", "131"},
		{"mlst_status", "PERFECT"},
		{"mlst_score", "100"},
		{"mlst_alleles", "adk(10);fumC(11);gyrB(4);icd(8);mdh(8);purA(8);recA(2)"},
	} {
		if first[tc.col] != tc.want {
			t.Errorf("SAMN00000001 %s = %q, want %q", tc.col, first[tc.col], tc.want)
		}
	}

	if first["mean_length"] == "" {
		t.Error("mean_length is empty: requesting mlst columns must not blank out other groups")
	}
}

func TestExecuteSylphUnknownSpeciesHasNoValues(t *testing.T) {
	filters := Filters{Species: "unknown"}
	cols := []string{"sample_accession", "Adjusted_ANI"}
	rows, err := Execute(fixturesDir, filters, cols)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 result for species unknown, got %d", len(rows))
	}
	if rows[0]["Adjusted_ANI"] != "" {
		t.Errorf("Adjusted_ANI = %q, want empty: no sylph row matches an unknown species", rows[0]["Adjusted_ANI"])
	}
}

func TestExecuteGenusFilter(t *testing.T) {
	filters := Filters{Genus: "Salmonella"}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 Salmonella results (SAMN5, SAMN6, SAMN18), got %d", len(rows))
	}
}

func TestExecuteDatasetFilter(t *testing.T) {
	filters := Filters{Dataset: "661k"}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(rows) != 7 {
		t.Errorf("expected 7 results for dataset=661k (SAMN1-4, SAMN15-17), got %d", len(rows))
	}
}

func TestExecuteNoResults(t *testing.T) {
	filters := Filters{Species: "Nonexistent"}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 results for nonexistent species, got %d", len(rows))
	}
}

func TestSortResultsNumeric(t *testing.T) {
	rows := []ResultRow{
		{"N50": "100000"},
		{"N50": "250000"},
		{"N50": "50000"},
	}
	SortResults(rows, "N50", true) // descending
	first, _ := strconv.ParseFloat(rows[0]["N50"], 64)
	last, _ := strconv.ParseFloat(rows[len(rows)-1]["N50"], 64)
	if first < last {
		t.Errorf("expected descending numeric sort: first=%v should be >= last=%v", first, last)
	}
}

func TestSortResultsString(t *testing.T) {
	rows := []ResultRow{
		{"sylph_species": "Staphylococcus aureus"},
		{"sylph_species": "Acinetobacter baumannii"},
		{"sylph_species": "Escherichia coli"},
	}
	SortResults(rows, "sylph_species", false) // ascending
	for i := 1; i < len(rows); i++ {
		if rows[i-1]["sylph_species"] > rows[i]["sylph_species"] {
			t.Errorf("expected ascending string sort at index %d: %q > %q",
				i, rows[i-1]["sylph_species"], rows[i]["sylph_species"])
		}
	}
}

func TestExecuteCollectionDateRange(t *testing.T) {
	filters := Filters{
		CollectionDateFrom: "2020-01-01",
		CollectionDateTo:   "2023-12-31",
	}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// Fixture SAMN00000001-04 are 2019 (excluded); SAMN00000005-20 are 2020-2023.
	if len(rows) != 16 {
		t.Errorf("expected 16 rows in 2020-2023 range, got %d", len(rows))
	}
	for _, r := range rows {
		if got := r["collection_date"]; got < "2020-01-01" || got > "2023-12-31" {
			t.Errorf("row outside range: collection_date=%q", got)
		}
	}
}

func TestExecuteCollectionDateFromOnly(t *testing.T) {
	filters := Filters{CollectionDateFrom: "2023-01-01"}
	rows, err := Execute(fixturesDir, filters, nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// SAMN00000017-20 are 2023; everything before is excluded.
	if len(rows) != 4 {
		t.Errorf("expected 4 rows from 2023-01-01, got %d", len(rows))
	}
}

func TestExecuteInvalidCollectionDate(t *testing.T) {
	filters := Filters{CollectionDateFrom: "2020/01/01"}
	if _, err := Execute(fixturesDir, filters, nil); err == nil {
		t.Fatal("expected error for malformed --collection-date-from")
	}
}

func TestParseCollectionDate(t *testing.T) {
	d := func(s string) time.Time {
		t, _ := time.Parse("2006-01-02", s)
		return t
	}
	cases := []struct {
		in         string
		wantOK     bool
		wantStart  time.Time
		wantEnd    time.Time
	}{
		{"2020-05-15", true, d("2020-05-15"), d("2020-05-15")},
		{"2020-05", true, d("2020-05-01"), d("2020-05-31")},
		{"2020", true, d("2020-01-01"), d("2020-12-31")},
		{"2020-01-15T13:45:00Z", true, d("2020-01-15"), d("2020-01-15")},
		{"", false, time.Time{}, time.Time{}},
		{"July 2020", false, time.Time{}, time.Time{}},
		{"2020-13-01", false, time.Time{}, time.Time{}},
		// ISO 8601 intervals
		{"2020-01-01/2020-06-30", true, d("2020-01-01"), d("2020-06-30")},
		{"2019/2020", true, d("2019-01-01"), d("2020-12-31")},
		{"2020-05/2020-08", true, d("2020-05-01"), d("2020-08-31")},
		{"2020-01-01/", false, time.Time{}, time.Time{}},
		{"/2020", false, time.Time{}, time.Time{}},
		{"2020/2019/2018", false, time.Time{}, time.Time{}},
		{"2020-13/2020", false, time.Time{}, time.Time{}},
		{"2020-01-01T00:00:00Z/2020-06-30T00:00:00Z", true, d("2020-01-01"), d("2020-06-30")},
	}
	for _, tc := range cases {
		start, end, ok := parseCollectionDate(tc.in)
		if ok != tc.wantOK {
			t.Errorf("parseCollectionDate(%q) ok=%v, want %v", tc.in, ok, tc.wantOK)
			continue
		}
		if ok && (!start.Equal(tc.wantStart) || !end.Equal(tc.wantEnd)) {
			t.Errorf("parseCollectionDate(%q) = [%s, %s], want [%s, %s]",
				tc.in, start.Format("2006-01-02"), end.Format("2006-01-02"),
				tc.wantStart.Format("2006-01-02"), tc.wantEnd.Format("2006-01-02"))
		}
	}
}

func TestSortResultsEmpty(t *testing.T) {
	rows := []ResultRow{
		{"N50": "300000"},
		{"N50": "100000"},
	}
	original := []string{rows[0]["N50"], rows[1]["N50"]}
	SortResults(rows, "", false) // empty sortBy: no-op
	for i, row := range rows {
		if row["N50"] != original[i] {
			t.Errorf("expected no-op sort with empty sortBy: row %d changed from %q to %q",
				i, original[i], row["N50"])
		}
	}
}
