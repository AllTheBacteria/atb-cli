package query

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

// ResultRow is a single query result as a map of column name to string value.
type ResultRow map[string]string

// sampleData holds all joined data for a single sample.
type sampleData struct {
	assembly      pq.AssemblyRow
	checkm2       *pq.CheckM2Row
	assemblyStats *pq.AssemblyStatsRow
	ena           *pq.ENARow
}

// Execute runs a full query against the parquet data directory and returns matching rows.
func Execute(dataDir string, filters Filters, columns []string) ([]ResultRow, error) {
	plan := Plan(filters, columns)

	tableSet := make(map[string]bool, len(plan.Tables))
	for _, t := range plan.Tables {
		tableSet[t] = true
	}

	// Step 1: read and filter assembly table
	sampleSet := filters.SampleSet()
	assemblies, err := pq.ReadFiltered[pq.AssemblyRow](
		filepath.Join(dataDir, "assembly.parquet"),
		func(row pq.AssemblyRow) bool {
			if filters.HQOnly && row.HQFilter != "PASS" {
				return false
			}
			if !filters.MatchesSpecies(row.SylphSpecies) {
				return false
			}
			if !filters.MatchesSpeciesLike(row.SylphSpecies) {
				return false
			}
			if filters.Genus != "" && !strings.EqualFold(pq.GenusFromSpecies(row.SylphSpecies), filters.Genus) {
				return false
			}
			if filters.Dataset != "" && !strings.EqualFold(row.Dataset, filters.Dataset) {
				return false
			}
			if filters.HasAssembly && row.AsmFastaOnOSF == 0 {
				return false
			}
			if filters.HasSampleFilter() {
				if _, ok := sampleSet[row.SampleAccession]; !ok {
					return false
				}
			}
			return true
		},
	)
	if err != nil {
		return nil, fmt.Errorf("reading assembly: %w", err)
	}

	// Step 2: build lookup by sample_accession
	lookup := make(map[string]*sampleData, len(assemblies))
	for _, a := range assemblies {
		a := a
		lookup[a.SampleAccession] = &sampleData{assembly: a}
	}

	// Step 3: join checkm2 if needed
	if tableSet["checkm2"] {
		ckRows, err := pq.ReadAll[pq.CheckM2Row](filepath.Join(dataDir, "checkm2.parquet"))
		if err != nil {
			return nil, fmt.Errorf("reading checkm2: %w", err)
		}
		for _, ck := range ckRows {
			ck := ck
			sd, ok := lookup[ck.SampleAccession]
			if !ok {
				continue
			}
			sd.checkm2 = &ck
		}
		// Filter by completeness / contamination
		for accession, sd := range lookup {
			if sd.checkm2 == nil {
				if filters.MinCompleteness > 0 || filters.MaxContamination > 0 {
					delete(lookup, accession)
				}
				continue
			}
			if filters.MinCompleteness > 0 && sd.checkm2.CompletenessGeneral < filters.MinCompleteness {
				delete(lookup, accession)
				continue
			}
			if filters.MaxContamination > 0 && sd.checkm2.Contamination > filters.MaxContamination {
				delete(lookup, accession)
			}
		}
	}

	// Step 4: join assembly_stats if needed
	if tableSet["assembly_stats"] {
		statsRows, err := pq.ReadAll[pq.AssemblyStatsRow](filepath.Join(dataDir, "assembly_stats.parquet"))
		if err != nil {
			return nil, fmt.Errorf("reading assembly_stats: %w", err)
		}
		for _, s := range statsRows {
			s := s
			sd, ok := lookup[s.SampleAccession]
			if !ok {
				continue
			}
			sd.assemblyStats = &s
		}
		// Filter by N50
		if filters.MinN50 > 0 {
			for accession, sd := range lookup {
				if sd.assemblyStats == nil || sd.assemblyStats.N50 < filters.MinN50 {
					delete(lookup, accession)
				}
			}
		}
	}

	// Step 5: join ENA if needed
	if tableSet["ena_20250506"] {
		enaRows, err := pq.ReadAll[pq.ENARow](filepath.Join(dataDir, "ena_20250506.parquet"))
		if err != nil {
			return nil, fmt.Errorf("reading ena_20250506: %w", err)
		}
		for _, e := range enaRows {
			e := e
			sd, ok := lookup[e.SampleAccession]
			if !ok {
				continue
			}
			sd.ena = &e
		}
		// Filter by ENA metadata. Parse the date bounds once so each row only
		// pays the cost of parsing its own collection_date.
		var (
			fromTime, toTime time.Time
			haveFrom, haveTo bool
		)
		if filters.CollectionDateFrom != "" {
			if t, err := time.Parse("2006-01-02", filters.CollectionDateFrom); err == nil {
				fromTime, haveFrom = t, true
			} else {
				return nil, fmt.Errorf("invalid --collection-date-from %q: expected YYYY-MM-DD", filters.CollectionDateFrom)
			}
		}
		if filters.CollectionDateTo != "" {
			if t, err := time.Parse("2006-01-02", filters.CollectionDateTo); err == nil {
				toTime, haveTo = t, true
			} else {
				return nil, fmt.Errorf("invalid --collection-date-to %q: expected YYYY-MM-DD", filters.CollectionDateTo)
			}
		}

		for accession, sd := range lookup {
			if sd.ena == nil {
				if filters.NeedsENA() {
					delete(lookup, accession)
				}
				continue
			}
			if filters.Country != "" && !strings.EqualFold(sd.ena.Country, filters.Country) {
				delete(lookup, accession)
				continue
			}
			if filters.Platform != "" && !strings.EqualFold(sd.ena.InstrumentPlatform, filters.Platform) {
				delete(lookup, accession)
				continue
			}
			if haveFrom || haveTo {
				rowStart, rowEnd, ok := parseCollectionDate(sd.ena.CollectionDate)
				if !ok {
					delete(lookup, accession)
					continue
				}
				if haveFrom && rowEnd.Before(fromTime) {
					delete(lookup, accession)
					continue
				}
				if haveTo && rowStart.After(toTime) {
					delete(lookup, accession)
					continue
				}
			}
		}
	}

	// Step 6: build result rows
	results := make([]ResultRow, 0, len(lookup))
	for _, sd := range lookup {
		row := buildRow(sd)
		results = append(results, row)
	}

	// Step 7: filter to requested columns
	if len(columns) > 0 {
		colSet := make(map[string]bool, len(columns))
		for _, c := range columns {
			colSet[c] = true
		}
		for i, row := range results {
			filtered := make(ResultRow, len(columns))
			for _, c := range columns {
				filtered[c] = row[c]
			}
			results[i] = filtered
		}
		_ = colSet
	}

	return results, nil
}

// SortResults sorts rows in place by the given column. If sortBy is empty, it
// is a no-op. Numeric values are compared numerically; everything else falls
// back to lexicographic string comparison.
func SortResults(rows []ResultRow, sortBy string, desc bool) {
	if sortBy == "" {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i][sortBy], rows[j][sortBy]
		na, errA := strconv.ParseFloat(a, 64)
		nb, errB := strconv.ParseFloat(b, 64)
		if errA == nil && errB == nil {
			if desc {
				return na > nb
			}
			return na < nb
		}
		if desc {
			return a > b
		}
		return a < b
	})
}

// parseCollectionDate interprets an ENA collection_date string as the inclusive
// [start, end] range it represents. ENA dates are often partial: "2020" covers
// the whole year, "2020-05" covers the whole month. Full timestamps are
// truncated to their date component. ISO 8601 interval notation (start/end) is
// also supported.
func parseCollectionDate(s string) (start, end time.Time, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, time.Time{}, false
	}
	if parts := strings.SplitN(s, "/", 3); len(parts) == 2 {
		ls, _, lok := parseSingleDate(parts[0])
		_, re, rok := parseSingleDate(parts[1])
		if !lok || !rok {
			return time.Time{}, time.Time{}, false
		}
		return ls, re, true
	}
	return parseSingleDate(s)
}

func parseSingleDate(s string) (start, end time.Time, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, time.Time{}, false
	}
	if i := strings.IndexAny(s, "T "); i >= 0 {
		s = s[:i]
	}
	switch len(s) {
	case 4:
		if t, err := time.Parse("2006", s); err == nil {
			return t, t.AddDate(1, 0, -1), true
		}
	case 7:
		if t, err := time.Parse("2006-01", s); err == nil {
			return t, t.AddDate(0, 1, -1), true
		}
	case 10:
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t, t, true
		}
	}
	return time.Time{}, time.Time{}, false
}

func buildRow(sd *sampleData) ResultRow {
	row := ResultRow{
		"sample_accession":   sd.assembly.SampleAccession,
		"run_accession":      sd.assembly.RunAccession,
		"assembly_accession": sd.assembly.AssemblyAccession,
		"sylph_species":      sd.assembly.SylphSpecies,
		"hq_filter":          sd.assembly.HQFilter,
		"asm_fasta_on_osf":   strconv.FormatInt(sd.assembly.AsmFastaOnOSF, 10),
		"dataset":            sd.assembly.Dataset,
		"scientific_name":    sd.assembly.ScientificName,
		"aws_url":            sd.assembly.AWSUrl,
		"osf_tarball_url":    sd.assembly.OSFTarballURL,
	}

	if sd.checkm2 != nil {
		row["Completeness_General"] = strconv.FormatFloat(sd.checkm2.CompletenessGeneral, 'f', -1, 64)
		row["Contamination"] = strconv.FormatFloat(sd.checkm2.Contamination, 'f', -1, 64)
		row["Completeness_Specific"] = strconv.FormatFloat(sd.checkm2.CompletenessSpecific, 'f', -1, 64)
		row["Genome_Size"] = strconv.FormatFloat(sd.checkm2.GenomeSize, 'f', -1, 64)
		row["GC_Content"] = strconv.FormatFloat(sd.checkm2.GCContent, 'f', -1, 64)
	}

	if sd.assemblyStats != nil {
		row["total_length"] = strconv.FormatInt(sd.assemblyStats.TotalLength, 10)
		row["number"] = strconv.FormatInt(sd.assemblyStats.Number, 10)
		row["mean_length"] = strconv.FormatFloat(sd.assemblyStats.MeanLength, 'f', -1, 64)
		row["longest"] = strconv.FormatInt(sd.assemblyStats.Longest, 10)
		row["shortest"] = strconv.FormatInt(sd.assemblyStats.Shortest, 10)
		row["N50"] = strconv.FormatInt(sd.assemblyStats.N50, 10)
		row["N90"] = strconv.FormatInt(sd.assemblyStats.N90, 10)
	}

	if sd.ena != nil {
		row["country"] = sd.ena.Country
		row["collection_date"] = sd.ena.CollectionDate
		row["instrument_platform"] = sd.ena.InstrumentPlatform
		row["instrument_model"] = sd.ena.InstrumentModel
		row["read_count"] = strconv.FormatInt(sd.ena.ReadCount, 10)
		row["base_count"] = strconv.FormatInt(sd.ena.BaseCount, 10)
		row["library_strategy"] = sd.ena.LibraryStrategy
		row["study_accession"] = sd.ena.StudyAccession
		row["fastq_ftp"] = sd.ena.FastqFTP
	}

	return row
}
