package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/allthebacteria/atb-cli/internal/query"
	"github.com/allthebacteria/atb-cli/internal/runs"
)

// runTable is the mapping table from sequencing run accessions to the sample
// accessions the rest of the database is keyed by.
const runTable = "run.parquet"

// missingRunsListed caps how many unresolved accessions the warning names. A
// run list can hold thousands of entries and the count carries the meaning.
const missingRunsListed = 5

// resolveRunFilter expands a run accession filter into the sample accessions
// those runs belong to and merges them into the sample filter. Runs and samples
// name the same thing two ways, so they combine as a union, the same way
// --samples and --sample-file do.
//
// Resolving here, before the query engine is chosen, keeps the SQLite index and
// the parquet scan on the same filter. It is a no-op when no run filter is set.
func resolveRunFilter(filters *query.Filters, dir string, w io.Writer) error {
	if !filters.HasRunFilter() {
		return nil
	}

	path := filepath.Join(dir, runTable)
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("filtering by run accession needs %s; run `atb fetch` to download it", runTable)
	}

	requested := filters.RunAccessions()
	res, err := runs.Resolve(path, requested)
	if err != nil {
		return fmt.Errorf("reading %s: %w", runTable, err)
	}

	if len(res.Missing) > 0 {
		fmt.Fprintf(w, "Warning: %d of %d run accession(s) are not in %s: %s\n",
			len(res.Missing), len(requested), runTable, formatMissingRuns(res.Missing))
	}

	if len(res.Samples) == 0 {
		return fmt.Errorf("none of the %d run accession(s) are in %s", len(requested), runTable)
	}

	fmt.Fprintf(w, "Resolved %d run accession(s) to %d sample accession(s)\n",
		len(requested)-len(res.Missing), len(res.Samples))

	filters.Samples = append(filters.Samples, res.Samples...)
	return nil
}

// formatMissingRuns renders at most missingRunsListed accessions, noting how
// many were left out.
func formatMissingRuns(missing []string) string {
	if len(missing) <= missingRunsListed {
		return strings.Join(missing, ", ")
	}
	return fmt.Sprintf("%s and %d more",
		strings.Join(missing[:missingRunsListed], ", "), len(missing)-missingRunsListed)
}
