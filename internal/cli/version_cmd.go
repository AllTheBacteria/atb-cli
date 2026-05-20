package cli

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print version and environment information",
		Example: `  atb version`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			resolvedDataDir := resolveDataDir(cfg)

			fmt.Fprintf(cmd.OutOrStdout(), "atb-cli version: %s\n", root.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "Go version:       %s\n", runtime.Version())
			fmt.Fprintf(cmd.OutOrStdout(), "OS/Arch:          %s/%s\n", runtime.GOOS, runtime.GOARCH)
			fmt.Fprintf(cmd.OutOrStdout(), "Data dir:         %s\n", resolvedDataDir)
			fmt.Fprintf(cmd.OutOrStdout(), "Config path:      %s\n", resolveConfigPath())

			// Count parquet files in data dir
			pattern := filepath.Join(resolvedDataDir, "*.parquet")
			matches, _ := filepath.Glob(pattern)
			fmt.Fprintf(cmd.OutOrStdout(), "Parquet files:    %d\n", len(matches))

			return nil
		},
	}
}
