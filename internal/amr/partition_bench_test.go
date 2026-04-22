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

func setupBenchData(b *testing.B, nRows int) string {
	b.Helper()
	dir := b.TempDir()
	path := filepath.Join(dir, amr.AMRFileName)
	f, err := os.Create(path)
	if err != nil {
		b.Fatalf("create: %v", err)
	}

	w := parquetgo.NewGenericWriter[pq.AMRRow](f)

	type genDist struct {
		genus string
		pct   float64
	}
	dist := []genDist{
		{"Escherichia", 0.60},
		{"Salmonella", 0.25},
		{"Staphylococcus", 0.06},
		{"Klebsiella", 0.05},
		{"Pseudomonas", 0.04},
	}

	classes := []string{"BETA-LACTAM", "AMINOGLYCOSIDE", "EFFLUX", "TETRACYCLINE", "CARBAPENEM"}
	types := []string{"AMR", "STRESS", "VIRULENCE"}

	buf := make([]pq.AMRRow, 0, 1000)
	var gi int
	for _, d := range dist {
		count := int(float64(nRows) * d.pct)
		for i := 0; i < count; i++ {
			row := pq.AMRRow{
				Name:           fmt.Sprintf("SAMN%08d", gi),
				GeneSymbol:     fmt.Sprintf("gene_%d", gi%200),
				HierarchyNode:  "node",
				ElementType:    types[gi%len(types)],
				ElementSubtype: "subtype",
				Coverage:       90.0 + float64(gi%100)/10.0,
				Identity:       95.0 + float64(gi%50)/10.0,
				Method:         "BLAST",
				Class:          classes[gi%len(classes)],
				Subclass:       "sub",
				Species:        d.genus + " sp.",
				Genus:          d.genus,
			}
			buf = append(buf, row)
			if len(buf) == 1000 {
				if _, err := w.Write(buf); err != nil {
					b.Fatal(err)
				}
				buf = buf[:0]
			}
			gi++
		}
	}
	if len(buf) > 0 {
		if _, err := w.Write(buf); err != nil {
			b.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		b.Fatal(err)
	}
	if err := f.Close(); err != nil {
		b.Fatal(err)
	}

	return dir
}

func setupMonolithic(b *testing.B, nRows int) string {
	return setupBenchData(b, nRows)
}

func setupParquetOnly(b *testing.B, nRows int) string {
	dir := setupBenchData(b, nRows)
	if err := amr.BuildPartitions(dir, nil); err != nil {
		b.Fatal(err)
	}
	// Remove SQLite files, keep only parquet partitions
	entries, _ := os.ReadDir(filepath.Join(dir, amr.PartitionDir))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sqlite") {
			os.Remove(filepath.Join(dir, amr.PartitionDir, e.Name()))
		}
	}
	return dir
}

func setupWithSQLite(b *testing.B, nRows int) string {
	dir := setupBenchData(b, nRows)
	if err := amr.BuildPartitions(dir, nil); err != nil {
		b.Fatal(err)
	}
	return dir
}

// --- 500K: Full genus scan (no limit) ---

func BenchmarkQuery_Monolithic_500K_FullScan(b *testing.B) {
	dir := setupMonolithic(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}

func BenchmarkQuery_Parquet_500K_FullScan(b *testing.B) {
	dir := setupParquetOnly(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}

func BenchmarkQuery_SQLite_500K_FullScan(b *testing.B) {
	dir := setupWithSQLite(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}

// --- 500K: Genus + Limit 100 ---

func BenchmarkQuery_Monolithic_500K_Limit100(b *testing.B) {
	dir := setupMonolithic(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}, Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) != 100 {
			b.Fatalf("got %d", len(r))
		}
	}
}

func BenchmarkQuery_Parquet_500K_Limit100(b *testing.B) {
	dir := setupParquetOnly(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}, Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) != 100 {
			b.Fatalf("got %d", len(r))
		}
	}
}

func BenchmarkQuery_SQLite_500K_Limit100(b *testing.B) {
	dir := setupWithSQLite(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}, Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) != 100 {
			b.Fatalf("got %d", len(r))
		}
	}
}

// --- 500K: Class filter ---

func BenchmarkQuery_Monolithic_500K_ClassFilter(b *testing.B) {
	dir := setupMonolithic(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}, Class: "BETA-LACTAM", Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}

func BenchmarkQuery_SQLite_500K_ClassFilter(b *testing.B) {
	dir := setupWithSQLite(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}, Class: "BETA-LACTAM", Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}

// --- 500K: Gene pattern filter ---

func BenchmarkQuery_Monolithic_500K_GenePattern(b *testing.B) {
	dir := setupMonolithic(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}, GenePattern: "gene_10%", Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}

func BenchmarkQuery_SQLite_500K_GenePattern(b *testing.B) {
	dir := setupWithSQLite(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}, GenePattern: "gene_10%", Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}

// --- 1M: Full scan and limit ---

func BenchmarkQuery_Monolithic_1M_FullScan(b *testing.B) {
	dir := setupMonolithic(b, 1_000_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}

func BenchmarkQuery_Parquet_1M_FullScan(b *testing.B) {
	dir := setupParquetOnly(b, 1_000_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}

func BenchmarkQuery_SQLite_1M_FullScan(b *testing.B) {
	dir := setupWithSQLite(b, 1_000_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}

func BenchmarkQuery_Monolithic_1M_Limit100(b *testing.B) {
	dir := setupMonolithic(b, 1_000_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}, Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) != 100 {
			b.Fatalf("got %d", len(r))
		}
	}
}

func BenchmarkQuery_SQLite_1M_Limit100(b *testing.B) {
	dir := setupWithSQLite(b, 1_000_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia"}, Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) != 100 {
			b.Fatalf("got %d", len(r))
		}
	}
}

// --- 500K: Multi-genus ---

func BenchmarkQuery_Monolithic_500K_MultiGenus(b *testing.B) {
	dir := setupMonolithic(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia", "Salmonella"}, Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}

func BenchmarkQuery_SQLite_500K_MultiGenus(b *testing.B) {
	dir := setupWithSQLite(b, 500_000)
	b.ResetTimer()
	for b.Loop() {
		r, err := amr.Query(dir, amr.Filters{Genera: []string{"Escherichia", "Salmonella"}, Limit: 100})
		if err != nil {
			b.Fatal(err)
		}
		if len(r) == 0 {
			b.Fatal("no results")
		}
	}
}
