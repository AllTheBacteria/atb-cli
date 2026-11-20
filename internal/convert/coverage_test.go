package convert

import (
	"strings"
	"testing"
	"unsafe"
)

func TestCoverage(t *testing.T) {
	samples := NewSampleSet()
	for _, s := range []string{"A", "B", "C", "D", "D"} {
		samples.Add(s)
	}
	if samples.Rows() != 5 || samples.Len() != 4 {
		t.Fatalf("SampleSet rows=%d len=%d, want 5 and 4", samples.Rows(), samples.Len())
	}

	c := samples.NewCoverage()
	for _, s := range []string{"A", "A", "B", "X", "X", "Y"} {
		c.Add(s)
	}
	want := CoverageReport{Rows: 6, Samples: 4, RowsNotInAssembly: 3, AssemblyMissing: 2}
	if got := c.Report(); got != want {
		t.Fatalf("Report() = %+v, want %+v", got, want)
	}
}

func TestSampleSetCopiesSamples(t *testing.T) {
	line := "S1\t" + strings.Repeat("x", 1000)
	samples := NewSampleSet()
	samples.Add(strings.Split(line, "\t")[0])
	coverage := samples.NewCoverage()
	coverage.Add(strings.Split("S2\t"+line, "\t")[0])

	for key := range samples.index {
		if unsafe.StringData(key) == unsafe.StringData(line) {
			t.Error("SampleSet key shares memory with the TSV line")
		}
	}
	for key := range coverage.unknown {
		if key != "S2" {
			t.Errorf("unknown sample %q, want S2", key)
		}
	}
}
