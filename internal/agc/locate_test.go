package agc

import (
	"testing"
)

func TestLocateReportsSpeciesAndNode(t *testing.T) {
	m := ArchiveMap{
		"SAMEA1": "atb.assembly.202505_all.batch.0001", // found
		"SAMEA2": "atb.assembly.202505_all.batch.0999", // maps, batch not in index
	}
	byName := map[string]ArchiveRef{
		"atb.assembly.202505_all.batch.0001": {
			Name:    "atb.assembly.202505_all.batch.0001",
			URL:     "https://osf.io/download/aaa/",
			Node:    "4jq8u",
			Species: "Escherichia_coli",
		},
	}
	got := Locate([]string{"SAMEA1", "SAMEA2", "SAMEA3"}, m, byName)
	if len(got) != 3 {
		t.Fatalf("len: got %d, want 3", len(got))
	}
	if got[0].Status != LocateFound || got[0].Species != "Escherichia_coli" || got[0].Node != "4jq8u" {
		t.Errorf("row0: got %+v, want found E. coli on 4jq8u", got[0])
	}
	if got[1].Status != LocateNotYetAvailable || got[1].Batch != "atb.assembly.202505_all.batch.0999" {
		t.Errorf("row1: got %+v, want not-yet-available", got[1])
	}
	if got[2].Status != LocateUnresolved {
		t.Errorf("row2: got %+v, want unresolved", got[2])
	}
}
