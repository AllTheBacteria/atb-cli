package sources_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/sources"
)

func TestArchiveMapDefaultsPresent(t *testing.T) {
	if !strings.HasPrefix(sources.AGCArchiveMapURL, "https://") {
		t.Errorf("AGCArchiveMapURL should be an https URL, got %q", sources.AGCArchiveMapURL)
	}
	if filepath.Ext(sources.AGCArchiveMapFilename) != ".gz" {
		t.Errorf("AGCArchiveMapFilename should be a .gz file, got %q", sources.AGCArchiveMapFilename)
	}
	if sources.AGCArchiveSubdir == "" || strings.ContainsAny(sources.AGCArchiveSubdir, `/\`) {
		t.Errorf("AGCArchiveSubdir should be a bare subdir name, got %q", sources.AGCArchiveSubdir)
	}
}

func TestAGCCollectionNodes(t *testing.T) {
	wantIDs := []string{"4jq8u", "jmeqg", "kzcnr"}
	if len(sources.AGCCollectionNodes) != len(wantIDs) {
		t.Fatalf("node count: got %d, want %d", len(sources.AGCCollectionNodes), len(wantIDs))
	}
	for i, n := range sources.AGCCollectionNodes {
		if n.ID != wantIDs[i] {
			t.Errorf("node %d: got %q, want %q", i, n.ID, wantIDs[i])
		}
	}
	if sources.AGCArchivesFolder != "agc_batches" {
		t.Errorf("AGCArchivesFolder: got %q, want agc_batches", sources.AGCArchivesFolder)
	}
	if sources.AGCArchiveMapURL != "https://osf.io/download/gtqrx/" {
		t.Errorf("AGCArchiveMapURL: got %q", sources.AGCArchiveMapURL)
	}
	if sources.AGCArchiveMapFilename != "assemblies_filelist.txt.gz" {
		t.Errorf("AGCArchiveMapFilename: got %q", sources.AGCArchiveMapFilename)
	}
}

func TestAGCBatchMetadataURLPresent(t *testing.T) {
	if sources.AGCBatchMetadataURL != "https://osf.io/download/8y9r2/" {
		t.Errorf("AGCBatchMetadataURL: got %q", sources.AGCBatchMetadataURL)
	}
}

func TestAGCIndexURLPublished(t *testing.T) {
	if !strings.HasPrefix(sources.AGCIndexURL, "https://osf.io/download/") {
		t.Errorf("AGCIndexURL should be a published OSF /download/ URL, got %q", sources.AGCIndexURL)
	}
	if !strings.HasSuffix(sources.AGCIndexURL, "/") {
		t.Errorf("AGCIndexURL should end with a trailing slash, got %q", sources.AGCIndexURL)
	}
}
