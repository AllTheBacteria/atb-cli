package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/download"
)

func newDownloadCmd() *cobra.Command {
	var (
		fromFile  string
		urlsFile  string
		singleURL string
		outputDir string
		parallel  int
		maxSamples int
		dryRun    bool
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download genome assemblies",
		Long:  "Download genome assembly files from ATB. URLs can be supplied via --url, --urls, --from, or stdin.",
		Example: `  # Download genomes from a query result file
  atb download --from results.tsv --output-dir ./genomes

  # Pipe query results directly to download
  atb query --species "Escherichia coli" --hq-only --limit 5 --format csv | atb download --from - -o ./ecoli

  # Preview what would be downloaded
  atb download --from results.tsv --dry-run

  # Download a single genome
  atb download --url https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMD00000355.fa.gz -o ./genomes

  # Download with higher parallelism
  atb download --from results.tsv -o ./genomes --parallel 8`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			// Collect URLs from all sources
			var urls []string

			if singleURL != "" {
				urls = append(urls, singleURL)
			}

			if urlsFile != "" {
				lines, err := readLines(urlsFile)
				if err != nil {
					return fmt.Errorf("reading --urls file: %w", err)
				}
				urls = append(urls, lines...)
			}

			if fromFile != "" {
				extracted, err := parseCSVURLs(fromFile)
				if err != nil {
					return fmt.Errorf("parsing --from file: %w", err)
				}
				urls = append(urls, extracted...)
			}

			// If no flags given, try reading from stdin pipe
			if len(urls) == 0 {
				fi, err := os.Stdin.Stat()
				if err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
					lines, err := readLinesFromReader(os.Stdin)
					if err != nil {
						return fmt.Errorf("reading stdin: %w", err)
					}
					urls = append(urls, lines...)
				}
			}

			if len(urls) == 0 {
				return fmt.Errorf("no URLs provided; use --url, --urls, --from, or pipe URLs to stdin")
			}

			// Apply max-samples cap
			if maxSamples > 0 && len(urls) > maxSamples {
				fmt.Fprintf(os.Stderr, "Capping to %d URLs (--max-samples)\n", maxSamples)
				urls = urls[:maxSamples]
			}

			// Resolve output dir
			outDir := outputDir
			if outDir == "" {
				outDir = cfg.Download.OutputDir
			}

			// Resolve parallel
			par := parallel
			if par == 0 {
				par = cfg.Download.Parallel
			}

			fmt.Fprintf(os.Stderr, "Downloading %d file(s) to %s\n", len(urls), outDir)

			if dryRun {
				fmt.Fprintln(os.Stderr, "(dry-run: no files will be downloaded)")
				for _, u := range urls {
					fmt.Fprintln(os.Stderr, "  would download:", u)
				}
				return nil
			}

			dl := download.New(download.Config{
				OutputDir:      outDir,
				Parallel:       par,
				CheckDiskSpace: cfg.Download.CheckDiskSpace && !force,
				MinFreeSpaceGB: cfg.Download.MinFreeSpaceGB,
				MaxRetries:     3,
			})

			result := dl.DownloadAll(urls)

			fmt.Fprintf(os.Stderr, "Completed: %d/%d  Failed: %d  Bytes: %s\n",
				result.Completed, result.Total, result.Failed, formatSize(result.Bytes))

			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  error: %s: %s\n", e.URL, e.Error)
			}

			if err := dl.WriteManifest("atb download", result); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to write manifest: %v\n", err)
			}

			if result.Failed > 0 {
				return fmt.Errorf("%d download(s) failed", result.Failed)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&fromFile, "from", "", "CSV/TSV query result file to extract aws_url column from")
	cmd.Flags().StringVar(&urlsFile, "urls", "", "file with one URL per line")
	cmd.Flags().StringVar(&singleURL, "url", "", "single URL to download")
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "directory to save downloads (default from config)")
	cmd.Flags().IntVarP(&parallel, "parallel", "p", 0, "parallel downloads (default from config)")
	cmd.Flags().IntVar(&maxSamples, "max-samples", 0, "limit number of downloads")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print URLs without downloading")
	cmd.Flags().BoolVar(&force, "force", false, "re-download even if file exists; skip disk space check")

	return cmd
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readLinesFromReader(f)
}

func readLinesFromReader(r interface{ Read([]byte) (int, error) }) ([]string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	if rd, ok := r.(*os.File); ok {
		scanner = bufio.NewScanner(rd)
	}
	var lines []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}
