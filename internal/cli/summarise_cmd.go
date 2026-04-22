package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/query"
	"github.com/allthebacteria/atb-cli/internal/summarise"
)

func newSummariseCmd() *cobra.Command {
	var (
		by   []string
		topN int
		from string
	)

	cmd := &cobra.Command{
		Use:     "summarise",
		Aliases: []string{"summarize"},
		Short:   "Print summary statistics for the ATB database",
		Long: `Print summary statistics for all genomes in the local ATB database.

By default prints total counts, HQ fraction, top species, and dataset breakdown.
Use --by to group results by a specific column (e.g. --by sylph_species).`,
		Example: `  # Default summary of the full database
  atb summarise

  # Group by species (top 20)
  atb summarise --by sylph_species --top 20

  # Summarise a previous query result
  atb summarise --from salmonella.tsv

  # Pipe query to summarise
  atb query --genus Salmonella --hq-only --limit 100 --format csv | atb summarise --from -`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}

			var rows []query.ResultRow

			if from != "" {
				var err error
				rows, err = readResultsFromFile(from)
				if err != nil {
					return fmt.Errorf("reading --from file: %w", err)
				}
			} else {
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

				// Run a full query with no filters to get all rows
				rows, err = query.Execute(dir, query.Filters{}, nil)
				if err != nil {
					return fmt.Errorf("reading database: %w", err)
				}
			}

			w := cmd.OutOrStdout()

			if len(by) > 0 {
				for _, dim := range by {
					dim = strings.TrimSpace(dim)
					groups := summarise.GroupBy(rows, dim)
					summarise.PrintGroupBy(w, groups, dim, topN)
					fmt.Fprintln(w)
				}
				return nil
			}

			s := summarise.DefaultSummary(rows)
			summarise.PrintSummary(w, s, topN)
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&by, "by", nil, "group results by column(s) (e.g. --by sylph_species)")
	cmd.Flags().IntVar(&topN, "top", 10, "number of top entries to show per group")
	cmd.Flags().StringVar(&from, "from", "", "read rows from a CSV/TSV query result file (use \"-\" for stdin)")

	return cmd
}
