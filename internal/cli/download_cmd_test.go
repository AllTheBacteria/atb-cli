package cli

import (
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/config"
)

// TestDownloadOutputDirUsageShowsDefault guards against the --output-dir help
// being too vague to resolve the "where do my downloads go?" confusion in
// issue #20. The old "directory to save downloads (default from config)" hid
// both the actual destination and the config key that changes it.
func TestDownloadOutputDirUsageShowsDefault(t *testing.T) {
	cmd := newDownloadCmd()
	flag := cmd.Flags().Lookup("output-dir")
	if flag == nil {
		t.Fatal("download command is missing the --output-dir flag")
	}
	usage := flag.Usage

	wantDir := config.Default().Download.OutputDir
	if !strings.Contains(usage, wantDir) {
		t.Errorf("--output-dir usage should show the default download dir %q, got: %q", wantDir, usage)
	}
	if !strings.Contains(usage, "download.output_dir") {
		t.Errorf("--output-dir usage should name the config key download.output_dir, got: %q", usage)
	}
}
