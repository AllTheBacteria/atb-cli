package amr_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	parquetgo "github.com/parquet-go/parquet-go"

	"github.com/allthebacteria/atb-cli/internal/amr"
	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

// writeAMRParquetAt writes the fixture rows to an explicit parquet path.
func writeAMRParquetAt(t *testing.T, path string, rows []amrFixtureRow) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create parquet: %v", err)
	}
	defer f.Close()

	w := parquetgo.NewGenericWriter[pq.AMRRow](f)
	var idx int
	for _, r := range rows {
		for i := 0; i < r.count; i++ {
			row := pq.AMRRow{
				Name:        fmt.Sprintf("SAMN%08d", idx),
				GeneSymbol:  fmt.Sprintf("gene_%d", idx),
				ElementType: "AMR",
				Coverage:    100,
				Identity:    100,
				Method:      "EXACT",
				Class:       "BETA-LACTAM",
				Species:     r.species,
				Genus:       r.genus,
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
}

// TestQueryAMRSpeciesGTDBSuffix verifies that an NCBI-style --species query
// matches GTDB-split rows on the monolithic scan path.
func TestQueryAMRSpeciesGTDBSuffix(t *testing.T) {
	dir := t.TempDir()
	writeAMRFixture(t, dir, []amrFixtureRow{
		{species: "Enterococcus_A faecium", genus: "Enterococcus_A", count: 2},
		{species: "Enterococcus_B faecium", genus: "Enterococcus_B", count: 3},
		{species: "Escherichia coli", genus: "Escherichia", count: 4},
	})

	results, err := amr.Query(dir, amr.Filters{
		Genera:  []string{"Enterococcus"},
		Species: []string{"Enterococcus faecium"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 GTDB-split rows, got %d", len(results))
	}
}

// TestQueryAMRGenusGTDBPartitions verifies that an NCBI-style genus query reads
// every on-disk GTDB clade partition, even when no monolithic file is present.
func TestQueryAMRGenusGTDBPartitions(t *testing.T) {
	dir := t.TempDir()
	writeAMRParquetAt(t, filepath.Join(dir, amr.PartitionDir, "Enterococcus_A.parquet"),
		[]amrFixtureRow{{species: "Enterococcus_A faecium", genus: "Enterococcus_A", count: 2}})
	writeAMRParquetAt(t, filepath.Join(dir, amr.PartitionDir, "Enterococcus_B.parquet"),
		[]amrFixtureRow{{species: "Enterococcus_B faecium", genus: "Enterococcus_B", count: 3}})

	results, err := amr.Query(dir, amr.Filters{
		Genera: []string{"Enterococcus"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 rows across both clade partitions, got %d", len(results))
	}
}
