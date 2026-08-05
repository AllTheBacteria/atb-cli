// Package runs maps sequencing run accessions (ERR/SRR/DRR) onto the sample
// accessions the rest of AllTheBacteria is keyed by.
package runs

import (
	"slices"
	"strings"

	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

// Resolution is the outcome of mapping run accessions onto sample accessions.
type Resolution struct {
	// Samples are the distinct sample accessions the runs belong to, sorted.
	Samples []string
	// Missing are the requested run accessions the mapping table does not
	// list, in the order they were requested.
	Missing []string
}

// Resolve maps run accessions to sample accessions using the run.parquet
// mapping table at path.
//
// A run may name more than one sample. The table records those as a
// comma-joined list in a single field, so every field is split. Rows are not
// filtered on the table's pass column: resolving an accession is a lookup, and
// quality filtering belongs to the query flags.
//
// An empty request returns an empty resolution without opening the file.
func Resolve(path string, runAccessions []string) (Resolution, error) {
	wanted := make(map[string]struct{}, len(runAccessions))
	for _, r := range runAccessions {
		if r = strings.TrimSpace(r); r != "" {
			wanted[r] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return Resolution{}, nil
	}

	rows, err := pq.ReadStreamFiltered[pq.RunRow](path, func(r pq.RunRow) bool {
		_, ok := wanted[r.RunAccession]
		return ok
	}, 0)
	if err != nil {
		return Resolution{}, err
	}

	found := make(map[string]struct{}, len(rows))
	samples := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		found[row.RunAccession] = struct{}{}
		for _, s := range strings.Split(row.SampleAccession, ",") {
			if s = strings.TrimSpace(s); s != "" {
				samples[s] = struct{}{}
			}
		}
	}

	res := Resolution{Samples: make([]string, 0, len(samples))}
	for s := range samples {
		res.Samples = append(res.Samples, s)
	}
	slices.Sort(res.Samples)

	seen := make(map[string]struct{}, len(runAccessions))
	for _, r := range runAccessions {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, dup := seen[r]; dup {
			continue
		}
		seen[r] = struct{}{}
		if _, ok := found[r]; !ok {
			res.Missing = append(res.Missing, r)
		}
	}

	return res, nil
}
