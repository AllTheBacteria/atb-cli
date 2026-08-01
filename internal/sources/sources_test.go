package sources_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

func TestArchiveURL(t *testing.T) {
	got := sources.ArchiveURL("atb_batch_42")
	want := sources.AGCArchiveBaseURL + "atb_batch_42.agc"
	if got != want {
		t.Errorf("ArchiveURL = %q, want %q", got, want)
	}
}

func TestArchiveMapDefaultsPresent(t *testing.T) {
	if !strings.HasPrefix(sources.AGCArchiveMapURL, "https://") {
		t.Errorf("AGCArchiveMapURL should be an https URL, got %q", sources.AGCArchiveMapURL)
	}
	if !strings.HasPrefix(sources.AGCArchiveBaseURL, "https://") || !strings.Contains(sources.AGCArchiveBaseURL, "files=") {
		t.Errorf("AGCArchiveBaseURL should be an https URL with the files= selector, got %q", sources.AGCArchiveBaseURL)
	}
	if filepath.Ext(sources.AGCArchiveMapFilename) != ".zip" {
		t.Errorf("AGCArchiveMapFilename should be a .zip file, got %q", sources.AGCArchiveMapFilename)
	}
	if sources.AGCArchiveSubdir == "" || strings.ContainsAny(sources.AGCArchiveSubdir, `/\`) {
		t.Errorf("AGCArchiveSubdir should be a bare subdir name, got %q", sources.AGCArchiveSubdir)
	}
}

func TestAGCCollectionNodes(t *testing.T) {
	if len(sources.AGCCollectionNodes) != 3 {
		t.Fatalf("want 3 collection nodes, got %d", len(sources.AGCCollectionNodes))
	}
	seen := map[string]bool{}
	for _, n := range sources.AGCCollectionNodes {
		if n.ID == "" || n.Part == "" {
			t.Errorf("node has empty field: %+v", n)
		}
		if seen[n.ID] {
			t.Errorf("duplicate node id %q", n.ID)
		}
		seen[n.ID] = true
	}
	if sources.AGCArchivesFolder != "agc_archives" {
		t.Errorf("AGCArchivesFolder = %q, want agc_archives", sources.AGCArchivesFolder)
	}
}

func TestPartForNode(t *testing.T) {
	if got := sources.PartForNode(sources.AGCCollectionNodes[0].ID); got != sources.AGCCollectionNodes[0].Part {
		t.Errorf("PartForNode(%q) = %q, want %q", sources.AGCCollectionNodes[0].ID, got, sources.AGCCollectionNodes[0].Part)
	}
	if got := sources.PartForNode("z7q5y"); got != "" {
		t.Errorf("PartForNode(unknown) = %q, want empty", got)
	}
}

func TestAGCBatchMetadataURLPresent(t *testing.T) {
	if sources.AGCBatchMetadataURL != "https://osf.io/download/8y9r2/" {
		t.Errorf("AGCBatchMetadataURL: got %q", sources.AGCBatchMetadataURL)
	}
}
