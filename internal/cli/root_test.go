package cli

import (
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/config"
)

// TestDataDirUsageClarifiesMetadataDir guards against the --data-dir help text
// implying it controls where `atb download` saves genome assemblies. The old
// wording "data directory for downloaded files" led a user to expect --data-dir
// (and ATB_DATA_DIR) to redirect downloads, when in fact those land in the
// separate download.output_dir (issue #20).
func TestDataDirUsageClarifiesMetadataDir(t *testing.T) {
	usage := dataDirUsage()

	if strings.Contains(usage, "downloaded files") {
		t.Errorf("--data-dir usage still conflates with genome downloads: %q", usage)
	}
	if !strings.Contains(usage, "metadata") {
		t.Errorf("--data-dir usage should describe the metadata index it governs, got: %q", usage)
	}
	if !strings.Contains(usage, "$ATB_DATA_DIR") {
		t.Errorf("--data-dir usage should still surface the $ATB_DATA_DIR override, got: %q", usage)
	}
	if !strings.Contains(usage, config.DefaultDataDir()) {
		t.Errorf("--data-dir usage should still show the resolved default, got: %q", usage)
	}
}
