package agc

import (
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/osf"
)

func sampleAGCIndex() *osf.Index {
	return &osf.Index{Entries: []osf.Entry{
		{Project: "Acinetobacter_baylyi", ProjectID: "z7q5y",
			Filename: "Acinetobacter_baylyi_global_ordered_0001.agc",
			URL:      "https://osf.io/download/aaa/", MD5: "md5aaa", SizeMB: 3.89},
		{Project: "Acinetobacter_baylyi", ProjectID: "z7q5y",
			Filename: "Acinetobacter_baylyi_global_ordered_0002.agc",
			URL:      "https://osf.io/download/bbb/", MD5: "md5bbb", SizeMB: 4.10},
		{Project: "Streptococcus_suis_AA", ProjectID: "z7q5y",
			Filename: "Streptococcus_suis_AA_global_ordered_0001.agc",
			URL:      "https://osf.io/download/ccc/", MD5: "md5ccc", SizeMB: 10.4},
		{Project: "subthreshold_remainder", ProjectID: "z7q5y",
			Filename: "subthreshold_remainder_global_ordered_0091.agc",
			URL:      "https://osf.io/download/ddd/", MD5: "md5ddd", SizeMB: 114.9},
	}}
}

func TestRefsFromIndex(t *testing.T) {
	refs := RefsFromIndex(sampleAGCIndex())
	if len(refs) != 4 {
		t.Fatalf("got %d refs, want 4", len(refs))
	}

	// Keyed by archive stem (no ".agc"), the fetch engine's group key space.
	ref, ok := refs["Acinetobacter_baylyi_global_ordered_0001"]
	if !ok {
		t.Fatalf("expected key keyed by stem without .agc not found")
	}
	if ref.Name != "Acinetobacter_baylyi_global_ordered_0001" {
		t.Errorf("Name = %q, want stem without .agc", ref.Name)
	}
	if ref.URL != "https://osf.io/download/aaa/" {
		t.Errorf("URL = %q", ref.URL)
	}
	if ref.MD5 != "md5aaa" {
		t.Errorf("MD5 = %q", ref.MD5)
	}
	if ref.SizeMB != 3.89 {
		t.Errorf("SizeMB = %v, want 3.89", ref.SizeMB)
	}

	// No key should retain the .agc extension.
	for k := range refs {
		if strings.HasSuffix(k, ".agc") {
			t.Errorf("ref key %q must not keep the .agc extension", k)
		}
	}
}

func TestWholeArchiveGroups(t *testing.T) {
	refs := SelectBySpecies(sampleAGCIndex(), "Acinetobacter baylyi") // 2 refs

	groups, byName := WholeArchiveGroups(refs)

	if len(groups) != 2 {
		t.Fatalf("groups: got %d, want 2", len(groups))
	}
	if len(byName) != 2 {
		t.Fatalf("byName: got %d, want 2", len(byName))
	}
	for name, accs := range groups {
		// Whole-archive sentinel: no specific accessions, so the engine runs
		// `agc getcol` over the entire batch.
		if len(accs) != 0 {
			t.Errorf("group %q should carry no accessions (whole-archive), got %v", name, accs)
		}
		ref, ok := byName[name]
		if !ok {
			t.Errorf("byName missing key %q present in groups", name)
			continue
		}
		if ref.Name != name {
			t.Errorf("byName[%q].Name = %q, want key to equal Name", name, ref.Name)
		}
		if ref.URL == "" || ref.MD5 == "" {
			t.Errorf("byName[%q] missing URL/MD5: %+v", name, ref)
		}
	}

	// Empty selection must yield empty (non-nil) maps, not a nil deref.
	g, b := WholeArchiveGroups(nil)
	if g == nil || b == nil {
		t.Errorf("empty selection must return non-nil maps, got groups=%v byName=%v", g, b)
	}
	if len(g) != 0 || len(b) != 0 {
		t.Errorf("empty selection must return empty maps, got %d/%d", len(g), len(b))
	}
}

func TestSelectBySpecies(t *testing.T) {
	idx := sampleAGCIndex()

	// Space form, exact species: both batches, none of the others.
	got := SelectBySpecies(idx, "Acinetobacter baylyi")
	if len(got) != 2 {
		t.Fatalf("Acinetobacter baylyi: got %d refs, want 2", len(got))
	}
	for _, r := range got {
		if !strings.HasPrefix(r.Name, "Acinetobacter_baylyi_global_ordered_") {
			t.Errorf("unexpected ref %q in species selection", r.Name)
		}
		if r.URL == "" || r.MD5 == "" {
			t.Errorf("ref %q missing URL/MD5", r.Name)
		}
	}

	// Case-insensitive, underscore form.
	if n := len(SelectBySpecies(idx, "acinetobacter_baylyi")); n != 2 {
		t.Errorf("case-insensitive underscore: got %d, want 2", n)
	}

	// GTDB letter-suffix species requires its full name.
	if n := len(SelectBySpecies(idx, "Streptococcus suis AA")); n != 1 {
		t.Errorf("Streptococcus suis AA: got %d, want 1", n)
	}

	// A bare prefix must NOT match the suffixed species (exact semantics).
	if n := len(SelectBySpecies(idx, "Streptococcus suis")); n != 0 {
		t.Errorf("bare prefix should not match suffixed species, got %d", n)
	}

	// Unknown species: empty slice, not an error.
	if n := len(SelectBySpecies(idx, "Escherichia coli")); n != 0 {
		t.Errorf("unknown species: got %d, want 0", n)
	}
}
