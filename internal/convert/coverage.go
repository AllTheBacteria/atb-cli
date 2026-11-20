package convert

import "strings"

// SampleSet holds the distinct sample accessions of the assembly table.
type SampleSet struct {
	index map[string]int
	rows  int64
}

func NewSampleSet() *SampleSet {
	return &SampleSet{index: make(map[string]int)}
}

// Add records one assembly row with the given sample accession.
func (s *SampleSet) Add(sample string) {
	s.rows++
	if _, ok := s.index[sample]; !ok {
		s.index[strings.Clone(sample)] = len(s.index)
	}
}

// Rows returns the number of rows added.
func (s *SampleSet) Rows() int64 { return s.rows }

// Len returns the number of distinct samples.
func (s *SampleSet) Len() int { return len(s.index) }

// Coverage compares the sample accessions of one table with a SampleSet.
type Coverage struct {
	set     *SampleSet
	seen    []bool
	unknown map[string]struct{}
	report  CoverageReport
}

// CoverageReport is the result of a Coverage.
type CoverageReport struct {
	Rows              int64
	Samples           int   // distinct sample accessions in the table
	RowsNotInAssembly int64 // rows whose sample is not in the SampleSet
	AssemblyMissing   int   // SampleSet samples with no row in the table
}

func (s *SampleSet) NewCoverage() *Coverage {
	return &Coverage{
		set:     s,
		seen:    make([]bool, len(s.index)),
		unknown: make(map[string]struct{}),
	}
}

// Add records one row with the given sample accession.
func (c *Coverage) Add(sample string) {
	c.report.Rows++
	if i, ok := c.set.index[sample]; ok {
		c.seen[i] = true
		return
	}
	c.report.RowsNotInAssembly++
	if _, ok := c.unknown[sample]; !ok {
		c.unknown[strings.Clone(sample)] = struct{}{}
	}
}

// Report returns the row and sample counts recorded so far.
func (c *Coverage) Report() CoverageReport {
	seen := 0
	for _, ok := range c.seen {
		if ok {
			seen++
		}
	}
	r := c.report
	r.Samples = seen + len(c.unknown)
	r.AssemblyMissing = len(c.seen) - seen
	return r
}
