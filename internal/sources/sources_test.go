package sources

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveURL(t *testing.T) {
	got := ArchiveURL("atb_batch_42")
	want := AGCArchiveBaseURL + "atb_batch_42.agc"
	if got != want {
		t.Errorf("ArchiveURL = %q, want %q", got, want)
	}
}

func TestArchiveMapDefaultsPresent(t *testing.T) {
	if !strings.HasPrefix(AGCArchiveMapURL, "https://") {
		t.Errorf("AGCArchiveMapURL should be an https URL, got %q", AGCArchiveMapURL)
	}
	if !strings.HasPrefix(AGCArchiveBaseURL, "https://") || !strings.Contains(AGCArchiveBaseURL, "files=") {
		t.Errorf("AGCArchiveBaseURL should be an https URL with the files= selector, got %q", AGCArchiveBaseURL)
	}
	if filepath.Ext(AGCArchiveMapFilename) != ".zip" {
		t.Errorf("AGCArchiveMapFilename should be a .zip file, got %q", AGCArchiveMapFilename)
	}
	if AGCArchiveSubdir == "" || strings.ContainsAny(AGCArchiveSubdir, `/\`) {
		t.Errorf("AGCArchiveSubdir should be a bare subdir name, got %q", AGCArchiveSubdir)
	}
}

func TestAGCCollectionNodes(t *testing.T) {
	if len(AGCCollectionNodes) != 3 {
		t.Fatalf("want 3 collection nodes, got %d", len(AGCCollectionNodes))
	}
	seen := map[string]bool{}
	for _, n := range AGCCollectionNodes {
		if n.ID == "" || n.Part == "" {
			t.Errorf("node has empty field: %+v", n)
		}
		if seen[n.ID] {
			t.Errorf("duplicate node id %q", n.ID)
		}
		seen[n.ID] = true
	}
	if AGCArchivesFolder != "agc_archives" {
		t.Errorf("AGCArchivesFolder = %q, want agc_archives", AGCArchivesFolder)
	}
}

func TestPartForNode(t *testing.T) {
	if got := PartForNode(AGCCollectionNodes[0].ID); got != AGCCollectionNodes[0].Part {
		t.Errorf("PartForNode(%q) = %q, want %q", AGCCollectionNodes[0].ID, got, AGCCollectionNodes[0].Part)
	}
	if got := PartForNode("z7q5y"); got != "" {
		t.Errorf("PartForNode(unknown) = %q, want empty", got)
	}
}
