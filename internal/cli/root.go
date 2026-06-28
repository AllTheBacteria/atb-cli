package cli

import (
	"fmt"
	"os"

	"github.com/allthebacteria/atb-cli/internal/config"
	"github.com/allthebacteria/atb-cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

// dataDirUsage returns the --data-dir flag help string with the effective
// default resolved at runtime, so users on shared installs see the
// $ATB_DATA_DIR override reflected in --help.
func dataDirUsage() string {
	return fmt.Sprintf("directory for the local metadata index (default %s; override with $ATB_DATA_DIR)", config.DefaultDataDir())
}

var (
	cfgFile string
	dataDir string

	// WaitForUpdateCheck is set by PersistentPreRun. Call it before exiting
	// main() to give the background update check time to save state.
	WaitForUpdateCheck func()
)

// RootCmd is the base command for atb.
var RootCmd = &cobra.Command{
	Use:          "atb",
	Short:        "Query and download AllTheBacteria genomes",
	SilenceUsage: true,
}

func init() {
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.atb/config.toml)")
	RootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", dataDirUsage())

	RootCmd.AddCommand(newConfigCmd())
	RootCmd.AddCommand(newQueryCmd())
	RootCmd.AddCommand(newDownloadCmd())
	RootCmd.AddCommand(newInfoCmd())
	RootCmd.AddCommand(newVersionCmd())
	RootCmd.AddCommand(newFetchCmd())
	RootCmd.AddCommand(newSummariseCmd())
	RootCmd.AddCommand(newUpdateCmd())
	RootCmd.AddCommand(newAMRCmd())
	RootCmd.AddCommand(newIndexCmd())
	RootCmd.AddCommand(newMCPCmd())
	RootCmd.AddCommand(newMLSTCmd())
	RootCmd.AddCommand(newOSFCmd())
	RootCmd.AddCommand(newSketchCmd())
	RootCmd.AddCommand(newAGCCmd())

	// Background update check (non-blocking, once every 24h)
	originalPreRun := RootCmd.PersistentPreRun
	RootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if originalPreRun != nil {
			originalPreRun(cmd, args)
		}
		WaitForUpdateCheck = selfupdate.CheckInBackground(cmd.Root().Version, os.Stderr)
	}
}

// NewRootCmd creates a fresh root command with its own flag state.
// Useful for testing to avoid shared global state between test runs.
func NewRootCmd(version string) *cobra.Command {
	var localCfgFile, localDataDir string

	root := &cobra.Command{
		Use:          "atb",
		Short:        "Query and download AllTheBacteria genomes",
		SilenceUsage: true,
		Version:      version,
	}

	root.PersistentFlags().StringVar(&localCfgFile, "config", "", "config file (default $HOME/.atb/config.toml)")
	root.PersistentFlags().StringVar(&localDataDir, "data-dir", "", dataDirUsage())

	// Sync local flag values into the package-level vars that subcommands read
	// before each command executes.
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cfgFile = localCfgFile
		dataDir = localDataDir
		return nil
	}

	root.AddCommand(newConfigCmd())
	root.AddCommand(newQueryCmd())
	root.AddCommand(newDownloadCmd())
	root.AddCommand(newInfoCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newFetchCmd())
	root.AddCommand(newSummariseCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newAMRCmd())
	root.AddCommand(newIndexCmd())
	root.AddCommand(newMCPCmd())
	root.AddCommand(newMLSTCmd())
	root.AddCommand(newOSFCmd())
	root.AddCommand(newSketchCmd())
	root.AddCommand(newAGCCmd())

	return root
}
