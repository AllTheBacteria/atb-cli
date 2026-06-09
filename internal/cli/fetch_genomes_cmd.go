package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/agc"
)

func newFetchGenomesCmd() *cobra.Command {
	var (
		fromFile   string
		outputDir  string
		archiveDir string
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
		Use:     "fetch-genomes [accession...]",
		Aliases: []string{"genomes"},
		Short:   "Fetch genome FASTA for ATB accessions from AGC archives",
		Long: `Fetch assembled-genome FASTA for one or more ATB sample accessions.

Accessions resolve to AGC archives via a cached sample->archive map; the
needed .agc archives are downloaded (cache-first) to <data-dir>/agc, then each
sample is extracted with the agc binary. Run 'atb agc install' once to install
that binary.

Accessions come from positional arguments, a --from file (a query result with a
sample_accession column, or one accession per line; - for stdin), or piped
stdin. By default each sample is written to <output-dir>/<accession>.fa; use
--combine to stream all samples to one file (or stdout).`,
		Example: `  # One sample to the default output directory
  atb fetch-genomes SAMD00000344

  # Pipe a query straight into retrieval
  atb query --species "Escherichia coli" --hq-only --limit 5 --format tsv | \
    atb fetch-genomes --from - -o ./ecoli

  # Combine many samples into one gzipped FASTA
  atb fetch-genomes --from accessions.txt --combine --gzip 6 -o all.fa.gz

  # Preview which archives would be downloaded
  atb fetch-genomes --from accessions.txt --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			errOut := cmd.ErrOrStderr()
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			dataDir := resolveDataDir(cfg)

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
				return fmt.Errorf("no accessions given; pass them as arguments, via --from <file>, or pipe them to stdin")
			}

			// Fail fast (before the large map download) if agc is missing.
			if _, err := agc.FindBinary(); err != nil {
				return err
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

			par := parallel
			if par == 0 {
				par = cfg.Download.Parallel
			}
			spec := agc.FetchSpec{
				Combine:    combine,
				ArchiveDir: agc.ArchiveDir(dataDir, firstNonEmpty(archiveDir, cfg.AGC.ArchiveDir)),
				BaseURL:    cfg.AGC.ArchiveBaseURL,
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
				result.Completed, result.Failed, len(unresolved))
			for _, e := range result.Errors {
				fmt.Fprintf(errOut, "  error: %s: %s\n", e.Accession, e.Error)
			}

			if result.Failed > 0 || len(unresolved) > 0 {
				return fmt.Errorf("%d sample(s) not retrieved", result.Failed+len(unresolved))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&fromFile, "from", "", "CSV/TSV with a sample_accession column, or one accession per line (- for stdin)")
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "per-sample output directory; with --combine, the single output file (default stdout)")
	cmd.Flags().StringVar(&archiveDir, "archive-dir", "", "directory to cache .agc archives (default <data-dir>/agc)")
	cmd.Flags().IntVarP(&threads, "threads", "t", 0, "agc threads (default: all cores minus one)")
	cmd.Flags().IntVar(&lineLength, "line-length", 0, "FASTA line wrap length (default: agc's 80)")
	cmd.Flags().IntVar(&gzipLevel, "gzip", 0, "gzip output at this level (0 = uncompressed)")
	cmd.Flags().IntVarP(&parallel, "parallel", "p", 0, "parallel archive downloads (default from config)")
	cmd.Flags().BoolVar(&combine, "combine", false, "write all samples to one stream/file instead of per-sample files")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "resolve and list archives without downloading or extracting")
	cmd.Flags().BoolVar(&keepGoing, "keep-going", true, "continue past unresolved/failed samples (still exits non-zero if any)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "re-download the archive map and archives even if cached")

	return cmd
}
