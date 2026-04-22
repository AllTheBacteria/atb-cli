package summarise

import (
	"fmt"
	"io"
	"sort"

	"github.com/dustin/go-humanize"

	"github.com/allthebacteria/atb-cli/internal/query"
)

// Summary holds aggregate statistics for a collection of genome rows.
type Summary struct {
	Total      int
	HQCount    int
	TopSpecies []GroupCount
	Datasets   []GroupCount
}

// GroupCount holds a dimension value and the number of rows with that value.
type GroupCount struct {
	Value string
	Count int
}

// DefaultSummary computes aggregate statistics over a slice of result rows.
func DefaultSummary(rows []query.ResultRow) Summary {
	var hq int
	for _, r := range rows {
		if r["hq_filter"] == "PASS" {
			hq++
		}
	}

	return Summary{
		Total:      len(rows),
		HQCount:    hq,
		TopSpecies: GroupBy(rows, "sylph_species"),
		Datasets:   GroupBy(rows, "dataset"),
	}
}

// GroupBy groups rows by the given dimension column and returns counts sorted
// descending by count, then alphabetically by value for ties.
func GroupBy(rows []query.ResultRow, dimension string) []GroupCount {
	counts := make(map[string]int)
	for _, r := range rows {
		v := r[dimension]
		counts[v]++
	}

	out := make([]GroupCount, 0, len(counts))
	for v, c := range counts {
		out = append(out, GroupCount{Value: v, Count: c})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})

	return out
}

// PrintSummary writes a formatted default summary report to w.
// topN controls how many top species and datasets are shown.
func PrintSummary(w io.Writer, s Summary, topN int) {
	fmt.Fprintf(w, "Total genomes:      %s\n", humanize.Comma(int64(s.Total)))
	fmt.Fprintf(w, "High-quality (HQ):  %s", humanize.Comma(int64(s.HQCount)))
	if s.Total > 0 {
		pct := float64(s.HQCount) / float64(s.Total) * 100
		fmt.Fprintf(w, " (%.1f%%)", pct)
	}
	fmt.Fprintln(w)

	if len(s.TopSpecies) > 0 {
		fmt.Fprintf(w, "\nTop species:\n")
		limit := topN
		if limit > len(s.TopSpecies) {
			limit = len(s.TopSpecies)
		}
		for _, g := range s.TopSpecies[:limit] {
			name := g.Value
			if name == "" {
				name = "(unclassified)"
			}
			fmt.Fprintf(w, "  %-50s %10s\n", name, humanize.Comma(int64(g.Count)))
		}
	}

	if len(s.Datasets) > 0 {
		fmt.Fprintf(w, "\nDatasets:\n")
		limit := topN
		if limit > len(s.Datasets) {
			limit = len(s.Datasets)
		}
		for _, g := range s.Datasets[:limit] {
			name := g.Value
			if name == "" {
				name = "(unknown)"
			}
			fmt.Fprintf(w, "  %-30s %10s\n", name, humanize.Comma(int64(g.Count)))
		}
	}
}

// PrintGroupBy writes a ranked list of group counts to w.
func PrintGroupBy(w io.Writer, groups []GroupCount, dimension string, topN int) {
	fmt.Fprintf(w, "Group by %q:\n", dimension)
	limit := topN
	if limit > len(groups) {
		limit = len(groups)
	}
	for i, g := range groups[:limit] {
		name := g.Value
		if name == "" {
			name = "(empty)"
		}
		fmt.Fprintf(w, "  %3d. %-50s %10s\n", i+1, name, humanize.Comma(int64(g.Count)))
	}
}
