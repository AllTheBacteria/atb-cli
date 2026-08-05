package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	parquetgo "github.com/parquet-go/parquet-go"

	pq "github.com/allthebacteria/atb-cli/internal/parquet"
	"github.com/allthebacteria/atb-cli/internal/query"
)

// writeRunTable builds a run.parquet fixture in dir from run/sample pairs. The
// sample value may be a comma-joined list, which is how the real table records
// a run that carries more than one sample.
func writeRunTable(t *testing.T, dir string, pairs [][2]string) {
	t.Helper()

	f, err := os.Create(filepath.Join(dir, "run.parquet"))
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	w := parquetgo.NewGenericWriter[pq.RunRow](f)
	for _, p := range pairs {
		row := pq.RunRow{RunAccession: p[0], SampleAccession: p[1], Pass: 1}
		if _, err := w.Write([]pq.RunRow{row}); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}
}

func TestResolveRunFilterAddsResolvedSamples(t *testing.T) {
	dir := t.TempDir()
	writeRunTable(t, dir, [][2]string{
		{"ERR1000001", "SAMEA1"},
		{"SRR2000001", "SAMN2"},
		{"DRR3000001", "SAMD3"},
	})

	filters := query.Filters{Runs: []string{"ERR1000001", "SRR2000001"}}
	var out bytes.Buffer
	if err := resolveRunFilter(&filters, dir, &out); err != nil {
		t.Fatalf("resolveRunFilter returned error: %v", err)
	}

	got := filters.SampleAccessions()
	want := []string{"SAMEA1", "SAMN2"}
	if len(got) != len(want) {
		t.Fatalf("SampleAccessions(): got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SampleAccessions()[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// A run may name more than one sample. The table records those as a
// comma-joined list, and every one of them must reach the sample filter.
func TestResolveRunFilterExpandsMultiplexedRuns(t *testing.T) {
	dir := t.TempDir()
	writeRunTable(t, dir, [][2]string{{"ERR1000001", "SAMEA1,SAMEA2,SAMEA3"}})

	filters := query.Filters{Runs: []string{"ERR1000001"}}
	if err := resolveRunFilter(&filters, dir, &bytes.Buffer{}); err != nil {
		t.Fatalf("resolveRunFilter returned error: %v", err)
	}

	if got := len(filters.SampleAccessions()); got != 3 {
		t.Errorf("SampleAccessions(): got %d entries (%v), want 3", got, filters.SampleAccessions())
	}
}

// Runs and samples name the same thing two ways, so they combine as a union,
// the same way --samples and --sample-file do.
func TestResolveRunFilterUnionsWithExistingSamples(t *testing.T) {
	dir := t.TempDir()
	writeRunTable(t, dir, [][2]string{{"ERR1000001", "SAMEA1"}})

	filters := query.Filters{Samples: []string{"SAMEA9"}, Runs: []string{"ERR1000001"}}
	if err := resolveRunFilter(&filters, dir, &bytes.Buffer{}); err != nil {
		t.Fatalf("resolveRunFilter returned error: %v", err)
	}

	got := filters.SampleAccessions()
	want := []string{"SAMEA9", "SAMEA1"}
	if len(got) != len(want) {
		t.Fatalf("SampleAccessions(): got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SampleAccessions()[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// A sample already named by --samples must not appear twice when a run
// resolves to it as well.
func TestResolveRunFilterDoesNotDuplicateASample(t *testing.T) {
	dir := t.TempDir()
	writeRunTable(t, dir, [][2]string{{"ERR1000001", "SAMEA9"}})

	filters := query.Filters{Samples: []string{"SAMEA9"}, Runs: []string{"ERR1000001"}}
	if err := resolveRunFilter(&filters, dir, &bytes.Buffer{}); err != nil {
		t.Fatalf("resolveRunFilter returned error: %v", err)
	}

	if got := filters.SampleAccessions(); len(got) != 1 || got[0] != "SAMEA9" {
		t.Errorf("SampleAccessions(): got %v, want [SAMEA9]", got)
	}
}

// Runs the table does not list are reported, but they do not fail the query as
// long as something resolved.
func TestResolveRunFilterWarnsAboutMissingRuns(t *testing.T) {
	dir := t.TempDir()
	writeRunTable(t, dir, [][2]string{{"ERR1000001", "SAMEA1"}})

	filters := query.Filters{Runs: []string{"ERR1000001", "ERR9999999"}}
	var out bytes.Buffer
	if err := resolveRunFilter(&filters, dir, &out); err != nil {
		t.Fatalf("resolveRunFilter returned error: %v", err)
	}

	msg := out.String()
	if !strings.Contains(msg, "ERR9999999") {
		t.Errorf("expected the missing accession in the warning, got %q", msg)
	}
	if strings.Contains(msg, "ERR1000001") {
		t.Errorf("resolved accession should not be reported as missing, got %q", msg)
	}
}

// A run filter that resolves to nothing must fail. Leaving the sample filter
// empty would silently widen the query to the whole database.
func TestResolveRunFilterFailsWhenNothingResolves(t *testing.T) {
	dir := t.TempDir()
	writeRunTable(t, dir, [][2]string{{"ERR1000001", "SAMEA1"}})

	filters := query.Filters{Runs: []string{"ERR9999999"}}
	err := resolveRunFilter(&filters, dir, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected an error when no run accession resolves")
	}
	if len(filters.SampleAccessions()) != 0 {
		t.Errorf("sample filter should stay empty, got %v", filters.SampleAccessions())
	}
}

// The same rule applies when --samples is also set: the user asked for runs and
// got none, which is worth stopping for.
func TestResolveRunFilterFailsWhenNothingResolvesAlongsideSamples(t *testing.T) {
	dir := t.TempDir()
	writeRunTable(t, dir, [][2]string{{"ERR1000001", "SAMEA1"}})

	filters := query.Filters{Samples: []string{"SAMEA9"}, Runs: []string{"ERR9999999"}}
	if err := resolveRunFilter(&filters, dir, &bytes.Buffer{}); err == nil {
		t.Fatal("expected an error when no run accession resolves")
	}
}

// run.parquet is a core table, but a data directory fetched before it was
// added will not have it. Say what to do about that.
func TestResolveRunFilterReportsAMissingTable(t *testing.T) {
	filters := query.Filters{Runs: []string{"ERR1000001"}}
	err := resolveRunFilter(&filters, t.TempDir(), &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected an error when run.parquet is absent")
	}
	if !strings.Contains(err.Error(), "atb fetch") {
		t.Errorf("error should point at atb fetch, got %q", err.Error())
	}
}

func TestResolveRunFilterIsANoOpWithoutRuns(t *testing.T) {
	// An empty directory: reaching for run.parquet at all would fail here.
	filters := query.Filters{Samples: []string{"SAMEA9"}}
	if err := resolveRunFilter(&filters, t.TempDir(), &bytes.Buffer{}); err != nil {
		t.Fatalf("resolveRunFilter returned error: %v", err)
	}

	if got := filters.SampleAccessions(); len(got) != 1 || got[0] != "SAMEA9" {
		t.Errorf("SampleAccessions(): got %v, want [SAMEA9]", got)
	}
}
