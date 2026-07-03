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
	if filepath.Ext(AGCArchiveMapFilename) != ".txt" {
		t.Errorf("AGCArchiveMapFilename should be a .txt file, got %q", AGCArchiveMapFilename)
	}
	if AGCArchiveSubdir == "" || strings.ContainsAny(AGCArchiveSubdir, `/\`) {
		t.Errorf("AGCArchiveSubdir should be a bare subdir name, got %q", AGCArchiveSubdir)
	}
}
