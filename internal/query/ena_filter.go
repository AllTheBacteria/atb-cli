package query

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

// ENAFileName is the name of the ENA metadata parquet file on disk.
const ENAFileName = "ena_20250506.parquet"

// ENAFilter holds optional filters that require joining against the ENA
// metadata table. It is the subset of Filters that any command can reuse
// without pulling in the full query Filters struct.
type ENAFilter struct {
	Country            string
	Platform           string
	CollectionDateFrom string
	CollectionDateTo   string
}

// Active reports whether any ENA filter is set.
func (f ENAFilter) Active() bool {
	return f.Country != "" ||
		f.Platform != "" ||
		f.CollectionDateFrom != "" ||
		f.CollectionDateTo != ""
}

// ENARecord is the subset of ENA columns exposed in output rows. It is
// deliberately narrow so that enrichment maps stay small even when the
// ENA table has millions of entries.
type ENARecord struct {
	Country            string
	CollectionDate     string
	InstrumentPlatform string
}

// BuildENALookup streams ena_20250506.parquet and returns a map of
// sample_accession -> ENARecord for every row matching the filter.
//
// When keep is non-nil, rows whose sample_accession is not in keep are
// skipped, so callers that already have a result set only materialise
// records they will use. First match wins when a sample has multiple
// ENA rows.
//
// Returns (nil, nil) when the filter is inactive and keep is nil, so
// callers can skip the scan entirely.
func BuildENALookup(dir string, f ENAFilter, keep map[string]struct{}) (map[string]ENARecord, error) {
	if !f.Active() && keep == nil {
		return nil, nil
	}

	var (
		fromTime, toTime time.Time
		haveFrom, haveTo bool
	)
	if f.CollectionDateFrom != "" {
		t, err := time.Parse("2006-01-02", f.CollectionDateFrom)
		if err != nil {
			return nil, fmt.Errorf("invalid --collection-date-from %q: expected YYYY-MM-DD", f.CollectionDateFrom)
		}
		fromTime, haveFrom = t, true
	}
	if f.CollectionDateTo != "" {
		t, err := time.Parse("2006-01-02", f.CollectionDateTo)
		if err != nil {
			return nil, fmt.Errorf("invalid --collection-date-to %q: expected YYYY-MM-DD", f.CollectionDateTo)
		}
		toTime, haveTo = t, true
	}

	enaPath := filepath.Join(dir, ENAFileName)
	out := make(map[string]ENARecord)
	_, err := pq.ReadStreamFiltered[pq.ENARow](enaPath, func(r pq.ENARow) bool {
		if keep != nil {
			if _, ok := keep[r.SampleAccession]; !ok {
				return false
			}
		}
		if f.Country != "" && !strings.EqualFold(r.Country, f.Country) {
			return false
		}
		if f.Platform != "" && !strings.EqualFold(r.InstrumentPlatform, f.Platform) {
			return false
		}
		if haveFrom || haveTo {
			rowStart, rowEnd, ok := parseCollectionDate(r.CollectionDate)
			if !ok {
				return false
			}
			if haveFrom && rowEnd.Before(fromTime) {
				return false
			}
			if haveTo && rowStart.After(toTime) {
				return false
			}
		}
		if _, exists := out[r.SampleAccession]; !exists {
			out[r.SampleAccession] = ENARecord{
				Country:            r.Country,
				CollectionDate:     r.CollectionDate,
				InstrumentPlatform: r.InstrumentPlatform,
			}
		}
		// Returning false tells ReadStreamFiltered to skip collecting this row
		// into its result slice — we already captured what we need in `out`.
		return false
	}, 0)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ENAFileName, err)
	}
	return out, nil
}

// BuildENASampleSet is the set-only variant of BuildENALookup. Retained for
// callers that only need the intersection key set.
func BuildENASampleSet(dir string, f ENAFilter) (map[string]struct{}, error) {
	if !f.Active() {
		return nil, nil
	}
	lookup, err := BuildENALookup(dir, f, nil)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(lookup))
	for k := range lookup {
		set[k] = struct{}{}
	}
	return set, nil
}
