package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/amr"
	"github.com/allthebacteria/atb-cli/internal/match"
	"github.com/allthebacteria/atb-cli/internal/output"
	pq "github.com/allthebacteria/atb-cli/internal/parquet"
	"github.com/allthebacteria/atb-cli/internal/query"
)

func newAMRCmd() *cobra.Command {
	var (
		species     string
		speciesLike string
		genus       string
		elementType string
		class       string
		gene        string
		samples     []string
		sampleFile  string
		hqOnly      bool
		minCoverage float64
		minIdentity float64
		limit       int
		format      string
		outputFile  string
		yes         bool

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

Use --species for a full-species match (e.g. "Escherichia coli"), --species-like
for a wildcard match (e.g. "Campylobacter_D jej%"), or --genus for a broader
genus-level match (e.g. "Escherichia"). --species and --genus accept comma-
separated lists and may be combined.

In --species-like patterns % matches any sequence of characters and _ matches
itself, so GTDB names such as "Campylobacter_D" work as written. A pattern that
starts with % cannot be narrowed to a genus and scans the full dataset.

When no filter is given (--species, --species-like, --genus, --gene, --class),
the full AMR dataset is scanned; you'll be prompted to confirm, or pass --yes to
skip the prompt.

Run 'atb fetch' to download the data before querying.`,
		Example: `  # Get AMR gene hits for E. coli (HQ only)
  atb amr --species "Escherichia coli" --hq-only --limit 100

  # Wildcard species match (GTDB names keep their underscores)
  atb amr --species-like "Campylobacter_D jej%" --hq-only --format tsv

  # Filter by drug class
  atb amr --species "Escherichia coli" --class "BETA-LACTAM"

  # Search for beta-lactamase genes in E. coli
  atb amr --species "Escherichia coli" --gene "bla%"

  # Compare beta-lactam resistance across species
  atb amr --species "Escherichia coli,Klebsiella pneumoniae" --class "BETA-LACTAM"

  # Find a gene across ALL genera (no species filter)
  atb amr --gene "blaCTX-M-15" --limit 100

  # Restrict to a specific sample list (parity with 'atb query --samples')
  atb amr --samples SAMD00093868,SAMD00093869 --type all
  atb amr --sample-file my_samples.txt --hq-only --class "BETA-LACTAM"

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
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := resolveDataDir(cfg)

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

			hasFilter := len(speciesList) > 0 || speciesLike != "" || len(explicitGenera) > 0 ||
				gene != "" || class != "" || len(samples) > 0 || sampleFile != ""
			if !hasFilter && !yes {
				if err := confirmFullAMRScan(dir, cmd.ErrOrStderr()); err != nil {
					return err
				}
			}

			// Check if amrfinderplus.parquet exists
			amrPath := filepath.Join(dir, amr.AMRFileName)
			if _, statErr := os.Stat(amrPath); statErr != nil {
				return fmt.Errorf("AMR data not found — run 'atb fetch' to download %s", amr.AMRFileName)
			}

			// Only the literal part before the first % names the genera a row can
			// belong to, so a pattern opening with % leaves nothing to narrow by.
			if strings.HasPrefix(speciesLike, "%") {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"Note: %q starts with a wildcard, so the full AMR dataset is scanned.\n"+
						"      Anchor the pattern to a genus (e.g. \"Escherichia%%\") for a faster query.\n",
					speciesLike)
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

			explicitSamples, err := collectSampleAccessions(samples, sampleFile)
			if err != nil {
				return err
			}

			var sampleSet map[string]struct{}
			if len(explicitSamples) > 0 {
				sampleSet = explicitSamples
			}
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
					if speciesLike != "" && !match.Like(r.SylphSpecies, speciesLike) {
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
				hqSet := make(map[string]struct{}, len(hqRows))
				for _, r := range hqRows {
					hqSet[r.SampleAccession] = struct{}{}
				}
				sampleSet = intersectSampleSets(sampleSet, hqSet)
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
				SpeciesLike: speciesLike,
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

			// Format priority: --format flag > -o filename extension > config
			// DefaultFormat > TTY-aware default. The filename hint outranks
			// DefaultFormat so a path like results.csv.gz produces CSV without
			// needing --format csv at the call site.
			resolvedFormat := format
			if resolvedFormat == "" {
				resolvedFormat = output.InferFormatFromPath(outputFile)
			}
			if resolvedFormat == "" {
				resolvedFormat = cfg.General.DefaultFormat
			}
			resolvedFormat = output.ResolveFormat(resolvedFormat)

			var w io.Writer = cmd.OutOrStdout()
			if outputFile != "" {
				out, closeFn, err := output.OpenWriter(outputFile)
				if err != nil {
					return err
				}
				defer func() { _ = closeFn() }()
				w = out
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
	cmd.Flags().StringVar(&speciesLike, "species-like", "", "wildcard species match, e.g. \"Campylobacter_D jej%\" (% is the wildcard, _ is literal)")
	cmd.Flags().StringVar(&genus, "genus", "", "filter by genus, e.g. \"Escherichia\" (comma-separated for multiple)")
	cmd.Flags().StringVar(&elementType, "type", "", "element type: amr, stress, virulence, all (default: all types)")
	cmd.Flags().StringVar(&class, "class", "", "filter by drug class (case-insensitive, substring match)")
	cmd.Flags().StringVar(&gene, "gene", "", "filter by gene symbol (supports % wildcards)")
	cmd.Flags().StringSliceVar(&samples, "samples", nil, "comma-separated sample accessions to restrict the query to")
	cmd.Flags().StringVar(&sampleFile, "sample-file", "", "file with one sample accession per line (combined with --samples)")
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
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt for an unfiltered full-dataset scan")

	return cmd
}

// confirmFullAMRScan warns the user that no filters were given and prompts for
// confirmation. On a non-TTY stdin, it returns an error directing them to --yes
// so we never hang in pipelines.
func confirmFullAMRScan(dataDir string, stderr io.Writer) error {
	size := amrDataSize(dataDir)
	sizeStr := "unknown size"
	if size > 0 {
		sizeStr = humanize.Bytes(uint64(size))
	}

	stat, _ := os.Stdin.Stat()
	isTTY := stat != nil && stat.Mode()&os.ModeCharDevice != 0
	if !isTTY {
		return fmt.Errorf("no filters supplied — this would scan the full AMR dataset (%s); pass --yes to confirm, or filter with --species/--genus/--gene/--class", sizeStr)
	}

	fmt.Fprintf(stderr, "No filters supplied. This will scan the full AMR dataset (~%s).\n", sizeStr)
	fmt.Fprintf(stderr, "Continue? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(strings.ToLower(input))
	if choice != "y" && choice != "yes" {
		return fmt.Errorf("aborted")
	}
	return nil
}

// amrDataSize returns the on-disk size of the AMR data — either the partition
// directory if present, or the monolithic parquet. Returns 0 if neither exists.
func amrDataSize(dataDir string) int64 {
	partDir := filepath.Join(dataDir, amr.PartitionDir)
	if entries, err := os.ReadDir(partDir); err == nil {
		var total int64
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			total += info.Size()
		}
		if total > 0 {
			return total
		}
	}
	if info, err := os.Stat(filepath.Join(dataDir, amr.AMRFileName)); err == nil {
		return info.Size()
	}
	return 0
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

// collectSampleAccessions builds a deduplicated set from --samples and
// --sample-file. Returns nil when neither was supplied so callers can
// distinguish "no restriction" from "empty restriction".
func collectSampleAccessions(samples []string, sampleFile string) (map[string]struct{}, error) {
	if len(samples) == 0 && sampleFile == "" {
		return nil, nil
	}
	set := make(map[string]struct{}, len(samples))
	for _, s := range samples {
		s = strings.TrimSpace(s)
		if s != "" {
			set[s] = struct{}{}
		}
	}
	if sampleFile != "" {
		f, err := os.Open(sampleFile)
		if err != nil {
			return nil, fmt.Errorf("opening --sample-file %s: %w", sampleFile, err)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			set[line] = struct{}{}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading --sample-file %s: %w", sampleFile, err)
		}
	}
	return set, nil
}

// intersectSampleSets returns the intersection of two sample sets. A nil
// receiver means "no restriction" — i.e. the other set wins. When both are
// non-nil, the result contains only samples present in both.
func intersectSampleSets(a, b map[string]struct{}) map[string]struct{} {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := make(map[string]struct{}, len(a))
	for s := range a {
		if _, ok := b[s]; ok {
			out[s] = struct{}{}
		}
	}
	return out
}

// amrColumns returns the fixed column order for AMR output. Headers match the
// AMRFinderPlus v4.2.5 TSV verbatim so downstream tooling sees the same names
// regardless of source. When withENA is true, country/collection_date/
// instrument_platform are appended.
func amrColumns(withENA bool) []string {
	cols := []string{
		"Name",
		"Protein id",
		"Contig id",
		"Start",
		"Stop",
		"Strand",
		"Element symbol",
		"Element name",
		"Scope",
		"Type",
		"Subtype",
		"Class",
		"Subclass",
		"Method",
		"Target length",
		"Reference sequence length",
		"% Coverage of reference",
		"% Identity to reference",
		"Alignment length",
		"Closest reference accession",
		"Closest reference name",
		"HMM accession",
		"HMM description",
		"Hierarchy node",
		"genus",
		"species",
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
			"Name":                        r.SampleAccession,
			"Protein id":                  r.ProteinID,
			"Contig id":                   r.ContigID,
			"Start":                       formatInt(r.Start),
			"Stop":                        formatInt(r.Stop),
			"Strand":                      r.Strand,
			"Element symbol":              r.GeneSymbol,
			"Element name":                r.ElementName,
			"Scope":                       r.Scope,
			"Type":                        r.ElementType,
			"Subtype":                     r.ElementSubtype,
			"Class":                       r.Class,
			"Subclass":                    r.Subclass,
			"Method":                      r.Method,
			"Target length":               formatInt(r.TargetLength),
			"Reference sequence length":   formatInt(r.ReferenceSequenceLength),
			"% Coverage of reference":     strconv.FormatFloat(r.Coverage, 'f', 2, 64),
			"% Identity to reference":     strconv.FormatFloat(r.Identity, 'f', 2, 64),
			"Alignment length":            formatInt(r.AlignmentLength),
			"Closest reference accession": r.ClosestReferenceAccession,
			"Closest reference name":      r.ClosestReferenceName,
			"HMM accession":               r.HMMAccession,
			"HMM description":             r.HMMDescription,
			"Hierarchy node":              r.HierarchyNode,
			"genus":                       r.Genus,
			"species":                     r.Species,
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

// formatInt renders an int64 as a string, but emits "" for zero so the AMRFP
// "no value" cells round-trip cleanly through TSV/CSV/JSON output.
func formatInt(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}
