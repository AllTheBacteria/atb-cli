package cli

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	idx "github.com/allthebacteria/atb-cli/internal/index"
	pq "github.com/allthebacteria/atb-cli/internal/parquet"
	"github.com/allthebacteria/atb-cli/internal/output"
	"github.com/allthebacteria/atb-cli/internal/query"
	"github.com/allthebacteria/atb-cli/internal/suggest"
)

func newQueryCmd() *cobra.Command {
	var (
		filterFile string
		columns    []string
		sortBy     string
		sortDesc   bool
		limit      int
		offset     int
		format     string
		outputFile string

		// filter flags
		species            string
		speciesLike        string
		genus              string
		samples            []string
		sampleFile         string
		hqOnly             bool
		minCompleteness    float64
		maxContamination   float64
		minN50             int64
		dataset            string
		hasAssembly        bool
		country            string
		platform           string
		collectionDateFrom string
		collectionDateTo   string
	)

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query the ATB database",
		Long:  "Query bacterial genome metadata from local parquet tables.",
		Example: `  # Get 10 high-quality E. coli genomes
  atb query --species "Escherichia coli" --hq-only --limit 10

  # Filter by quality and sort by N50
  atb query --species "Escherichia coli" --min-completeness 99 --min-n50 200000 --sort-by N50 --sort-desc --limit 20

  # Search by genus with specific columns
  atb query --genus Salmonella --hq-only --columns sample_accession,sylph_species,N50 --limit 20

  # Wildcard species search
  atb query --species-like "Streptococcus%" --hq-only --limit 10

  # Filter by country (requires ENA data)
  atb query --species "Salmonella enterica" --country "United Kingdom" --limit 20

  # Use a TOML filter file for reproducible queries
  atb query --filter my_query.toml

  # Output as CSV to a file
  atb query --species "Escherichia coli" --hq-only --format csv -o results.csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := resolveDataDir(cfg)

			if err := ensureDatabase(dir); err != nil {
				return err
			}

			// Build filters - start from TOML file if provided
			filters := query.Filters{}
			outCfg := query.OutputConfig{}

			if filterFile != "" {
				ff, err := query.LoadFilterFile(filterFile)
				if err != nil {
					return fmt.Errorf("loading filter file: %w", err)
				}
				filters = ff.Filter
				outCfg = ff.Output
			}

			// CLI flags override TOML values
			if cmd.Flags().Changed("species") {
				filters.Species = species
			}
			if cmd.Flags().Changed("species-like") {
				filters.SpeciesLike = speciesLike
			}
			if cmd.Flags().Changed("genus") {
				filters.Genus = genus
			}
			if cmd.Flags().Changed("samples") {
				filters.Samples = samples
			}
			if cmd.Flags().Changed("sample-file") {
				filters.SampleFile = sampleFile
			}
			if cmd.Flags().Changed("hq-only") {
				filters.HQOnly = hqOnly
			}
			if cmd.Flags().Changed("min-completeness") {
				filters.MinCompleteness = minCompleteness
			}
			if cmd.Flags().Changed("max-contamination") {
				filters.MaxContamination = maxContamination
			}
			if cmd.Flags().Changed("min-n50") {
				filters.MinN50 = minN50
			}
			if cmd.Flags().Changed("dataset") {
				filters.Dataset = dataset
			}
			if cmd.Flags().Changed("has-assembly") {
				filters.HasAssembly = hasAssembly
			}
			if cmd.Flags().Changed("country") {
				filters.Country = country
			}
			if cmd.Flags().Changed("platform") {
				filters.Platform = platform
			}
			if cmd.Flags().Changed("collection-date-from") {
				filters.CollectionDateFrom = collectionDateFrom
			}
			if cmd.Flags().Changed("collection-date-to") {
				filters.CollectionDateTo = collectionDateTo
			}

			// Load sample file if specified
			if filters.SampleFile != "" {
				if err := filters.LoadSampleFile(); err != nil {
					return fmt.Errorf("loading sample file: %w", err)
				}
			}

			// Output config overrides
			if cmd.Flags().Changed("columns") {
				outCfg.Columns = columns
			}
			if cmd.Flags().Changed("sort-by") {
				outCfg.SortBy = sortBy
			}
			if cmd.Flags().Changed("sort-desc") {
				outCfg.SortDesc = sortDesc
			}
			if cmd.Flags().Changed("limit") {
				outCfg.Limit = limit
			}
			if cmd.Flags().Changed("offset") {
				outCfg.Offset = offset
			}
			if cmd.Flags().Changed("format") {
				outCfg.Format = format
			}
			if cmd.Flags().Changed("output") {
				outCfg.Output = outputFile
			}

			// Try SQLite index first (much faster).
			var results []query.ResultRow
			var queryErr error
			if idx.Exists(dir) {
				if db, openErr := idx.Open(dir); openErr == nil {
					defer db.Close()
					params := idx.QueryParams{
						Species:          filters.Species,
						SpeciesLike:      filters.SpeciesLike,
						Genus:            filters.Genus,
						HQOnly:           filters.HQOnly,
						MinCompleteness:  filters.MinCompleteness,
						MaxContamination: filters.MaxContamination,
						MinN50:           filters.MinN50,
						Dataset:          filters.Dataset,
						HasAssembly:      filters.HasAssembly,
						Samples:          filters.SampleAccessions(),
						Columns:          outCfg.Columns,
						SortBy:           outCfg.SortBy,
						SortDesc:         outCfg.SortDesc,
						Limit:            outCfg.Limit,
						Offset:           outCfg.Offset,
					}
					// ENA filters (country, platform, date) are not in the index;
					// fall through to parquet scan if any are set.
					if !filters.NeedsENA() {
						idxRows, idxErr := db.Query(params)
						if idxErr == nil {
							results = make([]query.ResultRow, len(idxRows))
							for i, r := range idxRows {
								results[i] = query.ResultRow(r)
							}
							// A species filter that matched nothing on the fast
							// path still deserves a "Did you mean" hint. The goto
							// below skips the parquet fallback (and its
							// suggester), so suggest here from the index's own
							// species list.
							if len(results) == 0 && filters.Species != "" {
								if counts, sErr := db.SpeciesList(0); sErr == nil {
									names := make([]string, len(counts))
									for i, c := range counts {
										names[i] = c.Species
									}
									printSpeciesSuggestions(os.Stderr, filters.Species, names)
								}
							}
							// Skip the parquet scan and post-processing below;
							// index already applied limit/offset/sort.
							goto renderOutput
						}
					}
				}
			}

			if err := ensureParquetTables(dir, query.Plan(filters, outCfg.Columns).Tables); err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Querying parquet files (this may take a moment)...\n")
			results, queryErr = query.Execute(dir, filters, outCfg.Columns)
			if queryErr != nil {
				return fmt.Errorf("query failed: %w", queryErr)
			}
			fmt.Fprintf(os.Stderr, "\r\033[K")

			// Suggest species if no results and a species filter was set
			if len(results) == 0 && filters.Species != "" {
				assemblies, aErr := pq.ReadAll[pq.AssemblyRow](filepath.Join(dir, "assembly.parquet"))
				if aErr == nil {
					speciesSet := make(map[string]struct{}, len(assemblies))
					for _, a := range assemblies {
						if a.SylphSpecies != "" {
							speciesSet[a.SylphSpecies] = struct{}{}
						}
					}
					allSpecies := make([]string, 0, len(speciesSet))
					for s := range speciesSet {
						allSpecies = append(allSpecies, s)
					}
					printSpeciesSuggestions(os.Stderr, filters.Species, allSpecies)
				}
			}

			// Sort if requested
			query.SortResults(results, outCfg.SortBy, outCfg.SortDesc)

			// Apply offset
			if outCfg.Offset > 0 {
				if outCfg.Offset >= len(results) {
					results = nil
				} else {
					results = results[outCfg.Offset:]
				}
			}

			// Apply limit
			if outCfg.Limit > 0 && outCfg.Limit < len(results) {
				results = results[:outCfg.Limit]
			}

		renderOutput:
			fmt.Fprintf(os.Stderr, "%s result(s)\n", humanize.Comma(int64(len(results))))

			// Determine columns
			cols := outCfg.Columns
			if len(cols) == 0 {
				outRows := queryToOutputRows(results)
				cols = output.InferColumns(outRows)
			}

			// Format priority: --format flag (or TOML output.format) > -o
			// filename extension > config DefaultFormat > TTY-aware default.
			resolvedFormat := outCfg.Format
			if resolvedFormat == "" {
				resolvedFormat = output.InferFormatFromPath(outCfg.Output)
			}
			if resolvedFormat == "" {
				resolvedFormat = cfg.General.DefaultFormat
			}
			resolvedFormat = output.ResolveFormat(resolvedFormat)

			// Write output
			outRows := queryToOutputRows(results)

			var w io.Writer = cmd.OutOrStdout()
			if outCfg.Output != "" {
				out, closeFn, err := output.OpenWriter(outCfg.Output)
				if err != nil {
					return err
				}
				defer func() { _ = closeFn() }()
				w = out
			}

			return output.Format(w, outRows, cols, resolvedFormat)
		},
	}

	// Filter flags
	cmd.Flags().StringVar(&filterFile, "filter", "", "TOML filter file")
	cmd.Flags().StringVar(&species, "species", "", "exact species name (case-insensitive)")
	cmd.Flags().StringVar(&speciesLike, "species-like", "", "wildcard species match (use % for wildcards)")
	cmd.Flags().StringVar(&genus, "genus", "", "filter by genus")
	cmd.Flags().StringSliceVar(&samples, "samples", nil, "comma-separated sample accessions")
	cmd.Flags().StringVar(&sampleFile, "sample-file", "", "file with one sample accession per line")
	cmd.Flags().BoolVar(&hqOnly, "hq-only", false, "only return high-quality genomes (HQFilter=PASS)")
	cmd.Flags().Float64Var(&minCompleteness, "min-completeness", 0, "minimum CheckM2 completeness")
	cmd.Flags().Float64Var(&maxContamination, "max-contamination", 0, "maximum CheckM2 contamination")
	cmd.Flags().Int64Var(&minN50, "min-n50", 0, "minimum assembly N50")
	cmd.Flags().StringVar(&dataset, "dataset", "", "filter by dataset name")
	cmd.Flags().BoolVar(&hasAssembly, "has-assembly", false, "only samples with assembly FASTA on OSF")
	cmd.Flags().StringVar(&country, "country", "", "filter by country of origin (ENA metadata)")
	cmd.Flags().StringVar(&platform, "platform", "", "filter by sequencing platform (ENA metadata)")
	cmd.Flags().StringVar(&collectionDateFrom, "collection-date-from", "", "collection date lower bound (YYYY-MM-DD); rows with missing or unparseable dates are excluded")
	cmd.Flags().StringVar(&collectionDateTo, "collection-date-to", "", "collection date upper bound (YYYY-MM-DD); rows with missing or unparseable dates are excluded")

	// Output flags
	cmd.Flags().StringSliceVar(&columns, "columns", nil, "columns to include in output")
	cmd.Flags().StringVar(&sortBy, "sort-by", "", "column to sort by")
	cmd.Flags().BoolVar(&sortDesc, "sort-desc", false, "sort in descending order")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of rows to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "number of rows to skip")
	cmd.Flags().StringVar(&format, "format", "", "output format: tsv, csv, json, table, auto")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "write output to file instead of stdout")

	return cmd
}

// queryToOutputRows converts []query.ResultRow to []output.Row.
// Both types are map[string]string so this is a simple cast loop.
func queryToOutputRows(rows []query.ResultRow) []output.Row {
	out := make([]output.Row, len(rows))
	for i, r := range rows {
		out[i] = output.Row(r)
	}
	return out
}

// printSpeciesSuggestions writes a "Did you mean" hint to w when a species
// filter returned nothing. It surfaces the closest names among candidates —
// e.g. the GTDB-suffixed names stored in the index ("Pseudomonas_E
// fluorescens") that a user typing the NCBI name ("Pseudomonas fluorescens")
// would otherwise never find. It stays silent when there are no candidates.
func printSpeciesSuggestions(w io.Writer, species string, candidates []string) {
	suggestions := suggest.Suggest(species, candidates, 5)
	if len(suggestions) == 0 {
		return
	}
	fmt.Fprintf(w, "No results for species %q. Did you mean:\n", species)
	for _, s := range suggestions {
		fmt.Fprintf(w, "  %s\n", s)
	}
}

// parseCSVURLs reads a CSV or TSV file and extracts download URLs. It prefers
// the aws_url column; when that is absent (e.g. an mlst result file) it falls
// back to sample_accession and expands each bare accession into a full S3
// assembly URL, so downstream download works regardless of which columns the
// source file carries.
func parseCSVURLs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read first line to detect separator
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty file")
	}
	firstLine := scanner.Text()

	// Auto-detect: if the first line contains tabs, use tab; otherwise comma
	sep := ','
	if strings.Contains(firstLine, "\t") {
		sep = '\t'
	}

	// Re-read from start
	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("seeking file: %w", err)
	}

	r := csv.NewReader(f)
	r.Comma = rune(sep)
	r.LazyQuotes = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}

	awsIdx := -1
	sampleIdx := -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "aws_url":
			awsIdx = i
		case "sample_accession":
			sampleIdx = i
		}
	}

	if awsIdx == -1 && sampleIdx == -1 {
		return nil, fmt.Errorf("CSV/TSV file must contain an aws_url or sample_accession column")
	}

	var urls []string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		col := awsIdx
		if col == -1 {
			col = sampleIdx
		}
		if col < len(record) {
			v := strings.TrimSpace(record[col])
			if v != "" {
				// A sample_accession fallback yields a bare ID; turn it into a
				// full S3 assembly URL. Values that already carry a scheme
				// (an aws_url column) pass through untouched.
				if !strings.Contains(v, "://") {
					v = buildAssemblyURL(v)
				}
				urls = append(urls, v)
			}
		}
	}

	return urls, nil
}
