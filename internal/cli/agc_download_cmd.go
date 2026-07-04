package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/agc"
	"github.com/allthebacteria/atb-cli/internal/osf"
	"github.com/allthebacteria/atb-cli/internal/sources"
)

func newAGCDownloadCmd() *cobra.Command {
	var (
		fromFile   string
		outputDir  string
		archiveDir string
		species    string
		osfNode    string
		agcIndex   string
		threads    int
		lineLength int
		gzipLevel  int
		parallel   int
		combine    bool
		dryRun     bool
		keepGoing  bool
		refresh    bool
	)

	cmd := &cobra.Command{
		Use:   "download [accession...]",
		Short: "Download genome FASTA from AGC archives by accession or species",
		Long: `Download assembled-genome FASTA from AGC archives, two ways.

By accession (the default): accessions resolve to AGC archives via a cached
sample->archive map; the needed .agc archives are downloaded (cache-first) to
<data-dir>/agc, then each sample is extracted with the agc binary. Accessions
come from positional arguments, a --from file (a query result with a
sample_accession column, or one accession per line; - for stdin), or piped
stdin.

By species (--species "Escherichia coli"): every batch named
<Species>_global_ordered_* is downloaded and extracted whole — no sample->archive
map needed. Batches are resolved from a separate AGC index (atb_agc_files.tsv),
either downloaded from the OSF node (its published index by default, a live
crawl as fallback) and cached, or read from a local --agc-index file. This is
the bulk "give me all of species X" path.

Run 'atb agc install' once to install the agc binary. By default each sample is
written to <output-dir>/<accession>.fa; use --combine to stream everything to one
file (or stdout).`,
		Example: `  # One sample to the default output directory
  atb agc download SAMD00000344

  # Every Acinetobacter baylyi batch, combined into one FASTA
  atb agc download --species "Acinetobacter baylyi" --combine -o baylyi.fa

  # Pipe a query straight into retrieval
  atb query --species "Escherichia coli" --hq-only --limit 5 --format tsv | \
    atb agc download --from - -o ./ecoli

  # Combine many samples into one gzipped FASTA
  atb agc download --from accessions.txt --combine --gzip 6 -o all.fa.gz

  # Preview which archives would be downloaded
  atb agc download --from accessions.txt --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			errOut := cmd.ErrOrStderr()
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			dataDir := resolveDataDir(cfg)
			archiveDirResolved := agc.ArchiveDir(dataDir, firstNonEmpty(archiveDir, cfg.AGC.ArchiveDir))

			par := parallel
			if par == 0 {
				par = cfg.Download.Parallel
			}

			// Fail fast (before any large download) if agc is missing.
			if _, err := agc.FindBinary(); err != nil {
				return err
			}

			// run is the shared tail both access modes converge on: build the
			// fetch spec, point output at a combined file/stream or a per-sample
			// directory, extract, and report. refs is the R2 table (batch -> OSF
			// {url, md5}) for Mode A and nil for Mode B (which builds URLs from
			// BaseURL). unresolvedCount is only non-zero for Mode B.
			run := func(groups map[string][]string, refs map[string]agc.ArchiveRef, unresolvedCount int) error {
				spec := agc.FetchSpec{
					Combine:    combine,
					ArchiveDir: archiveDirResolved,
					BaseURL:    cfg.AGC.ArchiveBaseURL,
					Refs:       refs,
					Parallel:   par,
					Force:      refresh,
					Options: agc.Options{
						Threads:    threads,
						LineLength: lineLength,
						GzipLevel:  gzipLevel,
					},
				}

				var combinedFile *os.File
				if combine {
					w := cmd.OutOrStdout()
					if outputDir != "" {
						f, err := os.Create(outputDir)
						if err != nil {
							return fmt.Errorf("create output file: %w", err)
						}
						combinedFile = f
						w = f
					}
					spec.Combined = w
				} else {
					spec.OutputDir = outputDir
					if spec.OutputDir == "" {
						spec.OutputDir = cfg.Download.OutputDir
					}
				}

				result, runErr := agc.FetchGenomes(groups, spec)
				if combinedFile != nil {
					if cerr := combinedFile.Close(); cerr != nil && runErr == nil {
						runErr = cerr
					}
				}
				if runErr != nil {
					return runErr
				}

				fmt.Fprintf(errOut, "Completed: %d  Failed: %d  Unresolved: %d\n",
					result.Completed, result.Failed, unresolvedCount)
				for _, e := range result.Errors {
					fmt.Fprintf(errOut, "  error: %s: %s\n", e.Accession, e.Error)
				}
				if result.Failed > 0 || unresolvedCount > 0 {
					return fmt.Errorf("%d sample(s) not retrieved", result.Failed+unresolvedCount)
				}
				return nil
			}

			// Mode A — by species: download and extract whole batches.
			if species != "" {
				if len(args) > 0 || fromFile != "" {
					return fmt.Errorf("--species fetches whole batches by species and cannot be combined with accession arguments or --from")
				}
				node := resolveOSFNode(osfNode, cfg.AGC.OSFNode)
				idx, err := loadAGCIndex(agcIndex, sources.AGCIndexURL, archiveDirResolved, node, refresh)
				if err != nil {
					return err
				}
				refs := agc.SelectBySpecies(idx, species)
				if len(refs) == 0 {
					return fmt.Errorf("no AGC batches found for species %q; names are matched exactly (case-insensitive; spaces and underscores are equivalent)", species)
				}
				groups, byName := agc.WholeArchiveGroups(refs)

				if dryRun {
					var totalMB float64
					fmt.Fprintf(errOut, "(dry-run) species %q: %d batch(es):\n", species, len(refs))
					for _, r := range refs {
						totalMB += r.SizeMB
						fmt.Fprintf(errOut, "  %s.agc  (%.1f MB)\n", r.Name, r.SizeMB)
					}
					fmt.Fprintf(errOut, "  total: %.1f MB\n", totalMB)
					return nil
				}
				return run(groups, byName, 0)
			}

			// Mode B — by accession (default).
			accessions := append([]string{}, args...)
			if fromFile != "" {
				fromAcc, err := readAccessionsFromFile(fromFile)
				if err != nil {
					return fmt.Errorf("reading --from: %w", err)
				}
				accessions = append(accessions, fromAcc...)
			}
			if len(accessions) == 0 {
				if fi, statErr := os.Stdin.Stat(); statErr == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
					piped, err := readAccessionsFromFile("-")
					if err != nil {
						return err
					}
					accessions = append(accessions, piped...)
				}
			}
			accessions = dedupeStrings(accessions)
			if len(accessions) == 0 {
				return fmt.Errorf("no accessions given; pass them as arguments, via --from <file>, pipe them to stdin, or use --species for a whole-species fetch")
			}

			groups, unresolved, err := agc.ResolveArchives(dataDir, cfg.AGC.ArchiveMapURL, accessions, refresh)
			if err != nil {
				return err
			}
			for _, u := range unresolved {
				fmt.Fprintf(errOut, "warning: %s: %s\n", u.Accession, u.Reason)
			}
			if !keepGoing && len(unresolved) > 0 {
				return fmt.Errorf("%d accession(s) not found in the archive map", len(unresolved))
			}

			if dryRun {
				fmt.Fprintf(errOut, "(dry-run) %d archive(s) for %d sample(s):\n", len(groups), countSamples(groups))
				for archive, accs := range groups {
					fmt.Fprintf(errOut, "  %s.agc  (%d sample(s))\n", archive, len(accs))
				}
				if n := len(unresolved); n > 0 {
					return fmt.Errorf("%d accession(s) unresolved", n)
				}
				return nil
			}

			return run(groups, nil, len(unresolved))
		},
	}

	cmd.Flags().StringVar(&fromFile, "from", "", "CSV/TSV with a sample_accession column, or one accession per line (- for stdin)")
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "per-sample output directory; with --combine, the single output file (default stdout)")
	cmd.Flags().StringVar(&archiveDir, "archive-dir", "", "directory to cache .agc archives (default <data-dir>/agc)")
	cmd.Flags().StringVar(&species, "species", "", "fetch every batch of this species (whole-batch mode; no accession map needed)")
	cmd.Flags().StringVar(&osfNode, "osf-node", "", "OSF node to crawl for the by-species index (default from config or the staging node)")
	cmd.Flags().StringVar(&agcIndex, "agc-index", "", "local AGC index TSV for --species (default: download the published OSF index, else crawl)")
	cmd.Flags().IntVarP(&threads, "threads", "t", 0, "agc threads (default: all cores minus one)")
	cmd.Flags().IntVar(&lineLength, "line-length", 0, "FASTA line wrap length (default: agc's 80)")
	cmd.Flags().IntVar(&gzipLevel, "gzip", 0, "gzip output at this level (0 = uncompressed)")
	cmd.Flags().IntVarP(&parallel, "parallel", "p", 0, "parallel archive downloads (default from config)")
	cmd.Flags().BoolVar(&combine, "combine", false, "write all samples to one stream/file instead of per-sample files")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "resolve and list archives without downloading or extracting")
	cmd.Flags().BoolVar(&keepGoing, "keep-going", true, "continue past unresolved/failed samples (still exits non-zero if any)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "re-download the archive map and archives even if cached")

	// The staging-node override is for pre-release validation only.
	_ = cmd.Flags().MarkHidden("osf-node")

	return cmd
}

// useHostedAGCIndex reports whether the by-species index should be downloaded
// from the published OSF file rather than crawled live: only when there is no
// local --agc-index override, a hosted URL is configured, and the node is the
// default. An explicit --osf-node override targets a specific node, so it must
// crawl that node instead of reusing the default node's published snapshot.
func useHostedAGCIndex(localPath, indexURL, node string) bool {
	return localPath == "" && indexURL != "" && node == sources.AGCTestNodeID
}

// loadAGCIndex returns the by-species AGC index for Mode A. A non-empty
// localPath is read and parsed directly (the committed atb_agc_files.tsv, or any
// offline copy). Otherwise, when a hosted index URL is configured for the default
// node, the published TSV is downloaded (like the master index); failing that,
// the OSF node's agc_batches/ folder is crawled. Both network paths cache under
// cacheDir as atb_agc_files.tsv (refresh bypasses a fresh cache).
func loadAGCIndex(localPath, indexURL, cacheDir, node string, refresh bool) (*osf.Index, error) {
	if localPath != "" {
		f, err := os.Open(localPath)
		if err != nil {
			return nil, fmt.Errorf("open --agc-index: %w", err)
		}
		defer f.Close()
		return osf.ParseIndex(f)
	}
	if useHostedAGCIndex(localPath, indexURL, node) {
		return osf.FetchAGCIndexFromURL(cacheDir, indexURL, refresh)
	}
	return osf.FetchAGCIndex(cacheDir, sources.OSFNodeFilesURL(node), node, refresh)
}
