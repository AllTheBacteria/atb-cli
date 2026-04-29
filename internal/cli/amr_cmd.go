package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/amr"
	"github.com/allthebacteria/atb-cli/internal/output"
	pq "github.com/allthebacteria/atb-cli/internal/parquet"
	"github.com/allthebacteria/atb-cli/internal/query"
)

func newAMRCmd() *cobra.Command {
	var (
		species     string
		genus       string
		elementType string
		class       string
		gene        string
		hqOnly      bool
		minCoverage float64
		minIdentity float64
		limit       int
		format      string
		outputFile  string

		country            string
		platform           string
		collectionDateFrom string
		collectionDateTo   string
		withENA            bool

		downloadFlag bool
		downloadDir  string
		dryRun       bool
		maxSamples   int
	)

	cmd := &cobra.Command{
		Use:   "amr",
		Short: "Query AMR gene data",
		Long: `Query AMRFinderPlus gene hits from the merged amrfinderplus.parquet file.

Use --species for a full-species match (e.g. "Escherichia coli"), or --genus
for a broader genus-level match (e.g. "Escherichia"). Both accept comma-
separated lists and may be combined.

Run 'atb fetch' to download the data before querying.`,
		Example: `  # Get AMR gene hits for E. coli (HQ only)
  atb amr --species "Escherichia coli" --hq-only --limit 100

  # Filter by drug class
  atb amr --species "Escherichia coli" --class "BETA-LACTAM"

  # Search for beta-lactamase genes in E. coli
  atb amr --species "Escherichia coli" --gene "bla%"

  # Compare beta-lactam resistance across species
  atb amr --species "Escherichia coli,Klebsiella pneumoniae" --class "BETA-LACTAM"

  # Find a gene across ALL genera (no species filter)
  atb amr --gene "blaCTX-M-15" --limit 100

  # Query stress response genes
  atb amr --species "Escherichia coli" --type stress

  # Query all element types (AMR + stress + virulence)
  atb amr --species "Klebsiella pneumoniae" --type all --hq-only

  # Download assemblies with beta-lactam resistance
  atb amr --species "Escherichia coli" --class "BETA-LACTAM" --hq-only --download -d ./genomes

  # Preview assemblies that would be downloaded
  atb amr --species "Klebsiella pneumoniae" --gene "blaCTX-M-15" --download --dry-run

  # Filter by ENA metadata (requires ena_20250506.parquet).
  # Any ENA filter also appends country/collection_date/instrument_platform columns.
  atb amr --species "Escherichia coli" --class "BETA-LACTAM" --country "UK" --platform ILLUMINA
  atb amr --species "Salmonella enterica" --gene "blaCTX-M-15" --collection-date-from 2022-01-01

  # Append ENA columns without filtering (requires ena_20250506.parquet)
  atb amr --species "Escherichia coli" --class "BETA-LACTAM" --with-ena`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			speciesList := splitCSV(species)
			explicitGenera := splitCSV(genus)

			// Derive partition genera from species (first whitespace-separated token).
			// These are unioned with explicit --genus values so we know which
			// partition files / SQLite indexes to read.
			generaSet := make(map[string]struct{}, len(speciesList)+len(explicitGenera))
			for _, sp := range speciesList {
				g := pq.GenusFromSpecies(sp)
				if g == "" {
					return fmt.Errorf("could not derive genus from species %q", sp)
				}
				generaSet[g] = struct{}{}
			}
			for _, g := range explicitGenera {
				generaSet[g] = struct{}{}
			}
			genera := make([]string, 0, len(generaSet))
			for g := range generaSet {
				genera = append(genera, g)
			}

			// Require either --species/--genus or --gene/--class to avoid accidental full scans
			if len(genera) == 0 && gene == "" && class == "" {
				return fmt.Errorf("--species or --genus is required (or use --gene/--class to search across all genera)")
			}

			// Check if amrfinderplus.parquet exists
			amrPath := filepath.Join(dir, amr.AMRFileName)
			if _, statErr := os.Stat(amrPath); statErr != nil {
				return fmt.Errorf("AMR data not found — run 'atb fetch' to download %s", amr.AMRFileName)
			}

			enaFilter := query.ENAFilter{
				Country:            country,
				Platform:           platform,
				CollectionDateFrom: collectionDateFrom,
				CollectionDateTo:   collectionDateTo,
			}
			wantENA := enaFilter.Active() || withENA
			if wantENA {
				if err := ensureParquetTables(dir, []string{query.ENAFileName}); err != nil {
					return err
				}
			}

			var sampleSet map[string]struct{}
			if hqOnly {
				fmt.Fprintf(os.Stderr, "Loading HQ sample set...\n")
				assemblyPath := filepath.Join(dir, "assembly.parquet")
				lowerGenera := make(map[string]bool, len(genera))
				for _, g := range genera {
					lowerGenera[strings.ToLower(g)] = true
				}
				lowerSpecies := make(map[string]bool, len(speciesList))
				for _, s := range speciesList {
					lowerSpecies[strings.ToLower(s)] = true
				}
				hqRows, hqErr := pq.ReadStreamFiltered[pq.AssemblyRow](assemblyPath, func(r pq.AssemblyRow) bool {
					if r.HQFilter != "PASS" {
						return false
					}
					if len(lowerSpecies) > 0 {
						return lowerSpecies[strings.ToLower(r.SylphSpecies)]
					}
					if len(lowerGenera) > 0 {
						return lowerGenera[strings.ToLower(pq.GenusFromSpecies(r.SylphSpecies))]
					}
					return true
				}, 0)
				if hqErr != nil {
					return fmt.Errorf("loading HQ samples: %w", hqErr)
				}
				sampleSet = make(map[string]struct{}, len(hqRows))
				for _, r := range hqRows {
					sampleSet[r.SampleAccession] = struct{}{}
				}
			}

			var enaLookup map[string]query.ENARecord
			if enaFilter.Active() {
				fmt.Fprintf(os.Stderr, "Applying ENA metadata filter...\n")
				lookup, enaErr := query.BuildENALookup(dir, enaFilter, nil)
				if enaErr != nil {
					return enaErr
				}
				enaLookup = lookup
				enaSet := make(map[string]struct{}, len(lookup))
				for s := range lookup {
					enaSet[s] = struct{}{}
				}
				if sampleSet == nil {
					sampleSet = enaSet
				} else {
					for s := range sampleSet {
						if _, ok := enaSet[s]; !ok {
							delete(sampleSet, s)
						}
					}
				}
			}

			filters := amr.Filters{
				Samples:     sampleSet,
				Class:       class,
				GenePattern: gene,
				MinCoverage: minCoverage,
				MinIdentity: minIdentity,
				ElementType: elementType,
				Genera:      genera,
				Species:     speciesList,
				Limit:       limit,
			}

			fmt.Fprintf(os.Stderr, "Querying AMR data...\n")
			results, err := amr.Query(dir, filters)
			if err != nil {
				return fmt.Errorf("AMR query failed: %w", err)
			}

			// With --with-ena (and no filter) we scan the ENA table keyed to the
			// distinct sample set in the results, so enrichment cost scales with
			// the result size rather than the full 2.5 GB table.
			if withENA && enaLookup == nil && len(results) > 0 {
				fmt.Fprintf(os.Stderr, "Enriching with ENA metadata...\n")
				keep := make(map[string]struct{}, len(results))
				for _, r := range results {
					keep[r.SampleAccession] = struct{}{}
				}
				lookup, enaErr := query.BuildENALookup(dir, query.ENAFilter{}, keep)
				if enaErr != nil {
					return enaErr
				}
				enaLookup = lookup
			}

			fmt.Fprintf(os.Stderr, "%s result(s)\n", humanize.Comma(int64(len(results))))

			rows := amrResultsToOutputRows(results, enaLookup, wantENA)
			cols := amrColumns(wantENA)

			// Resolve output format
			resolvedFormat := format
			if resolvedFormat == "" {
				resolvedFormat = cfg.General.DefaultFormat
			}
			resolvedFormat = output.ResolveFormat(resolvedFormat)

			var w io.Writer = cmd.OutOrStdout()
			if outputFile != "" {
				f, err := os.Create(outputFile)
				if err != nil {
					return fmt.Errorf("opening output file: %w", err)
				}
				defer f.Close()
				w = f
			}

			if err := output.Format(w, rows, cols, resolvedFormat); err != nil {
				return err
			}

			if downloadFlag && len(results) > 0 {
				accessions := make([]string, len(results))
				for i, r := range results {
					accessions[i] = r.SampleAccession
				}

				outDir := downloadDir
				if outDir == "" {
					outDir = cfg.Download.OutputDir
				}

				return downloadAssemblies(AssemblyDownloadConfig{
					SampleAccessions: accessions,
					OutputDir:        outDir,
					Parallel:         cfg.Download.Parallel,
					DryRun:           dryRun,
					MaxSamples:       maxSamples,
					Force:            false,
					MinFreeSpaceGB:   cfg.Download.MinFreeSpaceGB,
					Stderr:           cmd.ErrOrStderr(),
				})
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&species, "species", "", "filter by full species name, e.g. \"Escherichia coli\" (comma-separated for multiple)")
	cmd.Flags().StringVar(&genus, "genus", "", "filter by genus, e.g. \"Escherichia\" (comma-separated for multiple)")
	cmd.Flags().StringVar(&elementType, "type", "", "element type: amr (default), stress, virulence, all")
	cmd.Flags().StringVar(&class, "class", "", "filter by drug class (case-insensitive, substring match)")
	cmd.Flags().StringVar(&gene, "gene", "", "filter by gene symbol (supports % wildcards)")
	cmd.Flags().BoolVar(&hqOnly, "hq-only", false, "only include HQ samples (hq_filter=PASS)")
	cmd.Flags().Float64Var(&minCoverage, "min-coverage", 0, "minimum coverage %")
	cmd.Flags().Float64Var(&minIdentity, "min-identity", 0, "minimum identity %")
	cmd.Flags().StringVar(&country, "country", "", "filter by ENA country (requires ena_20250506.parquet)")
	cmd.Flags().StringVar(&platform, "platform", "", "filter by ENA instrument platform, e.g. ILLUMINA (requires ena_20250506.parquet)")
	cmd.Flags().StringVar(&collectionDateFrom, "collection-date-from", "", "earliest ENA collection_date, YYYY-MM-DD (requires ena_20250506.parquet); rows with missing or unparseable dates are excluded")
	cmd.Flags().StringVar(&collectionDateTo, "collection-date-to", "", "latest ENA collection_date, YYYY-MM-DD (requires ena_20250506.parquet); rows with missing or unparseable dates are excluded")
	cmd.Flags().BoolVar(&withENA, "with-ena", false, "include country/collection_date/instrument_platform from the ENA table (requires ena_20250506.parquet)")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of results")
	cmd.Flags().StringVar(&format, "format", "", "output format: tsv, csv, json, table, auto")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "write output to file instead of stdout")
	cmd.Flags().BoolVar(&downloadFlag, "download", false, "download FASTA assemblies for matching samples")
	cmd.Flags().StringVarP(&downloadDir, "download-dir", "d", "", "directory to save downloaded assemblies (default from config)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print download URLs without downloading")
	cmd.Flags().IntVar(&maxSamples, "max-samples", 0, "limit number of assemblies to download")

	return cmd
}

// splitCSV trims and splits a comma-separated flag value, dropping empty entries.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// amrColumns returns the fixed column order for AMR output. When withENA is
// true, country/collection_date/instrument_platform are appended.
func amrColumns(withENA bool) []string {
	cols := []string{
		"sample_accession",
		"gene_symbol",
		"element_type",
		"element_subtype",
		"class",
		"subclass",
		"method",
		"coverage",
		"identity",
		"species",
		"genus",
	}
	if withENA {
		cols = append(cols, "country", "collection_date", "instrument_platform")
	}
	return cols
}

func amrResultsToOutputRows(results []amr.Result, enaLookup map[string]query.ENARecord, withENA bool) []output.Row {
	rows := make([]output.Row, len(results))
	for i, r := range results {
		row := output.Row{
			"sample_accession": r.SampleAccession,
			"gene_symbol":      r.GeneSymbol,
			"element_type":     r.ElementType,
			"element_subtype":  r.ElementSubtype,
			"class":            r.Class,
			"subclass":         r.Subclass,
			"method":           r.Method,
			"coverage":         strconv.FormatFloat(r.Coverage, 'f', 2, 64),
			"identity":         strconv.FormatFloat(r.Identity, 'f', 2, 64),
			"species":          r.Species,
			"genus":            r.Genus,
		}
		if withENA {
			rec := enaLookup[r.SampleAccession]
			row["country"] = rec.Country
			row["collection_date"] = rec.CollectionDate
			row["instrument_platform"] = rec.InstrumentPlatform
		}
		rows[i] = row
	}
	return rows
}
