package amr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

// AMRFileName is the single merged parquet file containing all AMR data.
const AMRFileName = "amrfinderplus.parquet"

// v4SchemaMarker is a column name unique to AMRFinderPlus v4.2.5. In v3.12.8
// this column was named "Gene symbol", so its presence is a reliable v4 probe.
const v4SchemaMarker = "Element symbol"

// Filters controls which AMR rows are returned by Query.
type Filters struct {
	// Samples restricts results to a specific set of sample accessions.
	// Nil or empty means no restriction.
	Samples map[string]struct{}
	// Class filters by drug class (case-insensitive substring match). Empty means all.
	Class string
	// GenePattern filters by gene symbol. Supports % wildcards (prefix/suffix/contains). Empty means all.
	GenePattern string
	// MinCoverage is the minimum coverage percentage (0 = no minimum).
	MinCoverage float64
	// MinIdentity is the minimum identity percentage (0 = no minimum).
	MinIdentity float64
	// ElementType restricts to a specific element type ("AMR", "STRESS", "VIRULENCE"). Empty means all.
	ElementType string
	// Genera restricts results to specific bacterial genera (case-insensitive).
	// Nil or empty means all genera (full scan).
	Genera []string
	// Species restricts results to specific full species names, e.g. "Escherichia coli"
	// (case-insensitive exact match against the row's Species field).
	// Nil or empty means no species-level restriction.
	Species []string
	// Limit caps the number of returned results. 0 means no limit.
	Limit int
}

// Result is a single AMR gene hit associated with a sample. The JSON tags
// preserve the AMRFinderPlus v4.2.5 column names verbatim so MCP output and
// the original TSV use identical headers.
type Result struct {
	SampleAccession           string  `json:"Name"`
	ProteinID                 string  `json:"Protein id"`
	ContigID                  string  `json:"Contig id"`
	Start                     int64   `json:"Start"`
	Stop                      int64   `json:"Stop"`
	Strand                    string  `json:"Strand"`
	GeneSymbol                string  `json:"Element symbol"`
	ElementName               string  `json:"Element name"`
	Scope                     string  `json:"Scope"`
	ElementType               string  `json:"Type"`
	ElementSubtype            string  `json:"Subtype"`
	Class                     string  `json:"Class"`
	Subclass                  string  `json:"Subclass"`
	Method                    string  `json:"Method"`
	TargetLength              int64   `json:"Target length"`
	ReferenceSequenceLength   int64   `json:"Reference sequence length"`
	Coverage                  float64 `json:"% Coverage of reference"`
	Identity                  float64 `json:"% Identity to reference"`
	AlignmentLength           int64   `json:"Alignment length"`
	ClosestReferenceAccession string  `json:"Closest reference accession"`
	ClosestReferenceName      string  `json:"Closest reference name"`
	HMMAccession              string  `json:"HMM accession"`
	HMMDescription            string  `json:"HMM description"`
	HierarchyNode             string  `json:"Hierarchy node"`
	Genus                     string  `json:"genus"`
	Species                   string  `json:"species"`
}

// Query reads AMR data from dataDir, applies filters, and returns matching results.
// For each genus, it tries (in order): SQLite index, parquet partition, monolithic file.
// When no genera are given, it scans the full monolithic amrfinderplus.parquet.
func Query(dataDir string, filters Filters) ([]Result, error) {
	if err := validateAMRSchema(dataDir); err != nil {
		return nil, err
	}

	// Try SQLite indexes first for each genus.
	if len(filters.Genera) > 0 {
		return queryWithIndexes(dataDir, filters)
	}

	// No genus filter — full parquet scan.
	return queryParquet(dataDir, filters)
}

// validateAMRSchema verifies that on-disk AMR parquet data uses the v4.2.5
// schema. Returns nil when no AMR data is present yet (so missing-data errors
// surface from the regular query path with their existing messages).
func validateAMRSchema(dataDir string) error {
	if path := firstAMRParquet(dataDir); path != "" {
		return checkAMRParquet(path)
	}
	return nil
}

// firstAMRParquet returns the path to any AMR parquet file under dataDir,
// preferring the monolithic file over partitions. Returns "" when none exist.
func firstAMRParquet(dataDir string) string {
	monolithic := filepath.Join(dataDir, AMRFileName)
	if _, err := os.Stat(monolithic); err == nil {
		return monolithic
	}

	partDir := filepath.Join(dataDir, PartitionDir)
	entries, err := os.ReadDir(partDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".parquet") {
			return filepath.Join(partDir, e.Name())
		}
	}
	return ""
}

// checkAMRParquet reads parquet column metadata and verifies the v4 schema.
func checkAMRParquet(path string) error {
	cols, err := pq.ColumnNames(path)
	if err != nil {
		return fmt.Errorf("inspecting AMR parquet schema: %w", err)
	}
	for _, c := range cols {
		if c == v4SchemaMarker {
			return nil
		}
	}
	return fmt.Errorf("AMR data at %s uses an older AMRFinderPlus schema "+
		"(missing %q column). Run 'atb fetch --tables amrfinderplus.parquet "+
		"--force --yes' to download the AMRFinderPlus v4.2.5 dataset and "+
		"rebuild its indexes.", filepath.Base(path), v4SchemaMarker)
}

func queryWithIndexes(dataDir string, filters Filters) ([]Result, error) {
	var results []Result
	remaining := filters.Limit

	for _, genus := range filters.Genera {
		var genusResults []Result
		var err error

		genusFilters := filters
		genusFilters.Genera = []string{genus}
		if remaining > 0 {
			genusFilters.Limit = remaining
		}

		if idxPath := IndexPath(dataDir, genus); idxPath != "" {
			genusResults, err = QueryIndex(idxPath, genusFilters)
		} else {
			genusResults, err = queryParquetForGenera(dataDir, []string{genus}, genusFilters)
		}
		if err != nil {
			return nil, err
		}

		results = append(results, genusResults...)
		if remaining > 0 {
			remaining -= len(genusResults)
			if remaining <= 0 {
				break
			}
		}
	}

	return results, nil
}

func queryParquet(dataDir string, filters Filters) ([]Result, error) {
	return queryParquetForGenera(dataDir, filters.Genera, filters)
}

func queryParquetForGenera(dataDir string, genera []string, filters Filters) ([]Result, error) {
	paths := resolvePaths(dataDir, genera)

	var results []Result
	remaining := filters.Limit

	for _, amrPath := range paths {
		rows, err := pq.ReadStreamFiltered[pq.AMRRow](amrPath, func(row pq.AMRRow) bool {
			return matchesFilters(row, filters)
		}, remaining)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", filepath.Base(amrPath), err)
		}

		for _, row := range rows {
			results = append(results, rowToResult(row))
		}

		if remaining > 0 {
			remaining -= len(rows)
			if remaining <= 0 {
				break
			}
		}
	}

	return results, nil
}

func resolvePaths(dataDir string, genera []string) []string {
	if len(genera) == 0 {
		return []string{filepath.Join(dataDir, AMRFileName)}
	}

	monolithic := filepath.Join(dataDir, AMRFileName)
	usedMonolithic := false
	var paths []string

	for _, genus := range genera {
		if partPath := PartitionPath(dataDir, genus); partPath != "" {
			paths = append(paths, partPath)
		} else if !usedMonolithic {
			paths = append(paths, monolithic)
			usedMonolithic = true
		}
	}

	return paths
}

func rowToResult(row pq.AMRRow) Result {
	return Result{
		SampleAccession:           row.Name,
		ProteinID:                 row.ProteinID,
		ContigID:                  row.ContigID,
		Start:                     row.Start,
		Stop:                      row.Stop,
		Strand:                    row.Strand,
		GeneSymbol:                row.GeneSymbol,
		ElementName:               row.ElementName,
		Scope:                     row.Scope,
		ElementType:               row.ElementType,
		ElementSubtype:            row.ElementSubtype,
		Class:                     row.Class,
		Subclass:                  row.Subclass,
		Method:                    row.Method,
		TargetLength:              row.TargetLength,
		ReferenceSequenceLength:   row.ReferenceSequenceLength,
		Coverage:                  row.Coverage,
		Identity:                  row.Identity,
		AlignmentLength:           row.AlignmentLength,
		ClosestReferenceAccession: row.ClosestReferenceAccession,
		ClosestReferenceName:      row.ClosestReferenceName,
		HMMAccession:              row.HMMAccession,
		HMMDescription:            row.HMMDescription,
		HierarchyNode:             row.HierarchyNode,
		Genus:                     row.Genus,
		Species:                   row.Species,
	}
}

func matchesFilters(row pq.AMRRow, f Filters) bool {
	if len(f.Samples) > 0 {
		if _, ok := f.Samples[row.Name]; !ok {
			return false
		}
	}
	if f.Class != "" && !strings.Contains(strings.ToUpper(row.Class), strings.ToUpper(f.Class)) {
		return false
	}
	if f.GenePattern != "" && !matchesPattern(row.GeneSymbol, f.GenePattern) {
		return false
	}
	if f.MinCoverage > 0 && row.Coverage < f.MinCoverage {
		return false
	}
	if f.MinIdentity > 0 && row.Identity < f.MinIdentity {
		return false
	}
	if f.ElementType != "" && !strings.EqualFold(f.ElementType, "all") {
		if !strings.EqualFold(row.ElementType, f.ElementType) {
			return false
		}
	}
	if len(f.Genera) > 0 && !matchesAny(row.Genus, f.Genera) {
		return false
	}
	if len(f.Species) > 0 && !matchesAny(row.Species, f.Species) {
		return false
	}
	return true
}

func matchesAny(value string, candidates []string) bool {
	for _, c := range candidates {
		if strings.EqualFold(value, c) {
			return true
		}
	}
	return false
}

// matchesPattern performs a case-insensitive wildcard match using % as the wildcard character.
func matchesPattern(s, pattern string) bool {
	s = strings.ToLower(s)
	pattern = strings.ToLower(pattern)

	prefix := strings.HasPrefix(pattern, "%")
	suffix := strings.HasSuffix(pattern, "%")

	switch {
	case prefix && suffix:
		inner := pattern[1 : len(pattern)-1]
		return strings.Contains(s, inner)
	case prefix:
		inner := pattern[1:]
		return strings.HasSuffix(s, inner)
	case suffix:
		inner := pattern[:len(pattern)-1]
		return strings.HasPrefix(s, inner)
	default:
		return s == pattern
	}
}
