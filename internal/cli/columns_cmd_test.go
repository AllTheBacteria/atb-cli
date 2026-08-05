package cli

import (
	"encoding/json"
	"strings"
	"testing"

	cols "github.com/allthebacteria/atb-cli/internal/columns"
)

// lineFor returns the output line listing the named column.
func lineFor(t *testing.T, out, name string) string {
	t.Helper()

	for _, line := range strings.Split(out, "\n") {
		for _, field := range strings.Fields(line) {
			if field == name {
				return line
			}
		}
	}
	t.Fatalf("no line lists column %q in:\n%s", name, out)
	return ""
}

// atb columns takes no data directory: it must answer from the registry alone,
// before a user has downloaded anything.
func TestColumnsListsEveryRegisteredColumn(t *testing.T) {
	stdout, stderr, err := runCmd("columns")
	if err != nil {
		t.Fatalf("columns failed: %v\nstderr: %s", err, stderr)
	}

	for _, name := range cols.Names() {
		if !strings.Contains(stdout, name) {
			t.Errorf("column %q is missing from the listing:\n%s", name, stdout)
		}
	}
}

func TestColumnsGroupsBySource(t *testing.T) {
	stdout, _, err := runCmd("columns")
	if err != nil {
		t.Fatalf("columns failed: %v", err)
	}

	for _, source := range []string{"assembly_stats", "checkm2", "sylph", "mlst", "ena_20250506"} {
		if !strings.Contains(stdout, source) {
			t.Errorf("source %q has no heading in:\n%s", source, stdout)
		}
	}
}

func TestColumnsMarksColumnsTheIndexCannotAnswer(t *testing.T) {
	stdout, _, err := runCmd("columns")
	if err != nil {
		t.Fatalf("columns failed: %v", err)
	}

	if marked := lineFor(t, stdout, "Adjusted_ANI"); !strings.HasPrefix(strings.TrimSpace(marked), "*") {
		t.Errorf("Adjusted_ANI is not in the index but its line is unmarked: %q", marked)
	}
	if unmarked := lineFor(t, stdout, "N50"); strings.HasPrefix(strings.TrimSpace(unmarked), "*") {
		t.Errorf("N50 is in the index but its line is marked: %q", unmarked)
	}
}

func TestColumnsTSV(t *testing.T) {
	stdout, _, err := runCmd("columns", "--format", "tsv")
	if err != nil {
		t.Fatalf("columns --format tsv failed: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if want := len(cols.All()) + 1; len(lines) != want {
		t.Fatalf("got %d lines, want %d (header plus every column)", len(lines), want)
	}
	if lines[0] != "name\tsource\tin_index\tdescription" {
		t.Errorf("header = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "sample_accession\tassembly\ttrue\t") {
		t.Errorf("first row = %q", lines[1])
	}
}

func TestColumnsJSON(t *testing.T) {
	stdout, _, err := runCmd("columns", "--format", "json")
	if err != nil {
		t.Fatalf("columns --format json failed: %v", err)
	}

	var got []map[string]string
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, stdout)
	}
	if len(got) != len(cols.All()) {
		t.Fatalf("got %d entries, want %d", len(got), len(cols.All()))
	}
	if got[0]["name"] != "sample_accession" || got[0]["in_index"] != "true" {
		t.Errorf("first entry = %v", got[0])
	}
}

func TestColumnsRejectsUnknownFormat(t *testing.T) {
	if _, _, err := runCmd("columns", "--format", "yaml"); err == nil {
		t.Error("columns --format yaml succeeded, want an error")
	}
}
