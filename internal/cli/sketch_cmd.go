package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/config"
	"github.com/allthebacteria/atb-cli/internal/download"
	idx "github.com/allthebacteria/atb-cli/internal/index"
	"github.com/allthebacteria/atb-cli/internal/output"
	"github.com/allthebacteria/atb-cli/internal/sketch"
	"github.com/allthebacteria/atb-cli/internal/sources"
)

func sketchDir(dir string) string {
	return filepath.Join(dir, sources.SketchSubdir)
}

func sketchSkmPath(dir string) string {
	return filepath.Join(sketchDir(dir), sources.SketchSkmFilename)
}

func sketchDbPrefix(dir string) string {
	return filepath.Join(sketchDir(dir), "atb_sketchlib")
}

func sketchDbExists(dir string) bool {
	skm := filepath.Join(sketchDir(dir), sources.SketchSkmFilename)
	skd := filepath.Join(sketchDir(dir), sources.SketchSkdFilename)
	_, err1 := os.Stat(skm)
	_, err2 := os.Stat(skd)
	return err1 == nil && err2 == nil
}

func newSketchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sketch",
		Short: "Find closest ATB genomes using sketch distances",
		Long: `Find the closest genomes in the AllTheBacteria database to your input
sequences using MinHash sketch distances (via sketchlib).

Requires sketchlib (Linux/macOS only). Run 'atb sketch install' to download it.`,
	}

	cmd.AddCommand(newSketchInstallCmd())
	cmd.AddCommand(newSketchFetchCmd())
	cmd.AddCommand(newSketchQueryCmd())
	cmd.AddCommand(newSketchInfoCmd())

	return cmd
}

func newSketchInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Download the sketchlib binary (Linux/macOS only)",
		Long: `Download the sketchlib binary from GitHub releases and install it
alongside the atb binary. This is required for 'atb sketch query'.

Not available on Windows.`,
		Example: `  atb sketch install`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if already installed
			if path, err := sketch.FindBinary(); err == nil {
				fmt.Fprintf(os.Stderr, "sketchlib already installed at %s\n", path)
				return nil
			}

			return sketch.InstallBinary(func(msg string) {
				fmt.Fprintln(os.Stderr, msg)
			})
		},
	}
}

func newSketchFetchCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Download the ATB sketch database from OSF",
		Long: `Download the AllTheBacteria sketch database (~4.2 GB) from OSF.
This is required before running 'atb sketch query'.`,
		Example: `  atb sketch fetch
  atb sketch fetch --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			sDir := sketchDir(dir)

			if !force && sketchDbExists(dir) {
				fmt.Fprintln(os.Stderr, "Sketch database already exists. Use --force to re-download.")
				return nil
			}

			fmt.Fprintln(os.Stderr, "Downloading ATB sketch database (~4.2 GB)...")

			tasks := []download.FileTask{
				{URL: sources.SketchSkmURL, Filename: sources.SketchSkmFilename},
				{URL: sources.SketchSkdURL, Filename: sources.SketchSkdFilename},
			}

			dl := download.New(download.Config{
				OutputDir:  sDir,
				Parallel:   2,
				MaxRetries: 3,
			})

			dl.OnProgress = func(filename string, written, total int64) {
				if total > 0 {
					pct := float64(written) / float64(total) * 100
					fmt.Fprintf(os.Stderr, "\r  %s: %.1f%% (%s / %s)",
						filename, pct, formatSize(written), formatSize(total))
				} else {
					fmt.Fprintf(os.Stderr, "\r  %s: %s", filename, formatSize(written))
				}
			}

			result := dl.DownloadAllFiles(tasks)

			fmt.Fprintf(os.Stderr, "\r\033[K")
			fmt.Fprintf(os.Stderr, "Completed: %d/%d  Bytes: %s\n",
				result.Completed, result.Total, formatSize(result.Bytes))

			if result.Failed > 0 {
				for _, e := range result.Errors {
					fmt.Fprintf(os.Stderr, "  error: %s\n", e.Error)
				}
				return fmt.Errorf("%d download(s) failed", result.Failed)
			}

			fmt.Fprintln(os.Stderr, "Sketch database ready.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "re-download even if database exists")

	return cmd
}

func newSketchQueryCmd() *cobra.Command {
	var (
		fileList    string
		knn         int
		threads     int
		format      string
		raw         bool
		downloadDir string
		dryRun      bool
	)

	cmd := &cobra.Command{
		Use:   "query <fasta> [fasta...]",
		Short: "Find closest ATB genomes for your input sequences",
		Long: `Sketch your input FASTA file(s) and find the closest matches in the
AllTheBacteria sketch database. Results include ANI (Average Nucleotide
Identity) and metadata from the local ATB index.`,
		Example: `  # Find 10 closest genomes (default)
  atb sketch query my_genome.fasta

  # Top 50 closest
  atb sketch query my_genome.fasta --knn 50

  # Multiple input files
  atb sketch query sample1.fasta sample2.fasta

  # Batch from file list (one path per line)
  atb sketch query -f input_list.txt

  # Raw sketchlib distances without metadata enrichment
  atb sketch query my_genome.fasta --raw

  # Find closest and download their assemblies
  atb sketch query my_genome.fasta --download ./closest_genomes

  # Preview downloads without actually downloading
  atb sketch query my_genome.fasta --download ./closest_genomes --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && fileList == "" {
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

			if _, err := sketch.FindBinary(); err != nil {
				return err
			}

			if !sketchDbExists(dir) {
				return fmt.Errorf("sketch database not found. Run 'atb sketch fetch' first")
			}

			inputs := args
			if fileList != "" {
				lines, err := readLines(fileList)
				if err != nil {
					return fmt.Errorf("reading file list: %w", err)
				}
				inputs = append(inputs, lines...)
			}
			if len(inputs) == 0 {
				return fmt.Errorf("no input files provided")
			}

			for _, f := range inputs {
				if _, err := os.Stat(f); err != nil {
					return fmt.Errorf("input file: %w", err)
				}
			}

			dbInfo, err := sketch.Info(sketchSkmPath(dir))
			if err != nil {
				return fmt.Errorf("reading sketch database: %w", err)
			}
			if len(dbInfo.KmerSizes) == 0 {
				return fmt.Errorf("sketch database has no k-mer sizes")
			}

			fmt.Fprintf(os.Stderr, "Sketching %d input file(s)...\n", len(inputs))

			tmpDir, queryPrefix, err := sketch.SketchQuery(inputs, dbInfo.KmerSizes, threads)
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)

			fmt.Fprintf(os.Stderr, "Querying ATB database (%s genomes)...\n", humanize.Comma(int64(dbInfo.Samples)))

			kmer := dbInfo.KmerSizes[len(dbInfo.KmerSizes)-1]
			matches, err := sketch.QueryDist(sketchDbPrefix(dir), queryPrefix, kmer, threads, knn)
			if err != nil {
				return err
			}

			if len(matches) == 0 {
				fmt.Fprintln(os.Stderr, "No matches found.")
				return nil
			}

			if matches[0].ANI < 0.80 {
				fmt.Fprintf(os.Stderr, "Warning: closest match has low ANI (%.1f%%) — query may not be bacterial or may be a novel species.\n",
					matches[0].ANI*100)
			}

			fmt.Fprintf(os.Stderr, "%d match(es) found.\n\n", len(matches))

			// Open database once for both display and download
			var db *idx.DB
			if !raw || downloadDir != "" {
				if idx.Exists(dir) {
					if d, err := idx.Open(dir); err == nil {
						db = d
						defer db.Close()
					}
				}
			}

			if raw {
				if err := printRawMatches(cmd, matches, format); err != nil {
					return err
				}
			} else {
				enriched := enrichMatches(matches, db)
				if err := printMatchRows(cmd, enriched, format); err != nil {
					return err
				}
			}

			if downloadDir != "" {
				return downloadMatchedGenomes(cmd, matches, db, downloadDir, cfg, dryRun)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&fileList, "file-list", "f", "", "file with one FASTA path per line")
	cmd.Flags().IntVar(&knn, "knn", 10, "number of closest matches to return")
	cmd.Flags().IntVar(&threads, "threads", 0, "CPU threads (default: all cores minus one)")
	cmd.Flags().StringVar(&format, "format", "", "output format: tsv, csv, json, table (default: auto)")
	cmd.Flags().BoolVar(&raw, "raw", false, "output raw distances without metadata enrichment")
	cmd.Flags().StringVar(&downloadDir, "download", "", "download matched genomes to this directory")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be downloaded without downloading")

	return cmd
}

func newSketchInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "info",
		Short:   "Show information about the local sketch database",
		Example: `  atb sketch info`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := dataDir
			if dir == "" {
				dir = cfg.General.DataDir
			}

			if !sketchDbExists(dir) {
				return fmt.Errorf("sketch database not found. Run 'atb sketch fetch' first")
			}

			info, err := sketch.Info(sketchSkmPath(dir))
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Sketch database: %s\n", sketchSkmPath(dir))
			fmt.Fprintf(w, "Samples:         %s\n", humanize.Comma(int64(info.Samples)))
			fmt.Fprintf(w, "K-mer sizes:     %v\n", info.KmerSizes)
			fmt.Fprintf(w, "Sketch size:     %s\n", humanize.Comma(int64(info.SketchSize)))

			sDir := sketchDir(dir)
			var totalSize int64
			for _, name := range []string{sources.SketchSkmFilename, sources.SketchSkdFilename} {
				if fi, err := os.Stat(filepath.Join(sDir, name)); err == nil {
					totalSize += fi.Size()
				}
			}
			fmt.Fprintf(w, "Database size:   %s\n", formatSize(totalSize))

			return nil
		},
	}
}

func printRawMatches(cmd *cobra.Command, matches []sketch.Match, format string) error {
	resolved := output.ResolveFormat(format)
	columns := []string{"query", "sample_accession", "ani"}
	rows := make([]output.Row, len(matches))
	for i, m := range matches {
		rows[i] = output.Row{
			"query":            m.QueryName,
			"sample_accession": m.RefName,
			"ani":              fmt.Sprintf("%.4f", m.ANI),
		}
	}
	return output.Format(cmd.OutOrStdout(), rows, columns, resolved)
}

// enrichMatches looks up metadata for each match from the SQLite index.
// db may be nil, in which case all metadata fields are "-".
func enrichMatches(matches []sketch.Match, db *idx.DB) []output.Row {
	rows := make([]output.Row, len(matches))
	for i, m := range matches {
		row := output.Row{
			"query":            m.QueryName,
			"sample_accession": m.RefName,
			"ani":              fmt.Sprintf("%.4f", m.ANI),
			"species":          "-",
			"N50":              "-",
			"completeness":     "-",
			"mlst_st":          "-",
			"aws_url":          "-",
		}
		if db != nil {
			if info, err := db.InfoRow(m.RefName); err == nil {
				for _, kv := range []struct{ key, col string }{
					{"sylph_species", "species"},
					{"N50", "N50"},
					{"Completeness_General", "completeness"},
					{"mlst_st", "mlst_st"},
					{"aws_url", "aws_url"},
				} {
					if v := info[kv.key]; v != "" {
						row[kv.col] = v
					}
				}
			}
		}
		rows[i] = row
	}
	return rows
}

var matchDisplayColumns = []string{"query", "sample_accession", "ani", "species", "N50", "completeness", "mlst_st"}

func printMatchRows(cmd *cobra.Command, rows []output.Row, format string) error {
	return output.Format(cmd.OutOrStdout(), rows, matchDisplayColumns, output.ResolveFormat(format))
}

func downloadMatchedGenomes(cmd *cobra.Command, matches []sketch.Match, db *idx.DB, outputDir string, cfg config.Config, dryRun bool) error {
	if db == nil {
		return fmt.Errorf("ATB index not found; run 'atb fetch' first to enable genome downloads")
	}

	enriched := enrichMatches(matches, db)

	var tasks []download.FileTask
	for _, row := range enriched {
		url := row["aws_url"]
		if url == "" || url == "-" {
			fmt.Fprintf(os.Stderr, "  skip %s: no download URL\n", row["sample_accession"])
			continue
		}
		tasks = append(tasks, download.FileTask{
			URL:      url,
			Filename: filepath.Base(url),
		})
	}

	if len(tasks) == 0 {
		fmt.Fprintln(os.Stderr, "No downloadable genomes found for matched samples.")
		return nil
	}

	if dryRun {
		fmt.Fprintf(os.Stderr, "\nWould download %d genome(s) to %s:\n", len(tasks), outputDir)
		for _, t := range tasks {
			fmt.Fprintf(os.Stderr, "  would download: %s\n", t.URL)
		}
		return nil
	}

	// Save query results as TSV alongside the downloads
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	resultsPath := filepath.Join(outputDir, "sketch_results.tsv")
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return fmt.Errorf("create results file: %w", err)
	}
	resultsCols := append(matchDisplayColumns, "aws_url")
	if err := output.Format(resultsFile, enriched, resultsCols, "tsv"); err != nil {
		resultsFile.Close()
		return fmt.Errorf("write results: %w", err)
	}
	resultsFile.Close()
	fmt.Fprintf(os.Stderr, "\nQuery results saved to %s\n", resultsPath)

	// Download genomes (consistent with atb download)
	par := cfg.Download.Parallel
	if par <= 0 {
		par = 4
	}

	fmt.Fprintf(os.Stderr, "Downloading %d file(s) to %s\n", len(tasks), outputDir)

	dl := download.New(download.Config{
		OutputDir:      outputDir,
		Parallel:       par,
		CheckDiskSpace: cfg.Download.CheckDiskSpace,
		MinFreeSpaceGB: cfg.Download.MinFreeSpaceGB,
		MaxRetries:     3,
	})

	dl.OnProgress = func(filename string, written, total int64) {
		if total > 0 {
			pct := float64(written) / float64(total) * 100
			fmt.Fprintf(os.Stderr, "\r  %s: %.1f%% (%s / %s)",
				filename, pct, formatSize(written), formatSize(total))
		} else {
			fmt.Fprintf(os.Stderr, "\r  %s: %s", filename, formatSize(written))
		}
	}

	result := dl.DownloadAllFiles(tasks)

	fmt.Fprintf(os.Stderr, "\r\033[K")
	fmt.Fprintf(os.Stderr, "Completed: %d/%d  Failed: %d  Bytes: %s\n",
		result.Completed, result.Total, result.Failed, formatSize(result.Bytes))

	for _, e := range result.Errors {
		fmt.Fprintf(os.Stderr, "  error: %s: %s\n", e.URL, e.Error)
	}

	if err := dl.WriteManifest("atb sketch query --download", result); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write manifest: %v\n", err)
	}

	if result.Failed > 0 {
		return fmt.Errorf("%d download(s) failed", result.Failed)
	}

	return nil
}
