package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/amr"
	"github.com/allthebacteria/atb-cli/internal/fetch"
	"github.com/allthebacteria/atb-cli/internal/index"
)

func newFetchCmd() *cobra.Command {
	var (
		all      bool
		tables   []string
		force    bool
		parallel int
		yes      bool
	)

	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Download ATB parquet metadata tables",
		Long: `Download parquet metadata tables from AllTheBacteria.

By default the core tables are downloaded, which includes amrfinderplus.parquet.
Note: AMR data is ~1.2 GB to download but expands to ~35 GB of per-genus indexes,
so you'll be prompted to confirm before download and again before index build.
Use --yes to skip both prompts and proceed automatically.

Use --all to download all available tables.
Use --tables to specify exact tables by name.`,
		Example: `  # Download core tables (includes AMR data)
  atb fetch

  # Download and build indexes automatically (no prompt)
  atb fetch --yes

  # Download all tables including ENA metadata
  atb fetch --all

  # Force re-download
  atb fetch --force

  # Download specific tables
  atb fetch --tables assembly.parquet,amrfinderplus.parquet`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := resolveDataDir(cfg)

			par := parallel
			if par == 0 {
				par = cfg.Fetch.Parallel
			}
			if par <= 0 {
				par = 4
			}

			f := fetch.New(fetch.Config{
				DataDir:  dir,
				Parallel: par,
			})

			// Determine which tables to fetch
			var targets []string
			switch {
			case len(tables) > 0:
				targets = tables
			case all:
				targets = fetch.AllTables()
			default:
				targets = fetch.CoreTables()
			}

			// Validate table names
			for _, name := range targets {
				if _, ok := fetch.URLForTable(name); !ok {
					return fmt.Errorf("unknown table %q; run 'atb fetch --all' to see available tables", name)
				}
			}

			if containsTable(targets, amr.AMRFileName) && !yes {
				if !promptAMRDiskUsage() {
					targets = removeTable(targets, amr.AMRFileName)
					fmt.Fprintf(os.Stderr, "Skipping %s. AMR queries will not work without it.\n", amr.AMRFileName)
					if len(targets) == 0 {
						fmt.Fprintf(os.Stderr, "No tables left to fetch.\n")
						return nil
					}
				}
			}

			fmt.Fprintf(os.Stderr, "Fetching %d table(s) into %s\n", len(targets), dir)

			sem := make(chan struct{}, par)
			var mu sync.Mutex
			var wg sync.WaitGroup
			var failed int

			for _, name := range targets {
				wg.Add(1)
				tableURL, _ := fetch.URLForTable(name)
				go func(tableName, tURL string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					fmt.Fprintf(os.Stderr, "  fetching %s\n", tableName)
					if err := f.FetchTable(tableName, tURL, force); err != nil {
						fmt.Fprintf(os.Stderr, "  error: %s: %v\n", tableName, err)
						mu.Lock()
						failed++
						mu.Unlock()
					} else {
						fmt.Fprintf(os.Stderr, "  done: %s\n", tableName)
					}
				}(name, tableURL)
			}

			wg.Wait()

			if failed > 0 {
				return fmt.Errorf("%d table(s) failed to download", failed)
			}

			fmt.Fprintf(os.Stderr, "All tables downloaded successfully.\n")

			// Report database location and file sizes.
			reportDatabaseSummary(dir)

			// Determine if we should build indexes.
			shouldBuild := yes
			if !shouldBuild {
				shouldBuild = promptBuildIndexes()
			}

			if !shouldBuild {
				fmt.Fprintf(os.Stderr, "\nSkipped index build. Run 'atb fetch --force --yes' later to build.\n")
				return nil
			}

			stderrLog := func(format string, args ...any) {
				fmt.Fprintf(os.Stderr, format+"\n", args...)
			}

			// Build genus partitions for faster AMR queries.
			fmt.Fprintf(os.Stderr, "\n")
			if err := amr.BuildPartitions(dir, stderrLog); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: AMR partition build failed: %v\n", err)
			}

			// Build the SQLite query index.
			fmt.Fprintf(os.Stderr, "\nBuilding query index...\n")
			if err := index.Build(dir, stderrLog); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: index build failed: %v\n", err)
			}

			// Final summary with updated sizes.
			fmt.Fprintf(os.Stderr, "\n")
			reportDatabaseSummary(dir)

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "download all available tables")
	cmd.Flags().StringSliceVar(&tables, "tables", nil, "specific tables to download (comma-separated names)")
	cmd.Flags().BoolVar(&force, "force", false, "re-download even if table already exists")
	cmd.Flags().IntVar(&parallel, "parallel", 0, "parallel downloads (default from config)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "build indexes without prompting")

	return cmd
}

func containsTable(tables []string, name string) bool {
	for _, t := range tables {
		if t == name {
			return true
		}
	}
	return false
}

func removeTable(tables []string, name string) []string {
	out := tables[:0]
	for _, t := range tables {
		if t != name {
			out = append(out, t)
		}
	}
	return out
}

func promptAMRDiskUsage() bool {
	stat, _ := os.Stdin.Stat()
	interactive := stat.Mode()&os.ModeCharDevice != 0
	if !interactive {
		return true // non-interactive (scripts/CI) → proceed
	}

	fmt.Fprintf(os.Stderr, "Note: amrfinderplus.parquet is ~1.2 GB to download, but during indexing\n")
	fmt.Fprintf(os.Stderr, "it expands to per-genus partitions and SQLite indexes totaling ~35 GB.\n")
	fmt.Fprintf(os.Stderr, "Make sure your data directory has enough free space.\n")
	fmt.Fprintf(os.Stderr, "Continue? [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(strings.ToLower(input))

	return choice == "" || choice == "y" || choice == "yes"
}

func promptBuildIndexes() bool {
	stat, _ := os.Stdin.Stat()
	interactive := stat.Mode()&os.ModeCharDevice != 0
	if !interactive {
		return true // non-interactive (scripts/CI) → build automatically
	}

	fmt.Fprintf(os.Stderr, "\nBuild query indexes? This speeds up queries but takes ~5 minutes.\n")
	fmt.Fprintf(os.Stderr, "  [y] Yes, build now (recommended)\n")
	fmt.Fprintf(os.Stderr, "  [n] No, skip for now\n")
	fmt.Fprintf(os.Stderr, "  Choice [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(strings.ToLower(input))

	return choice == "" || choice == "y" || choice == "yes"
}

func reportDatabaseSummary(dir string) {
	fmt.Fprintf(os.Stderr, "\nDatabase: %s\n", dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var parquetSize, indexSize, partitionSize int64
	var parquetCount, partitionCount int

	for _, e := range entries {
		if e.IsDir() {
			if e.Name() == amr.PartitionDir {
				partEntries, _ := os.ReadDir(filepath.Join(dir, amr.PartitionDir))
				for _, pe := range partEntries {
					info, err := pe.Info()
					if err == nil {
						partitionSize += info.Size()
						partitionCount++
					}
				}
			}
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		switch {
		case strings.HasSuffix(e.Name(), ".parquet"):
			parquetSize += info.Size()
			parquetCount++
		case e.Name() == index.IndexFileName:
			indexSize = info.Size()
		}
	}

	fmt.Fprintf(os.Stderr, "  Parquet tables:  %d files (%s)\n", parquetCount, formatSize(parquetSize))
	if partitionCount > 0 {
		fmt.Fprintf(os.Stderr, "  AMR partitions:  %d files (%s)\n", partitionCount, formatSize(partitionSize))
	}
	if indexSize > 0 {
		fmt.Fprintf(os.Stderr, "  Query index:     %s\n", formatSize(indexSize))
	}
	total := parquetSize + indexSize + partitionSize
	fmt.Fprintf(os.Stderr, "  Total:           %s\n", formatSize(total))
}

// commaStr formats a numeric string with thousand separators.
// Non-numeric strings are returned as-is.
func commaStr(s string) string {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		// Try float (e.g. "99.06")
		if _, ferr := strconv.ParseFloat(s, 64); ferr == nil {
			return s // floats don't get commas
		}
		return s
	}
	return humanize.Comma(n)
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
