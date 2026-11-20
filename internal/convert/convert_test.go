package convert

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/format"
)

var testTable = Table{
	Name:   "test",
	Input:  "test.tsv.gz",
	Output: "test.parquet",
	Columns: []Column{
		{Name: "sample_accession", Kind: String},
		{Name: "count", Kind: Int64},
		{Name: "ratio", Kind: Float64},
		{Name: "note", Kind: String},
		{Name: "model", Kind: String, NAIsNull: true},
		{Name: "flag", Kind: Int64, Want: "1"},
	},
}

const (
	testHeader = "sample_accession\tcount\tratio\tnote\tmodel\tflag"
	testRow    = "S1\t5\t0.5\tn\tm\t1"
)

func writeGzip(t *testing.T, path string, lines []string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	for _, line := range lines {
		if _, err := io.WriteString(gz, line+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// convertLines writes lines to a gzipped TSV named after table.Input, converts
// it and returns the output path.
func convertLines(t *testing.T, table Table, lines []string, onSample func(string)) (string, Result, error) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, table.Input)
	writeGzip(t, in, lines)
	out := filepath.Join(dir, table.Output)
	result, err := Convert(table, in, out, onSample)
	return out, result, err
}

// readCells returns every cell of the parquet file at path: nil for null,
// otherwise a string, int64 or float64.
func readCells(t *testing.T, path string) [][]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := parquet.NewReader(f)
	defer r.Close()
	var cells [][]any
	buf := make([]parquet.Row, 16)
	for {
		n, err := r.ReadRows(buf)
		for _, row := range buf[:n] {
			cells = append(cells, rowCells(t, row))
		}
		if errors.Is(err, io.EOF) {
			return cells
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func rowCells(t *testing.T, row parquet.Row) []any {
	t.Helper()
	cells := make([]any, len(row))
	for i, v := range row {
		switch {
		case v.IsNull():
			cells[i] = nil
		case v.Kind() == parquet.ByteArray:
			cells[i] = string(v.ByteArray())
		case v.Kind() == parquet.Int64:
			cells[i] = v.Int64()
		case v.Kind() == parquet.Double:
			cells[i] = v.Double()
		default:
			t.Fatalf("column %d: unexpected kind %v", i, v.Kind())
		}
	}
	return cells
}

func TestConvertRejectsBadInput(t *testing.T) {
	tests := []struct {
		name    string
		lines   []string
		wantErr string
	}{
		{"renamed header column", []string{"sample_accession\tcount\tratio\tnotes\tmodel\tflag", testRow},
			`test.tsv.gz line 1: header column 4 is "notes", want "note"`},
		{"reordered header", []string{"sample_accession\tratio\tcount\tnote\tmodel\tflag", testRow},
			`line 1: header column 2 is "ratio", want "count"`},
		{"missing header column", []string{"sample_accession\tcount\tratio\tnote\tmodel", testRow},
			"line 1: header has 5 columns, want 6: missing flag"},
		{"extra header column", []string{testHeader + "\textra", testRow},
			`line 1: header has 7 columns, want 6: unexpected "extra"`},
		{"CRLF header", []string{testHeader + "\r", testRow},
			`line 1: header column 6 is "flag\r", want "flag"`},
		{"empty file", nil, "test.tsv.gz: file is empty"},
		{"short row", []string{testHeader, testRow, "S2\t5\t0.5\tn\tm"},
			"test.tsv.gz line 3: row has 5 fields, want 6"},
		{"long row", []string{testHeader, testRow + "\textra"},
			"test.tsv.gz line 2: row has 7 fields, want 6"},
		{"blank line", []string{testHeader, testRow, "", testRow},
			"line 3: row has 1 fields, want 6"},
		{"fractional int", []string{testHeader, testRow, testRow, "S1\t1.5\t0.5\tn\tm\t1"},
			`test.tsv.gz line 4: column count: invalid int64 value "1.5"`},
		{"hex int", []string{testHeader, "S1\t0x10\t0.5\tn\tm\t1"}, `line 2: column count: invalid int64 value "0x10"`},
		{"padded int", []string{testHeader, "S1\t 5\t0.5\tn\tm\t1"}, `column count: invalid int64 value " 5"`},
		{"word float", []string{testHeader, "S1\t5\tabc\tn\tm\t1"}, `line 2: column ratio: invalid float64 value "abc"`},
		{"NaN float", []string{testHeader, "S1\t5\tNaN\tn\tm\t1"}, `column ratio: invalid float64 value "NaN"`},
		{"Inf float", []string{testHeader, "S1\t5\t-Inf\tn\tm\t1"}, `column ratio: invalid float64 value "-Inf"`},
		{"underscore float", []string{testHeader, "S1\t5\t1_000.5\tn\tm\t1"}, `column ratio: invalid float64 value "1_000.5"`},
		{"hex float", []string{testHeader, "S1\t5\t0x1p-2\tn\tm\t1"}, `column ratio: invalid float64 value "0x1p-2"`},
		{"overflow float", []string{testHeader, "S1\t5\t1e400\tn\tm\t1"}, `column ratio: invalid float64 value "1e400"`},
		{"wanted value differs", []string{testHeader, testRow, "S2\t5\t0.5\tn\tm\t0"},
			`test.tsv.gz line 3: column flag: value "0", want "1"`},
		{"wanted value empty", []string{testHeader, "S1\t5\t0.5\tn\tm\t"}, `line 2: column flag: value "", want "1"`},
		{"wanted value NA", []string{testHeader, "S1\t5\t0.5\tn\tm\tNA"}, `line 2: column flag: value "NA", want "1"`},
		{"invalid UTF-8", []string{testHeader, "S1\t5\t0.5\t\xff\tm\t1"}, `line 2: column note: invalid UTF-8 value "\xff"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := convertLines(t, testTable, tc.lines, nil)
			if err == nil {
				t.Fatalf("Convert succeeded, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.wantErr)
			}
			for _, path := range []string{out, out + ".tmp"} {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("%s exists after a failed conversion", filepath.Base(path))
				}
			}
		})
	}
}

func TestConvertReplacesOutputOnlyOnSuccess(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, testTable.Input)
	out := filepath.Join(dir, testTable.Output)
	if err := os.WriteFile(out, []byte("previous"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGzip(t, in, []string{testHeader, "S1\tx\t0.5\tn\tm\t1"})
	if _, err := Convert(testTable, in, out, nil); err == nil {
		t.Fatal("Convert succeeded on a bad row")
	}
	if got, _ := os.ReadFile(out); string(got) != "previous" {
		t.Fatalf("output changed after a failed conversion")
	}

	writeGzip(t, in, []string{testHeader, testRow})
	result, err := Convert(testTable, in, out, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != (Result{InputRows: 1, OutputRows: 1}) {
		t.Fatalf("result = %+v", result)
	}
	if got := readCells(t, out); len(got) != 1 {
		t.Fatalf("read %d rows, want 1", len(got))
	}
}

func TestNullRules(t *testing.T) {
	lines := []string{
		testHeader,
		"S1\t5\t0.5\tNA\tNA\t1",
		"S2\tNA\tNA\t\t\t1",
		"S3\t\t\tNone\tNone\t1",
		"\t-7\t-1.5e-3\t \tNA \t1",
		"S5\t+8\t.5\tna\tn/a\t1",
	}
	want := [][]any{
		{"S1", int64(5), 0.5, "NA", nil, int64(1)},
		{"S2", nil, nil, nil, nil, int64(1)},
		{"S3", nil, nil, "None", "None", int64(1)},
		{nil, int64(-7), -0.0015, " ", "NA ", int64(1)},
		{"S5", int64(8), 0.5, "na", "n/a", int64(1)},
	}
	out, result, err := convertLines(t, testTable, lines, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.InputRows != 5 || result.OutputRows != 5 {
		t.Fatalf("result = %+v, want 5 rows in and out", result)
	}
	if got := readCells(t, out); !reflect.DeepEqual(got, want) {
		t.Fatalf("cells:\n got %v\nwant %v", got, want)
	}
}

// sampleValue returns a TSV value for column c in row that differs from the
// value of every other column, so a swapped column fails a comparison.
func sampleValue(c Column, column, row int) string {
	switch {
	case c.Want != "":
		return c.Want
	case c.Kind == Int64:
		return fmt.Sprint(row*1000 + column)
	case c.Kind == Float64:
		return fmt.Sprintf("%d.25", row*1000+column)
	}
	return fmt.Sprintf("%s-%d", c.Name, row)
}

func header(table Table) string {
	names := make([]string, len(table.Columns))
	for i, c := range table.Columns {
		names[i] = c.Name
	}
	return strings.Join(names, "\t")
}

func TestReleaseTableNARules(t *testing.T) {
	tests := []struct {
		table    Table
		column   string
		value    string
		wantNull bool
	}{
		{checkM2, "Completeness_Model_Used", "NA", true},
		{checkM2, "Additional_Notes", "NA", true},
		{checkM2, "Additional_Notes", "None", false},
		{checkM2, "Completeness_Model_Used", "", true},
		{checkM2, "Contamination", "NA", true},
		{assemblyStats, "N50", "NA", true},
		{assembly, "comments", "NA", false},
		{assembly, "sylph_species", "NA", false},
		{assembly, "hq_filter", "", true},
		{sylph, "Species", "NA", false},
		{sylph, "Median_cov", "", true},
		{run, "failure_reasons", "NA", false},
		{run, "comments", "None", false},
	}
	for _, tc := range tests {
		t.Run(tc.table.Name+"/"+tc.column+"="+tc.value, func(t *testing.T) {
			index := tc.table.columnIndex(tc.column)
			if index < 0 {
				t.Fatalf("table %s has no column %s", tc.table.Name, tc.column)
			}
			fields := make([]string, len(tc.table.Columns))
			for i, c := range tc.table.Columns {
				fields[i] = sampleValue(c, i, 1)
			}
			fields[index] = tc.value
			out, _, err := convertLines(t, tc.table, []string{header(tc.table), strings.Join(fields, "\t")}, nil)
			if err != nil {
				t.Fatal(err)
			}
			got := readCells(t, out)[0][index]
			switch {
			case tc.wantNull && got != nil:
				t.Errorf("cell = %v, want null", got)
			case !tc.wantNull && got != tc.value:
				t.Errorf("cell = %v, want %q", got, tc.value)
			}
		})
	}
}

func TestSchemaColumnTypes(t *testing.T) {
	for _, table := range Tables {
		t.Run(table.Name, func(t *testing.T) {
			schema, err := Schema(table)
			if err != nil {
				t.Fatal(err)
			}
			fields := schema.Fields()
			if len(fields) != len(table.Columns) {
				t.Fatalf("schema has %d fields, want %d", len(fields), len(table.Columns))
			}
			for i, c := range table.Columns {
				f := fields[i]
				if f.Name() != c.Name || !f.Optional() {
					t.Errorf("field %d = %s optional=%v, want optional %s", i, f.Name(), f.Optional(), c.Name)
				}
				kind := f.Type().Kind()
				switch c.Kind {
				case String:
					if kind != parquet.ByteArray || f.Type().LogicalType() == nil || f.Type().LogicalType().UTF8 == nil {
						t.Errorf("%s: type %v, want UTF8 byte array", c.Name, f.Type())
					}
				case Int64:
					if kind != parquet.Int64 {
						t.Errorf("%s: kind %v, want INT64", c.Name, kind)
					}
				case Float64:
					if kind != parquet.Double {
						t.Errorf("%s: kind %v, want DOUBLE", c.Name, kind)
					}
				}
			}
		})
	}
}

func TestRowGroupSize(t *testing.T) {
	table := Table{
		Name:    "groups",
		Input:   "groups.tsv.gz",
		Output:  "groups.parquet",
		Columns: []Column{{Name: "sample_accession", Kind: String}, {Name: "n", Kind: Int64}},
	}
	lines := []string{"sample_accession\tn"}
	for i := range RowsPerRowGroup + 1 {
		lines = append(lines, fmt.Sprintf("S%d\t%d", i, i))
	}
	out, _, err := convertLines(t, table, lines, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sizes []int64
	for _, rg := range openParquet(t, out).RowGroups() {
		sizes = append(sizes, rg.NumRows())
	}
	if want := []int64{RowsPerRowGroup, 1}; !reflect.DeepEqual(sizes, want) {
		t.Fatalf("row group sizes = %v, want %v", sizes, want)
	}
}

func TestColumnChunkEncoding(t *testing.T) {
	lines := []string{
		header(testTable),
		"S1\t1\t0.5\tnote\tmodel\t1",
		"S2\t2\t1.5\tnote\tmodel\t1",
	}
	out, _, err := convertLines(t, testTable, lines, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, rg := range openParquet(t, out).Metadata().RowGroups {
		for i, chunk := range rg.Columns {
			c := testTable.Columns[i]
			if chunk.MetaData.Codec != format.Zstd {
				t.Errorf("%s: codec %v, want ZSTD", c.Name, chunk.MetaData.Codec)
			}
			if c.Kind == String && !slices.Equal(chunk.MetaData.Encoding, []format.Encoding{format.Plain, format.RLE}) {
				t.Errorf("%s: encodings %v, want PLAIN and RLE", c.Name, chunk.MetaData.Encoding)
			}
		}
	}
}

func openParquet(t *testing.T, path string) *parquet.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		t.Fatal(err)
	}
	return pf
}

func TestConvertReportsSamples(t *testing.T) {
	var samples []string
	_, _, err := convertLines(t, testTable, []string{testHeader, testRow, "S2\t5\t0.5\tn\tm\t1"}, func(s string) {
		samples = append(samples, s)
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"S1", "S2"}; !reflect.DeepEqual(samples, want) {
		t.Fatalf("samples = %v, want %v", samples, want)
	}
}
