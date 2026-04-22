package amr_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	parquetgo "github.com/parquet-go/parquet-go"

	"github.com/allthebacteria/atb-cli/internal/amr"
	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

// generateTestParquet creates a synthetic AMR parquet file for partition testing.
// It creates rows across multiple genera with varying counts.
func generateTestParquet(t *testing.T, dir string, genera map[string]int) string {
	t.Helper()
	path := filepath.Join(dir, amr.AMRFileName)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test parquet: %v", err)
	}

	w := parquetgo.NewGenericWriter[pq.AMRRow](f)
	var idx int
	for genus, count := range genera {
		for i := 0; i < count; i++ {
			row := pq.AMRRow{
				Name:           fmt.Sprintf("SAMN%08d", idx),
				GeneSymbol:     fmt.Sprintf("gene_%d", idx%50),
				HierarchyNode:  "node",
				ElementType:    "AMR",
				ElementSubtype: "subtype",
				Coverage:       99.5,
				Identity:       99.8,
				Method:         "BLAST",
				Class:          "BETA-LACTAM",
				Subclass:       "sub",
				Species:        genus + " sp.",
				Genus:          genus,
			}
			if _, err := w.Write([]pq.AMRRow{row}); err != nil {
				t.Fatalf("write row: %v", err)
			}
			idx++
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
	return dir
}

func TestBuildPartitions(t *testing.T) {
	dir := t.TempDir()
	// BigGenus exceeds threshold, SmallGenus does not
	generateTestParquet(t, dir, map[string]int{
		"BigGenus":   15_000,
		"SmallGenus": 500,
	})

	var logs []string
	logFn := func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	if err := amr.BuildPartitions(dir, logFn); err != nil {
		t.Fatalf("BuildPartitions: %v", err)
	}

	// BigGenus should have its own file
	bigPath := filepath.Join(dir, amr.PartitionDir, "BigGenus.parquet")
	if _, err := os.Stat(bigPath); err != nil {
		t.Errorf("expected BigGenus.parquet to exist: %v", err)
	}

	// SmallGenus should NOT have its own file
	smallPath := filepath.Join(dir, amr.PartitionDir, "SmallGenus.parquet")
	if _, err := os.Stat(smallPath); !os.IsNotExist(err) {
		t.Errorf("SmallGenus.parquet should not exist (below threshold)")
	}

	// _other.parquet should exist with SmallGenus rows
	otherPath := filepath.Join(dir, amr.PartitionDir, "_other.parquet")
	if _, err := os.Stat(otherPath); err != nil {
		t.Errorf("expected _other.parquet to exist: %v", err)
	}

	// Verify row counts via ReadStreamFiltered
	bigRows, err := pq.ReadStreamFiltered[pq.AMRRow](bigPath, func(r pq.AMRRow) bool { return true }, 0)
	if err != nil {
		t.Fatalf("reading BigGenus partition: %v", err)
	}
	if len(bigRows) != 15_000 {
		t.Errorf("expected 15000 rows in BigGenus, got %d", len(bigRows))
	}

	otherRows, err := pq.ReadStreamFiltered[pq.AMRRow](otherPath, func(r pq.AMRRow) bool { return true }, 0)
	if err != nil {
		t.Fatalf("reading _other partition: %v", err)
	}
	if len(otherRows) != 500 {
		t.Errorf("expected 500 rows in _other, got %d", len(otherRows))
	}

	if len(logs) == 0 {
		t.Error("expected progress log output")
	}
}

func TestBuildPartitionsAllBelowThreshold(t *testing.T) {
	dir := t.TempDir()
	generateTestParquet(t, dir, map[string]int{
		"TinyA": 100,
		"TinyB": 200,
	})

	if err := amr.BuildPartitions(dir, nil); err != nil {
		t.Fatalf("BuildPartitions: %v", err)
	}

	// Everything should be in _other.parquet
	otherPath := filepath.Join(dir, amr.PartitionDir, "_other.parquet")
	otherRows, err := pq.ReadStreamFiltered[pq.AMRRow](otherPath, func(r pq.AMRRow) bool { return true }, 0)
	if err != nil {
		t.Fatalf("reading _other: %v", err)
	}
	if len(otherRows) != 300 {
		t.Errorf("expected 300 rows in _other, got %d", len(otherRows))
	}

	// No genus-specific files should exist (only _other.parquet and _other.sqlite)
	entries, _ := os.ReadDir(filepath.Join(dir, amr.PartitionDir))
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "_other.") {
			t.Errorf("unexpected partition file: %s", e.Name())
		}
	}
}

func TestQueryUsesPartition(t *testing.T) {
	dir := t.TempDir()
	generateTestParquet(t, dir, map[string]int{
		"Escherichia":    15_000,
		"Staphylococcus": 500,
	})

	if err := amr.BuildPartitions(dir, nil); err != nil {
		t.Fatalf("BuildPartitions: %v", err)
	}

	// Query for a genus that has its own partition
	results, err := amr.Query(dir, amr.Filters{
		Genera: []string{"Escherichia"},
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 10 {
		t.Errorf("expected 10 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Genus != "Escherichia" {
			t.Errorf("expected genus Escherichia, got %q", r.Genus)
		}
	}
}

func TestQueryFallsBackToMonolithic(t *testing.T) {
	// Use fixture data (no partitions built)
	results, err := amr.Query(fixturesDir(t), amr.Filters{
		Genera: []string{"Escherichia"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 9 {
		t.Errorf("expected 9 results from monolithic fallback, got %d", len(results))
	}
}

func TestPartitionPathNormalization(t *testing.T) {
	dir := t.TempDir()
	generateTestParquet(t, dir, map[string]int{
		"Escherichia": 15_000,
	})

	if err := amr.BuildPartitions(dir, nil); err != nil {
		t.Fatalf("BuildPartitions: %v", err)
	}

	// Case-insensitive lookup should find the partition
	tests := []string{"Escherichia", "escherichia", "ESCHERICHIA"}
	for _, genus := range tests {
		path := amr.PartitionPath(dir, genus)
		if path == "" {
			t.Errorf("PartitionPath(%q) returned empty, expected a path", genus)
		}
	}

	// Non-existent genus should return empty
	path := amr.PartitionPath(dir, "Nonexistent")
	if path != "" {
		t.Errorf("PartitionPath(Nonexistent) should be empty, got %q", path)
	}
}

func TestQueryUsesSQLiteIndex(t *testing.T) {
	dir := t.TempDir()
	generateTestParquet(t, dir, map[string]int{
		"Escherichia": 15_000,
	})

	if err := amr.BuildPartitions(dir, nil); err != nil {
		t.Fatalf("BuildPartitions: %v", err)
	}

	// Verify SQLite index was created
	idxPath := amr.IndexPath(dir, "Escherichia")
	if idxPath == "" {
		t.Fatal("expected SQLite index for Escherichia, got none")
	}

	// Query via SQLite (happens automatically since index exists)
	results, err := amr.Query(dir, amr.Filters{
		Genera: []string{"Escherichia"},
		Class:  "BETA-LACTAM",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Genus != "Escherichia" {
			t.Errorf("expected genus Escherichia, got %q", r.Genus)
		}
		if !strings.Contains(strings.ToUpper(r.Class), "BETA-LACTAM") {
			t.Errorf("expected class containing BETA-LACTAM, got %q", r.Class)
		}
	}
}

func TestQuerySQLiteMatchesParquet(t *testing.T) {
	dir := t.TempDir()
	generateTestParquet(t, dir, map[string]int{
		"Escherichia": 15_000,
	})

	// Query without indexes (parquet only)
	parquetResults, err := amr.Query(dir, amr.Filters{
		Genera: []string{"Escherichia"},
		Class:  "BETA-LACTAM",
	})
	if err != nil {
		t.Fatalf("parquet query: %v", err)
	}

	// Build indexes
	if err := amr.BuildPartitions(dir, nil); err != nil {
		t.Fatalf("BuildPartitions: %v", err)
	}

	// Query with indexes (SQLite)
	sqliteResults, err := amr.Query(dir, amr.Filters{
		Genera: []string{"Escherichia"},
		Class:  "BETA-LACTAM",
	})
	if err != nil {
		t.Fatalf("sqlite query: %v", err)
	}

	if len(parquetResults) != len(sqliteResults) {
		t.Errorf("result count mismatch: parquet=%d, sqlite=%d", len(parquetResults), len(sqliteResults))
	}
}

func TestQuerySQLiteWithLargeSampleSet(t *testing.T) {
	dir := t.TempDir()
	generateTestParquet(t, dir, map[string]int{
		"Escherichia": 15_000,
	})

	if err := amr.BuildPartitions(dir, nil); err != nil {
		t.Fatalf("BuildPartitions: %v", err)
	}

	// Build a large sample set (simulates --hq-only with thousands of samples)
	samples := make(map[string]struct{}, 5000)
	for i := 0; i < 5000; i++ {
		samples[fmt.Sprintf("SAMN%08d", i)] = struct{}{}
	}

	results, err := amr.Query(dir, amr.Filters{
		Genera:  []string{"Escherichia"},
		Samples: samples,
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("Query with large sample set failed: %v", err)
	}
	if len(results) > 10 {
		t.Errorf("expected at most 10 results, got %d", len(results))
	}
}
