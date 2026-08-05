package query

import (
	"bufio"
	"os"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/allthebacteria/atb-cli/internal/match"
)

// FilterFile is the top-level structure for a TOML filter file.
type FilterFile struct {
	Filter Filters      `toml:"filter"`
	Output OutputConfig `toml:"output"`
}

// Filters holds all query filter criteria.
type Filters struct {
	Species            string   `toml:"species"`
	SpeciesLike        string   `toml:"species_like"`
	Genus              string   `toml:"genus"`
	Samples            []string `toml:"samples"`
	SampleFile         string   `toml:"sample_file"`
	HQOnly             bool     `toml:"hq_only"`
	MinCompleteness    float64  `toml:"min_completeness"`
	MaxContamination   float64  `toml:"max_contamination"`
	MinN50             int64    `toml:"min_n50"`
	Dataset            string   `toml:"dataset"`
	HasAssembly        bool     `toml:"has_assembly"`
	Country            string   `toml:"country"`
	Platform           string   `toml:"platform"`
	CollectionDateFrom string   `toml:"collection_date_from"`
	CollectionDateTo   string   `toml:"collection_date_to"`

	// sampleFileEntries holds accessions loaded from SampleFile.
	sampleFileEntries []string
}

// OutputConfig controls how query results are formatted and written.
type OutputConfig struct {
	Columns  []string `toml:"columns"`
	SortBy   string   `toml:"sort_by"`
	SortDesc bool     `toml:"sort_desc"`
	Limit    int      `toml:"limit"`
	Offset   int      `toml:"offset"`
	Format   string   `toml:"format"`
	Output   string   `toml:"output"`
}

// LoadFilterFile parses a TOML filter file at the given path.
func LoadFilterFile(path string) (*FilterFile, error) {
	var ff FilterFile
	if _, err := toml.DecodeFile(path, &ff); err != nil {
		return nil, err
	}
	return &ff, nil
}

// NeedsCheckM2 reports whether CheckM2 quality metrics are required.
func (f *Filters) NeedsCheckM2() bool {
	return f.MinCompleteness > 0 || f.MaxContamination > 0
}

// NeedsAssemblyStats reports whether assembly statistics are required.
func (f *Filters) NeedsAssemblyStats() bool {
	return f.MinN50 > 0
}

// NeedsENA reports whether ENA metadata (country, platform, dates) is required.
func (f *Filters) NeedsENA() bool {
	return f.Country != "" || f.Platform != "" || f.CollectionDateFrom != "" || f.CollectionDateTo != ""
}

// NeedsSylph is reserved for future use and always returns false.
func (f *Filters) NeedsSylph() bool {
	return false
}

// MatchesSpecies performs a case-insensitive exact match against the Species filter.
// An empty filter matches everything.
func (f *Filters) MatchesSpecies(species string) bool {
	if f.Species == "" {
		return true
	}
	return strings.EqualFold(f.Species, species)
}

// MatchesSpeciesLike performs a wildcard match using % against the SpeciesLike
// filter. An empty filter matches everything.
func (f *Filters) MatchesSpeciesLike(species string) bool {
	if f.SpeciesLike == "" {
		return true
	}
	return match.Like(species, f.SpeciesLike)
}

// LoadSampleFile reads sample accessions from the file referenced by SampleFile.
// Lines starting with # and blank lines are skipped.
func (f *Filters) LoadSampleFile() error {
	file, err := os.Open(f.SampleFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var entries []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries = append(entries, line)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	f.sampleFileEntries = entries
	return nil
}

// HasSampleFilter reports whether any sample filter is active.
func (f *Filters) HasSampleFilter() bool {
	return len(f.Samples) > 0 || len(f.sampleFileEntries) > 0
}

// SampleSet returns a deduplicated set of all sample accessions from both
// the Samples slice and any loaded sample file.
func (f *Filters) SampleSet() map[string]struct{} {
	all := make([]string, 0, len(f.Samples)+len(f.sampleFileEntries))
	all = append(all, f.Samples...)
	all = append(all, f.sampleFileEntries...)

	set := make(map[string]struct{}, len(all))
	for _, s := range all {
		set[s] = struct{}{}
	}
	return set
}

// SampleAccessions returns the deduplicated union of the Samples slice and any
// accessions loaded from a sample file, preserving first-seen order. It is the
// slice counterpart to SampleSet, suitable for building SQL IN clauses.
func (f *Filters) SampleAccessions() []string {
	seen := make(map[string]struct{}, len(f.Samples)+len(f.sampleFileEntries))
	out := make([]string, 0, len(f.Samples)+len(f.sampleFileEntries))
	for _, list := range [][]string{f.Samples, f.sampleFileEntries} {
		for _, s := range list {
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
