package columns

import "testing"

func TestLookupReturnsColumnMetadata(t *testing.T) {
	c, ok := Lookup("N50")
	if !ok {
		t.Fatal("Lookup(\"N50\") returned false, want a known column")
	}
	if c.Source != AssemblyStats {
		t.Errorf("N50 Source = %q, want %q", c.Source, AssemblyStats)
	}
	if !c.InIndex {
		t.Error("N50 InIndex = false, want true: N50 is served by the SQLite index")
	}
}

func TestLookupMarksParquetOnlyColumns(t *testing.T) {
	c, ok := Lookup("Adjusted_ANI")
	if !ok {
		t.Fatal("Lookup(\"Adjusted_ANI\") returned false, want a known column")
	}
	if c.Source != Sylph {
		t.Errorf("Adjusted_ANI Source = %q, want %q", c.Source, Sylph)
	}
	if c.InIndex {
		t.Error("Adjusted_ANI InIndex = true, want false: sylph columns are not in the index")
	}
}

func TestLookupIsCaseSensitive(t *testing.T) {
	if _, ok := Lookup("n50"); ok {
		t.Error("Lookup(\"n50\") returned true, want false: column names are case-sensitive")
	}
}

func TestLookupRejectsUnknownColumn(t *testing.T) {
	if _, ok := Lookup("N5O"); ok {
		t.Error("Lookup(\"N5O\") returned true, want false")
	}
}

func TestAllIsGroupedBySourceInCanonicalOrder(t *testing.T) {
	want := []Source{Assembly, AssemblyStats, CheckM2, Sylph, MLST, ENA}

	var order []Source
	seen := make(map[Source]bool)
	for _, c := range All() {
		if !seen[c.Source] {
			seen[c.Source] = true
			order = append(order, c.Source)
		}
	}

	if len(order) != len(want) {
		t.Fatalf("All() covers %d sources %v, want %d %v", len(order), order, len(want), want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("source %d = %q, want %q", i, order[i], want[i])
		}
	}
}

func TestEveryColumnHasADescription(t *testing.T) {
	for _, c := range All() {
		if c.Description == "" {
			t.Errorf("column %q has no description", c.Name)
		}
	}
}

func TestNamesMatchesAll(t *testing.T) {
	names := Names()
	all := All()
	if len(names) != len(all) {
		t.Fatalf("Names() has %d entries, All() has %d", len(names), len(all))
	}
	for i := range all {
		if names[i] != all[i].Name {
			t.Errorf("Names()[%d] = %q, want %q", i, names[i], all[i].Name)
		}
	}
}
