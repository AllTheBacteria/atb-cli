package runs_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	parquetgo "github.com/parquet-go/parquet-go"

	pq "github.com/allthebacteria/atb-cli/internal/parquet"
	"github.com/allthebacteria/atb-cli/internal/runs"
)

// writeRunParquet builds a run.parquet fixture from run/sample pairs. The
// sample value may be a comma-joined list, which is how the real table records
// a run that carries more than one sample.
func writeRunParquet(t *testing.T, pairs [][2]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "run.parquet")
	f, err := os.Create(path)
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
	return path
}

func TestResolveMapsRunsToSamples(t *testing.T) {
	path := writeRunParquet(t, [][2]string{
		{"ERR1000001", "SAMEA1000001"},
		{"ERR1000002", "SAMEA1000002"},
		{"SRR2000001", "SAMN2000001"},
	})

	got, err := runs.Resolve(path, []string{"ERR1000001", "SRR2000001"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	want := []string{"SAMEA1000001", "SAMN2000001"}
	if !slices.Equal(got.Samples, want) {
		t.Errorf("Samples = %v, want %v", got.Samples, want)
	}
	if len(got.Missing) != 0 {
		t.Errorf("Missing = %v, want none", got.Missing)
	}
}

// A multiplexed run names several samples in one comma-joined field. Treating
// that field as a single accession loses every sample behind it: 3,651 rows of
// the published table are joined this way, covering 10,037 distinct samples.
func TestResolveExpandsCommaJoinedSamples(t *testing.T) {
	path := writeRunParquet(t, [][2]string{
		{"ERR016606", "SAMEA970816,SAMEA970817,SAMEA970818"},
	})

	got, err := runs.Resolve(path, []string{"ERR016606"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	want := []string{"SAMEA970816", "SAMEA970817", "SAMEA970818"}
	if !slices.Equal(got.Samples, want) {
		t.Errorf("Samples = %v, want %v", got.Samples, want)
	}
}

func TestResolveTrimsSpaceAroundJoinedSamples(t *testing.T) {
	path := writeRunParquet(t, [][2]string{
		{"ERR3000001", "SAMEA3000001, SAMEA3000002 ,SAMEA3000003"},
	})

	got, err := runs.Resolve(path, []string{"ERR3000001"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	want := []string{"SAMEA3000001", "SAMEA3000002", "SAMEA3000003"}
	if !slices.Equal(got.Samples, want) {
		t.Errorf("Samples = %v, want %v", got.Samples, want)
	}
}

func TestResolveReturnsEachSampleOnce(t *testing.T) {
	path := writeRunParquet(t, [][2]string{
		{"DRR015756", "SAMD00000585"},
		{"DRR015757", "SAMD00000585"},
	})

	got, err := runs.Resolve(path, []string{"DRR015756", "DRR015757"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	want := []string{"SAMD00000585"}
	if !slices.Equal(got.Samples, want) {
		t.Errorf("Samples = %v, want %v", got.Samples, want)
	}
}

func TestResolveReportsRunsMissingFromTheTable(t *testing.T) {
	path := writeRunParquet(t, [][2]string{
		{"ERR1000001", "SAMEA1000001"},
	})

	got, err := runs.Resolve(path, []string{"ERR1000001", "ERR9999999", "SRR9999999"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if !slices.Equal(got.Samples, []string{"SAMEA1000001"}) {
		t.Errorf("Samples = %v, want [SAMEA1000001]", got.Samples)
	}
	want := []string{"ERR9999999", "SRR9999999"}
	if !slices.Equal(got.Missing, want) {
		t.Errorf("Missing = %v, want %v", got.Missing, want)
	}
}

func TestResolveAcceptsARepeatedRunAccession(t *testing.T) {
	path := writeRunParquet(t, [][2]string{
		{"ERR1000001", "SAMEA1000001"},
	})

	got, err := runs.Resolve(path, []string{"ERR1000001", "ERR1000001"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if !slices.Equal(got.Samples, []string{"SAMEA1000001"}) {
		t.Errorf("Samples = %v, want [SAMEA1000001]", got.Samples)
	}
	if len(got.Missing) != 0 {
		t.Errorf("Missing = %v, want none", got.Missing)
	}
}

// Scanning the 3.5 million row table takes seconds, so an empty request must
// not read it at all. A path that does not exist proves the read was skipped.
func TestResolveWithNoRunsDoesNotReadTheTable(t *testing.T) {
	got, err := runs.Resolve(filepath.Join(t.TempDir(), "absent.parquet"), nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(got.Samples) != 0 || len(got.Missing) != 0 {
		t.Errorf("got %+v, want an empty resolution", got)
	}
}

func TestResolveSkipsBlankSampleValues(t *testing.T) {
	path := writeRunParquet(t, [][2]string{
		{"ERR1000001", ""},
		{"ERR1000002", "SAMEA1000002"},
	})

	got, err := runs.Resolve(path, []string{"ERR1000001", "ERR1000002"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if !slices.Equal(got.Samples, []string{"SAMEA1000002"}) {
		t.Errorf("Samples = %v, want [SAMEA1000002]", got.Samples)
	}
	// The run exists in the table, so it is not missing - it just names no sample.
	if len(got.Missing) != 0 {
		t.Errorf("Missing = %v, want none", got.Missing)
	}
}

func TestResolveFailsWhenTheTableIsUnreadable(t *testing.T) {
	_, err := runs.Resolve(filepath.Join(t.TempDir(), "absent.parquet"), []string{"ERR1000001"})
	if err == nil {
		t.Fatal("Resolve: expected an error for a missing run.parquet")
	}
}
