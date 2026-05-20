package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/download"
	"github.com/allthebacteria/atb-cli/internal/osf"
	"github.com/allthebacteria/atb-cli/internal/output"
)

func newOSFCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "osf",
		Short: "Browse and download files from the AllTheBacteria OSF repository",
		Long: `Browse the AllTheBacteria file index on OSF (Open Science Framework).

The index catalogs ~3,000 files across 75+ project categories including
assemblies, annotations, AMR results, MLST, protein structures, and more.

The index is cached locally and refreshed every 7 days.`,
	}

	cmd.AddCommand(newOSFLsCmd())
	cmd.AddCommand(newOSFDownloadCmd())

	return cmd
}

func newOSFLsCmd() *cobra.Command {
	var (
		grep    string
		project string
		sortBy  string
		format  string
		refresh bool
	)

	cmd := &cobra.Command{
		Use:   "ls [filter]",
		Short: "List files and projects in the ATB index",
		Long: `List files from the AllTheBacteria OSF index.

Without arguments, shows project categories with file counts and total sizes.
With a positional argument, shows files in projects matching that substring.`,
		Example: `  # List all project categories
  atb osf ls

  # Show files in projects matching "AMR"
  atb osf ls AMR

  # Show files in an exact project
  atb osf ls --project AllTheBacteria/Assembly

  # Regex search across project and filename
  atb osf ls --grep "bakta.*batch"

  # Sort by size descending
  atb osf ls AMR --sort size

  # JSON output
  atb osf ls AMR --format json

  # Force refresh of the cached index
  atb osf ls --refresh`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := resolveDataDir(cfg)

			idx, err := osf.FetchIndex(dir, refresh)
			if err != nil {
				return fmt.Errorf("loading index: %w", err)
			}

			filter := ""
			if len(args) > 0 {
				filter = args[0]
			}

			// No filter, no grep, no project → show project summary
			if filter == "" && grep == "" && project == "" {
				return printProjectSummary(cmd, idx, format)
			}

			var entries []osf.Entry

			if grep != "" {
				entries, err = idx.Filter(project, grep)
				if err != nil {
					return err
				}
			} else if project != "" {
				entries, err = idx.Filter(project, "")
				if err != nil {
					return err
				}
			} else {
				entries = idx.MatchProject(filter)
			}

			if len(entries) == 0 {
				fmt.Fprintln(os.Stderr, "No files found.")
				return nil
			}

			if sortBy == "size" {
				sort.Slice(entries, func(i, j int) bool {
					return entries[i].SizeMB > entries[j].SizeMB
				})
			}

			return printEntries(cmd, entries, format)
		},
	}

	cmd.Flags().StringVar(&grep, "grep", "", "regex filter across project+filename")
	cmd.Flags().StringVar(&project, "project", "", "filter by exact project prefix")
	cmd.Flags().StringVar(&sortBy, "sort", "", "sort order: name (default) or size")
	cmd.Flags().StringVar(&format, "format", "", "output format: tsv, csv, json, table (default: table in terminal, tsv in pipe)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "re-download the index even if cached")

	return cmd
}

func newOSFDownloadCmd() *cobra.Command {
	var (
		outputDir string
		parallel  int
		dryRun    bool
		force     bool
		all       bool
		project   string
		verify    bool
		refresh   bool
	)

	cmd := &cobra.Command{
		Use:   "download <pattern> [pattern...]",
		Short: "Download files from OSF matching a pattern",
		Long: `Download files from the AllTheBacteria OSF repository.

Patterns are matched as regex against "project/filename".
Use --project to filter by project prefix first.`,
		Example: `  # Download files matching a pattern
  atb osf download "AMRFinderPlus.*results.*latest"

  # Download all files in a project
  atb osf download --project AllTheBacteria/MLST --all

  # Preview what would be downloaded
  atb osf download --dry-run "bakta.*batch.*1\."

  # Download to a specific directory
  atb osf download -o ./data "Assembly.*batch1"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) == 0 {
				return cmd.Help()
			}

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := resolveDataDir(cfg)

			idx, err := osf.FetchIndex(dir, refresh)
			if err != nil {
				return fmt.Errorf("loading index: %w", err)
			}

			// Collect matching entries
			var entries []osf.Entry
			if all && len(args) == 0 {
				filtered, err := idx.Filter(project, "")
				if err != nil {
					return err
				}
				entries = filtered
			} else {
				for _, pattern := range args {
					matched, err := idx.Filter(project, pattern)
					if err != nil {
						return err
					}
					entries = append(entries, matched...)
				}
			}

			// Deduplicate by URL
			entries = deduplicateEntries(entries)

			if len(entries) == 0 {
				fmt.Fprintln(os.Stderr, "No files matched.")
				return nil
			}

			// Safety check for large downloads
			if len(entries) > 10 && !dryRun {
				var totalMB float64
				for _, e := range entries {
					totalMB += e.SizeMB
				}
				fmt.Fprintf(os.Stderr, "This will download %d files (%.1f MB total).\n", len(entries), totalMB)
				if !confirmDownload() {
					fmt.Fprintln(os.Stderr, "Cancelled.")
					return nil
				}
			}

			outDir := outputDir
			if outDir == "" {
				outDir = dir
			}

			par := parallel
			if par == 0 {
				par = cfg.Download.Parallel
			}

			if dryRun {
				fmt.Fprintf(os.Stderr, "Would download %d file(s) to %s\n", len(entries), outDir)
				for _, e := range entries {
					fmt.Fprintf(os.Stderr, "  %.1f MB  %s/%s\n", e.SizeMB, e.Project, e.Filename)
				}
				return nil
			}

			fmt.Fprintf(os.Stderr, "Downloading %d file(s) to %s\n", len(entries), outDir)

			dl := download.New(download.Config{
				OutputDir:      outDir,
				Parallel:       par,
				CheckDiskSpace: !force,
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

			tasks := make([]download.FileTask, len(entries))
			for i, e := range entries {
				md5 := ""
				if verify {
					md5 = e.MD5
				}
				tasks[i] = download.FileTask{
					URL:      e.URL,
					Filename: filepath.Base(e.Filename),
					MD5:      md5,
				}
			}

			result := dl.DownloadAllFiles(tasks)

			fmt.Fprintf(os.Stderr, "\r\033[K") // clear progress line
			fmt.Fprintf(os.Stderr, "Completed: %d/%d  Failed: %d  Bytes: %s\n",
				result.Completed, result.Total, result.Failed, formatSize(result.Bytes))

			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "  error: %s: %s\n", e.URL, e.Error)
			}

			if result.Failed > 0 {
				return fmt.Errorf("%d download(s) failed", result.Failed)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "directory to save downloads (default: data dir)")
	cmd.Flags().IntVarP(&parallel, "parallel", "p", 0, "parallel downloads (default from config)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print files without downloading")
	cmd.Flags().BoolVar(&force, "force", false, "re-download even if file exists")
	cmd.Flags().BoolVar(&all, "all", false, "download all matching files")
	cmd.Flags().StringVar(&project, "project", "", "filter by project prefix")
	cmd.Flags().BoolVar(&verify, "verify", false, "verify MD5 after download")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "re-download the index even if cached")

	return cmd
}

func printProjectSummary(cmd *cobra.Command, idx *osf.Index, format string) error {
	projects := idx.Projects()
	resolved := output.ResolveFormat(format)

	columns := []string{"project", "files", "size"}
	rows := make([]output.Row, len(projects))
	for i, p := range projects {
		rows[i] = output.Row{
			"project": p.Project,
			"files":   fmt.Sprintf("%d", p.FileCount),
			"size":    humanSize(p.TotalMB),
		}
	}

	fmt.Fprintf(os.Stderr, "%d projects, %s files total\n\n", len(projects), humanize.Comma(int64(len(idx.Entries))))
	return output.Format(cmd.OutOrStdout(), rows, columns, resolved)
}

func printEntries(cmd *cobra.Command, entries []osf.Entry, format string) error {
	resolved := output.ResolveFormat(format)

	columns := []string{"project", "filename", "size", "url"}
	rows := make([]output.Row, len(entries))
	for i, e := range entries {
		rows[i] = output.Row{
			"project":  e.Project,
			"filename": e.Filename,
			"size":     humanSize(e.SizeMB),
			"url":      e.URL,
		}
	}

	fmt.Fprintf(os.Stderr, "%d file(s)\n\n", len(entries))
	return output.Format(cmd.OutOrStdout(), rows, columns, resolved)
}

func humanSize(mb float64) string {
	switch {
	case mb >= 1024:
		return fmt.Sprintf("%.1f GB", mb/1024)
	case mb >= 1:
		return fmt.Sprintf("%.1f MB", mb)
	case mb > 0:
		return fmt.Sprintf("%.0f KB", mb*1024)
	default:
		return "0 B"
	}
}

func deduplicateEntries(entries []osf.Entry) []osf.Entry {
	seen := make(map[string]bool, len(entries))
	out := make([]osf.Entry, 0, len(entries))
	for _, e := range entries {
		if !seen[e.URL] {
			seen[e.URL] = true
			out = append(out, e)
		}
	}
	return out
}

func confirmDownload() bool {
	stat, _ := os.Stdin.Stat()
	if stat.Mode()&os.ModeCharDevice == 0 {
		return true // non-interactive
	}

	fmt.Fprintf(os.Stderr, "Continue? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(strings.ToLower(input))
	return choice == "y" || choice == "yes"
}
