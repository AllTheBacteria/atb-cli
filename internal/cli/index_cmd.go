package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/index"
)

func newIndexCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "index",
		Short: "Build or rebuild the SQLite query index",
		Example: `  # Build the index (runs automatically after fetch)
  atb index

  # Rebuild the index
  atb index --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := resolveDataDir(cfg)

			if err := ensureDatabase(dir); err != nil {
				return err
			}

			if !force && index.Exists(dir) {
				fmt.Fprintln(os.Stderr, "Index already exists. Use --force to rebuild.")
				return nil
			}

			stderrLog := func(format string, args ...any) {
				fmt.Fprintf(os.Stderr, format+"\n", args...)
			}

			fmt.Fprintln(os.Stderr, "Building query index...")
			if err := index.Build(dir, stderrLog); err != nil {
				return fmt.Errorf("building index: %w", err)
			}
			fmt.Fprintln(os.Stderr, "Index built successfully.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "rebuild even if index already exists")
	return cmd
}
