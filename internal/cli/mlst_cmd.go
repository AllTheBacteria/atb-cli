package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	idx "github.com/allthebacteria/atb-cli/internal/index"
	"github.com/allthebacteria/atb-cli/internal/output"
	"github.com/allthebacteria/atb-cli/internal/query"
)

func newMLSTCmd() *cobra.Command {
	var (
		species      string
		sequenceType string
		scheme       string
		mlstStatus   string
		hqOnly       bool
		limit        int
		format       string
		outputFile   string

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
		Use:   "mlst",
		Short: "Query MLST (Multi-Locus Sequence Typing) data for bacterial genomes",
		Example: `  # Get all STs for E. coli
  atb mlst --species "Escherichia coli" --hq-only --limit 20

  # Find ST131 E. coli
  atb mlst --species "Escherichia coli" --st 131

  # Query by scheme name
  atb mlst --scheme salmonella --limit 50

  # Only perfect MLST calls
  atb mlst --species "Escherichia coli" --status PERFECT --limit 20

  # Download assemblies for ST131 E. coli
  atb mlst --species "Escherichia coli" --st 131 --download -d ./st131

  # Preview download, cap at 20 assemblies
  atb mlst --species "Salmonella enterica" --status PERFECT --download --dry-run --max-samples 20

  # Filter MLST results by ENA metadata (requires ena_20250506.parquet).
  # Any ENA filter also appends country/collection_date/instrument_platform columns.
  atb mlst --species "Escherichia coli" --st 131 --country "UK"
  atb mlst --species "Salmonella enterica" --platform ILLUMINA --collection-date-from 2022-01-01 --limit 100

  # Append ENA columns without filtering (requires ena_20250506.parquet)
  atb mlst --species "Escherichia coli" --st 131 --with-ena`,
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

			if err := ensureDatabase(dir); err != nil {
				return err
			}

			db, err := idx.Open(dir)
			if err != nil {
				return fmt.Errorf("opening index: %w", err)
			}
			defer db.Close()

			mlstCols := []string{
				"sample_accession",
				"sylph_species",
				"mlst_scheme",
				"mlst_st",
				"mlst_status",
				"mlst_score",
				"mlst_alleles",
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

			// When ENA filters are active we must pull all matching MLST rows
			// from the index and apply limit after the join; otherwise limit
			// would cut off rows before the ENA intersection.
			queryLimit := limit
			if enaFilter.Active() {
				queryLimit = 0
			}

			fmt.Fprintf(os.Stderr, "Querying MLST data...\n")
			rows, err := db.Query(idx.QueryParams{
				Species:      species,
				HQOnly:       hqOnly,
				SequenceType: sequenceType,
				Scheme:       scheme,
				MLSTStatus:   mlstStatus,
				Columns:      mlstCols,
				Limit:        queryLimit,
				MLSTOnly:     mlstStatus == "",
			})
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}

			// Scan the ENA parquet only when the user asked for ENA data, either
			// via a filter or --with-ena. A filter-driven scan also populates
			// the enrichment map, so we never scan the 2.5 GB table twice.
			var enaLookup map[string]query.ENARecord
			if enaFilter.Active() {
				fmt.Fprintf(os.Stderr, "Applying ENA metadata filter...\n")
				lookup, enaErr := query.BuildENALookup(dir, enaFilter, nil)
				if enaErr != nil {
					return enaErr
				}
				enaLookup = lookup
				filtered := rows[:0]
				for _, r := range rows {
					if _, ok := enaLookup[r["sample_accession"]]; ok {
						filtered = append(filtered, r)
					}
				}
				rows = filtered
				if limit > 0 && len(rows) > limit {
					rows = rows[:limit]
				}
			} else if withENA && len(rows) > 0 {
				fmt.Fprintf(os.Stderr, "Enriching with ENA metadata...\n")
				keep := make(map[string]struct{}, len(rows))
				for _, r := range rows {
					keep[r["sample_accession"]] = struct{}{}
				}
				lookup, enaErr := query.BuildENALookup(dir, query.ENAFilter{}, keep)
				if enaErr != nil {
					return enaErr
				}
				enaLookup = lookup
			}

			fmt.Fprintf(os.Stderr, "%s result(s)\n", humanize.Comma(int64(len(rows))))

			if len(rows) == 0 {
				return nil
			}

			if wantENA {
				mlstCols = append(mlstCols, "country", "collection_date", "instrument_platform")
			}

			outRows := make([]output.Row, len(rows))
			for i, r := range rows {
				outRows[i] = output.Row(r)
				if wantENA {
					rec := enaLookup[r["sample_accession"]]
					outRows[i]["country"] = rec.Country
					outRows[i]["collection_date"] = rec.CollectionDate
					outRows[i]["instrument_platform"] = rec.InstrumentPlatform
				}
			}

			var w io.Writer = cmd.OutOrStdout()
			if outputFile != "" {
				f, ferr := os.Create(outputFile)
				if ferr != nil {
					return fmt.Errorf("opening output file: %w", ferr)
				}
				defer f.Close()
				w = f
			}

			resolvedFormat := format
			if resolvedFormat == "" {
				resolvedFormat = "tsv"
			}

			if err := output.Format(w, outRows, mlstCols, resolvedFormat); err != nil {
				return err
			}

			if downloadFlag && len(rows) > 0 {
				accessions := make([]string, len(rows))
				for i, r := range rows {
					accessions[i] = r["sample_accession"]
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

	cmd.Flags().StringVar(&species, "species", "", "filter by species name (case-insensitive)")
	cmd.Flags().StringVar(&sequenceType, "sequence-type", "", "filter by sequence type (ST number)")
	cmd.Flags().StringVar(&sequenceType, "st", "", "filter by sequence type (shorthand for --sequence-type)")
	cmd.Flags().StringVar(&scheme, "scheme", "", "filter by MLST scheme name")
	cmd.Flags().StringVar(&mlstStatus, "status", "", "filter by MLST status (PERFECT, NOVEL, OK, MIXED, BAD, NONE, MISSING)")
	cmd.Flags().BoolVar(&hqOnly, "hq-only", false, "only include high-quality genomes (hq_filter=PASS)")
	cmd.Flags().StringVar(&country, "country", "", "filter by ENA country (requires ena_20250506.parquet)")
	cmd.Flags().StringVar(&platform, "platform", "", "filter by ENA instrument platform, e.g. ILLUMINA (requires ena_20250506.parquet)")
	cmd.Flags().StringVar(&collectionDateFrom, "collection-date-from", "", "earliest ENA collection_date, YYYY-MM-DD (requires ena_20250506.parquet); rows with missing or unparseable dates are excluded")
	cmd.Flags().StringVar(&collectionDateTo, "collection-date-to", "", "latest ENA collection_date, YYYY-MM-DD (requires ena_20250506.parquet); rows with missing or unparseable dates are excluded")
	cmd.Flags().BoolVar(&withENA, "with-ena", false, "include country/collection_date/instrument_platform from the ENA table (requires ena_20250506.parquet)")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum number of results (0 = unlimited)")
	cmd.Flags().StringVar(&format, "format", "tsv", "output format: tsv, csv, json, table")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "write output to file instead of stdout")
	cmd.Flags().BoolVar(&downloadFlag, "download", false, "download FASTA assemblies for matching samples")
	cmd.Flags().StringVarP(&downloadDir, "download-dir", "d", "", "directory to save downloaded assemblies (default from config)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print download URLs without downloading")
	cmd.Flags().IntVar(&maxSamples, "max-samples", 0, "limit number of assemblies to download")

	return cmd
}
