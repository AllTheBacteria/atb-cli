package amr

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	parquetgo "github.com/parquet-go/parquet-go"

	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

func writeBenchPartition(tb testing.TB, path string, nRows int) {
	tb.Helper()
	f, err := os.Create(path)
	if err != nil {
		tb.Fatalf("create parquet: %v", err)
	}
	defer f.Close()

	w := parquetgo.NewGenericWriter[pq.AMRRow](f)

	classes := []string{"BETA-LACTAM", "AMINOGLYCOSIDE", "EFFLUX", "TETRACYCLINE", "CARBAPENEM"}
	types := []string{"AMR", "STRESS", "VIRULENCE"}

	buf := make([]pq.AMRRow, 0, 1000)
	for i := 0; i < nRows; i++ {
		buf = append(buf, pq.AMRRow{
			Name:           fmt.Sprintf("SAMN%08d", i),
			GeneSymbol:     fmt.Sprintf("gene_%d", i%200),
			Class:          classes[i%len(classes)],
			ElementType:    types[i%len(types)],
			ElementSubtype: "subtype",
			HierarchyNode:  fmt.Sprintf("node_%d", i%50),
			Method:         "BLAST",
			Subclass:       "sub",
			Coverage:       90.0 + float64(i%100)/10.0,
			Identity:       95.0 + float64(i%50)/10.0,
			Genus:          "Escherichia",
			Species:        "Escherichia coli",
		})
		if len(buf) == 1000 {
			if _, err := w.Write(buf); err != nil {
				tb.Fatalf("write parquet: %v", err)
			}
			buf = buf[:0]
		}
	}
	if len(buf) > 0 {
		if _, err := w.Write(buf); err != nil {
			tb.Fatalf("write parquet: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		tb.Fatalf("close parquet writer: %v", err)
	}
}

func BenchmarkBuildOneIndex(b *testing.B) {
	for _, nRows := range []int{10_000, 100_000, 1_000_000} {
		nRows := nRows
		b.Run(fmt.Sprintf("%d", nRows), func(b *testing.B) {
			if nRows == 1_000_000 && testing.Short() {
				b.Skip("slow")
			}

			dir := b.TempDir()
			pqPath := fmt.Sprintf("%s/bench.parquet", dir)

			b.StopTimer()
			writeBenchPartition(b, pqPath, nRows)
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				sqlitePath := fmt.Sprintf("%s/bench_%d.sqlite", dir, i)
				os.Remove(sqlitePath)

				count, err := buildOneIndex(pqPath, sqlitePath)
				if err != nil {
					b.Fatalf("buildOneIndex: %v", err)
				}
				if count != int64(nRows) {
					b.Fatalf("expected %d rows, got %d", nRows, count)
				}
			}
		})
	}
}

func BenchmarkBuildOneIndexPeakRSS(b *testing.B) {
	if testing.Short() {
		b.Skip("slow")
	}

	const nRows = 1_000_000

	dir := b.TempDir()
	pqPath := fmt.Sprintf("%s/bench_rss.parquet", dir)

	b.StopTimer()
	writeBenchPartition(b, pqPath, nRows)

	for i := 0; i < b.N; i++ {
		if i > 0 {
			// Single-shot measurement — skip extra iterations.
			break
		}

		runtime.GC()
		var baseline runtime.MemStats
		runtime.ReadMemStats(&baseline)

		var (
			peak   uint64
			mu     sync.Mutex
			stopCh = make(chan struct{})
			doneCh = make(chan struct{})
		)

		go func() {
			defer close(doneCh)
			var ms runtime.MemStats
			for {
				select {
				case <-stopCh:
					return
				case <-time.After(10 * time.Millisecond):
					// HeapAlloc is only updated on GC, so this is a noisy lower bound
					// rather than true peak; acceptable for regression tracking.
					runtime.ReadMemStats(&ms)
					mu.Lock()
					if ms.HeapAlloc > peak {
						peak = ms.HeapAlloc
					}
					mu.Unlock()
				}
			}
		}()

		sqlitePath := fmt.Sprintf("%s/bench_rss.sqlite", dir)
		b.StartTimer()
		count, err := buildOneIndex(pqPath, sqlitePath)
		b.StopTimer()

		close(stopCh)
		<-doneCh

		if err != nil {
			b.Fatalf("buildOneIndex: %v", err)
		}
		if count != int64(nRows) {
			b.Fatalf("expected %d rows, got %d", nRows, count)
		}

		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		mu.Lock()
		if after.HeapAlloc > peak {
			peak = after.HeapAlloc
		}
		mu.Unlock()

		b.ReportMetric(float64(peak)/1024/1024, "peak_heap_MB")
		b.ReportMetric(float64(peak-baseline.HeapAlloc)/1024/1024, "peak_heap_delta_MB")
		b.ReportMetric(float64(after.TotalAlloc-baseline.TotalAlloc)/1024/1024, "total_alloc_MB")
	}
}
