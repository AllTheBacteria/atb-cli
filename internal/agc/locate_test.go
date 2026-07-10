package agc

import (
	"testing"

	"github.com/allthebacteria/atb-cli/internal/osf"
)

func TestLocate(t *testing.T) {
	idx := &osf.Index{Entries: []osf.Entry{
		{Project: "Escherichia_coli", ProjectID: "6g8by",
			Filename: "Escherichia_coli_global_ordered_0001.agc",
			URL:      "https://osf.io/download/ec1/", MD5: "md5ec", SizeMB: 1.0},
	}}
	byName := RefsFromIndex(idx)
	m := ArchiveMap{
		"ACC_FOUND":  "Escherichia_coli_global_ordered_0001",
		"ACC_NOTYET": "Ghost_global_ordered_9999", // in map, not in index
	}
	partFor := func(node string) string {
		if node == "6g8by" {
			return "major"
		}
		return ""
	}
	// Input order preserved: found, not-yet, unresolved.
	got := Locate([]string{"ACC_FOUND", "ACC_NOTYET", "ACC_MISSING"}, m, byName, partFor)
	if len(got) != 3 {
		t.Fatalf("got %d results, want 3", len(got))
	}
	if got[0].Status != LocateFound || got[0].Batch != "Escherichia_coli_global_ordered_0001" ||
		got[0].Part != "major" || got[0].URL != "https://osf.io/download/ec1/" {
		t.Errorf("found row wrong: %+v", got[0])
	}
	if got[1].Status != LocateNotYetAvailable || got[1].Batch != "Ghost_global_ordered_9999" {
		t.Errorf("not-yet row wrong: %+v", got[1])
	}
	if got[2].Status != LocateUnresolved || got[2].Batch != "" {
		t.Errorf("unresolved row wrong: %+v", got[2])
	}
}
