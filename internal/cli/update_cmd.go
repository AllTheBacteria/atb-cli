package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/allthebacteria/atb-cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update atb to the latest version",
		Example: `  # Check for updates and install interactively
  atb update

  # Update without confirmation (for scripts)
  atb update --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			currentVersion := cmd.Root().Version
			w := cmd.ErrOrStderr()

			fmt.Fprintf(w, "Current version: %s\n", currentVersion)
			fmt.Fprintf(w, "Checking for updates...\n")

			release, err := selfupdate.FetchLatestRelease()
			if err != nil {
				return fmt.Errorf("check for updates: %w", err)
			}

			if !selfupdate.CompareVersions(currentVersion, release.TagName) {
				fmt.Fprintf(w, "Already up to date.\n")
				return nil
			}

			fmt.Fprintf(w, "New version available: %s\n", release.TagName)
			if release.Body != "" {
				fmt.Fprintf(w, "\nChangelog:\n")
				for _, line := range strings.Split(strings.TrimSpace(release.Body), "\n") {
					fmt.Fprintf(w, "  %s\n", line)
				}
			}
			fmt.Fprintf(w, "\n")

			asset := selfupdate.FindAsset(release)
			if asset == nil {
				return fmt.Errorf("no binary available for your platform, download manually:\n  %s", release.HTMLURL)
			}

			fmt.Fprintf(w, "  Asset: %s\n", asset.Name)

			if !force {
				stat, _ := os.Stdin.Stat()
				if stat.Mode()&os.ModeCharDevice != 0 {
					fmt.Fprintf(w, "\nUpdate to %s? [y/N]: ", release.TagName)
					reader := bufio.NewReader(os.Stdin)
					input, _ := reader.ReadString('\n')
					if strings.TrimSpace(strings.ToLower(input)) != "y" {
						fmt.Fprintf(w, "Update cancelled.\n")
						return nil
					}
				} else {
					return fmt.Errorf("run with --force to update non-interactively")
				}
			}

			fmt.Fprintf(w, "Downloading %s...\n", release.TagName)
			if err := selfupdate.Apply(asset); err != nil {
				return fmt.Errorf("apply update: %w", err)
			}

			fmt.Fprintf(w, "Updated to %s\n", release.TagName)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "update without confirmation prompt")

	return cmd
}
