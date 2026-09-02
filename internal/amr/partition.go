package amr

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	parquetgo "github.com/parquet-go/parquet-go"

	"github.com/allthebacteria/atb-cli/internal/match"
	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

// PartitionDir is the subdirectory under the data dir where genus partitions live.
const PartitionDir = "amr"

// PartitionThreshold is the minimum number of rows a genus must have to get its
// own partition file. Genera below this threshold are grouped into _other.parquet.
const PartitionThreshold = 10_000

const partitionRowGroupSize = 100_000

const otherPartition = "_other"

// BuildPartitions reads the monolithic amrfinderplus.parquet and writes per-genus
// partition files into <dataDir>/amr/. Uses a streaming two-pass approach:
//
//  1. First pass: count rows per genus (reads only Genus column equivalent).
//  2. Second pass: stream rows into per-genus writers, routing small genera to _other.
//
// logFn is called with progress messages (pass nil to suppress output).
func BuildPartitions(dataDir string, logFn func(string, ...any)) error {
	if logFn == nil {
		logFn = func(string, ...any) {}
	}

	srcPath := filepath.Join(dataDir, AMRFileName)
	outDir := filepath.Join(dataDir, PartitionDir)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("creating partition dir: %w", err)
	}

	logFn("Building AMR genus partitions...")
	start := time.Now()

	// Pass 1: count rows per genus
	genusCounts, _, err := countByGenus(srcPath)
	if err != nil {
		return fmt.Errorf("counting genera: %w", err)
	}

	// Determine which genera get their own file
	promoted := make(map[string]bool, len(genusCounts))
	for genus, count := range genusCounts {
		if count >= PartitionThreshold {
			promoted[genus] = true
		}
	}

	// Pass 2: stream rows into partition writers
	writers := make(map[string]*genusWriter)
	defer func() {
		for _, gw := range writers {
			gw.close()
		}
	}()

	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", AMRFileName, err)
	}
	defer f.Close()

	r := parquetgo.NewGenericReader[pq.AMRRow](f)
	defer r.Close()

	buf := make([]pq.AMRRow, 512)
	var written int64
	for {
		n, readErr := r.Read(buf)
		for i := 0; i < n; i++ {
			row := buf[i]
			key := row.Genus
			if !promoted[key] {
				key = otherPartition
			}

			gw, ok := writers[key]
			if !ok {
				gw, err = newGenusWriter(outDir, key)
				if err != nil {
					return fmt.Errorf("creating writer for %s: %w", key, err)
				}
				writers[key] = gw
			}
			if err := gw.write(row); err != nil {
				return fmt.Errorf("writing row to %s: %w", key, err)
			}
		}
		written += int64(n)
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", AMRFileName, readErr)
		}
	}

	// Close all writers and report
	var otherGenera int
	for key, gw := range writers {
		if err := gw.close(); err != nil {
			return fmt.Errorf("closing writer for %s: %w", key, err)
		}
		if key == otherPartition {
			otherGenera = len(genusCounts) - len(promoted)
			logFn("  %s.parquet (%s rows, %d genera)", key, formatCount(gw.count), otherGenera)
		} else {
			logFn("  %s.parquet (%s rows)", key, formatCount(gw.count))
		}
	}

	elapsed := time.Since(start).Truncate(time.Millisecond)
	logFn("Partitioned %s rows into %d files (%s)", formatCount(written), len(writers), elapsed)

	// Build SQLite indexes for each partition in parallel.
	if err := BuildIndexes(dataDir, logFn); err != nil {
		return fmt.Errorf("building indexes: %w", err)
	}

	return nil
}

// expandGeneraToPartitions replaces each requested genus with the on-disk
// partition genera whose canonical (GTDB-suffix-stripped) name matches it, so
// an NCBI-style genus such as "Enterococcus" resolves to the clade partitions
// "Enterococcus_A" and "Enterococcus_B". A genus with no matching partition is
// kept unchanged so the caller can fall back to the monolithic scan. When no
// partition directory exists, the input is returned as-is.
func expandGeneraToPartitions(dataDir string, genera []string) []string {
	if len(genera) == 0 {
		return genera
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, PartitionDir))
	if err != nil {
		return genera
	}
	var partitions []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".parquet") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".parquet")
		if name == otherPartition {
			continue
		}
		partitions = append(partitions, name)
	}

	seen := make(map[string]struct{}, len(genera))
	var out []string
	add := func(g string) {
		if _, ok := seen[g]; ok {
			return
		}
		seen[g] = struct{}{}
		out = append(out, g)
	}
	for _, g := range genera {
		var matched []string
		for _, p := range partitions {
			if match.SpeciesMatches(g, p) {
				matched = append(matched, p)
			}
		}
		if len(matched) > 0 {
			for _, m := range matched {
				add(m)
			}
			continue
		}
		add(g)
	}
	return out
}

// PartitionPath returns the path to a genus partition file if it exists.
// Returns empty string if the partition doesn't exist. Lookup is case-
// insensitive so that GTDB letter clades (e.g. Legionella_C) match files
// stored under their exact source-case filename.
func PartitionPath(dataDir, genus string) string {
	return findPartitionFile(filepath.Join(dataDir, PartitionDir), genus+".parquet")
}

// PartitionsForSpeciesPattern returns the partition names that can hold rows
// whose species matches pattern, genus partitions first and the _other sweep
// file last. Returns nil when the pattern cannot narrow the search or no
// partitions are built, in which case the caller scans the monolithic file.
func PartitionsForSpeciesPattern(dataDir, pattern string) []string {
	genus, prefix := genusConstraint(pattern)
	if genus == "" && prefix == "" {
		return nil
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, PartitionDir))
	if err != nil {
		return nil
	}

	var names []string
	var hasOther bool
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".parquet") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".parquet")
		switch {
		case name == otherPartition:
			hasOther = true
		case genus != "" && strings.EqualFold(name, genus):
			names = append(names, name)
		case prefix != "" && strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)):
			names = append(names, name)
		}
	}
	sort.Strings(names)

	// A promoted genus holds all of its own rows, so a pinned genus with a
	// partition needs nothing else. Every other case can also have matching
	// rows among the small genera swept into _other.
	if genus != "" && len(names) > 0 {
		return names
	}
	if hasOther {
		names = append(names, otherPartition)
	}
	return names
}

// genusConstraint reports what a species pattern implies about the genus of a
// matching row: an exact genus when the pattern's literal prefix reaches past
// the space, otherwise the prefix that genus must start with. Both are empty
// when the pattern opens with a wildcard and constrains nothing.
func genusConstraint(pattern string) (genus, prefix string) {
	literal := pattern
	if i := strings.IndexByte(literal, '%'); i >= 0 {
		literal = literal[:i]
	}
	if literal == "" {
		return "", ""
	}
	if i := strings.IndexByte(literal, ' '); i >= 0 {
		return literal[:i], ""
	}
	return "", literal
}

// findPartitionFile resolves name in dir, falling back to a case-insensitive
// directory scan if the exact name isn't present. Returns "" when no match
// exists or the directory cannot be read.
func findPartitionFile(dir, name string) string {
	direct := filepath.Join(dir, name)
	if _, err := os.Stat(direct); err == nil {
		return direct
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

func countByGenus(path string) (map[string]int64, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	r := parquetgo.NewGenericReader[pq.AMRRow](f)
	defer r.Close()

	counts := make(map[string]int64)
	buf := make([]pq.AMRRow, 512)
	var total int64

	for {
		n, readErr := r.Read(buf)
		for i := 0; i < n; i++ {
			counts[buf[i].Genus]++
			total++
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, 0, readErr
		}
	}

	return counts, total, nil
}

type genusWriter struct {
	file   *os.File
	writer *parquetgo.GenericWriter[pq.AMRRow]
	count  int64
	buf    []pq.AMRRow
}

func newGenusWriter(dir, name string) (*genusWriter, error) {
	path := filepath.Join(dir, name+".parquet")
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &genusWriter{
		file:   f,
		writer: parquetgo.NewGenericWriter[pq.AMRRow](f, parquetgo.MaxRowsPerRowGroup(partitionRowGroupSize)),
		buf:    make([]pq.AMRRow, 0, 512),
	}, nil
}

func (gw *genusWriter) write(row pq.AMRRow) error {
	gw.buf = append(gw.buf, row)
	gw.count++
	if len(gw.buf) >= 512 {
		return gw.flush()
	}
	return nil
}

func (gw *genusWriter) flush() error {
	if len(gw.buf) == 0 {
		return nil
	}
	if _, err := gw.writer.Write(gw.buf); err != nil {
		return err
	}
	gw.buf = gw.buf[:0]
	return nil
}

func (gw *genusWriter) close() error {
	if gw.writer == nil {
		return nil
	}
	if err := gw.flush(); err != nil {
		return err
	}
	if err := gw.writer.Close(); err != nil {
		return err
	}
	gw.writer = nil
	return gw.file.Close()
}

func formatCount(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%d,%03d,%03d", n/1_000_000, (n/1_000)%1_000, n%1_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%d,%03d", n/1_000, n%1_000)
	}
	return fmt.Sprintf("%d", n)
}
