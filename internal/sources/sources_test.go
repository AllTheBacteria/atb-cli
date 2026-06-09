package sources

import "testing"

func TestArchiveURL(t *testing.T) {
	got := ArchiveURL("atb_batch_42")
	want := AGCArchiveBaseURL + "atb_batch_42.agc"
	if got != want {
		t.Errorf("ArchiveURL = %q, want %q", got, want)
	}
}

func TestArchiveMapDefaultsPresent(t *testing.T) {
	if AGCArchiveMapURL == "" || AGCArchiveBaseURL == "" {
		t.Fatal("provisional AGC archive URLs must be set")
	}
	if AGCArchiveMapFilename == "" || AGCArchiveSubdir == "" {
		t.Fatal("AGC archive map filename and subdir must be set")
	}
}
