package convert

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress/zstd"
)

// RowsPerRowGroup is the number of rows in each row group of the output.
const RowsPerRowGroup = 100_000

const rowsPerWrite = 1024

// Result counts the rows of one conversion.
type Result struct {
	InputRows  int64
	OutputRows int64
}

// Convert reads the gzipped TSV at inPath and writes it to outPath as the
// parquet file described by t. When onSample is not nil it is called with the
// sample_accession of every row. Convert writes to outPath.tmp, checks the row
// count in its footer and then renames it to outPath. On error no file is left
// at outPath.tmp.
func Convert(t Table, inPath, outPath string, onSample func(string)) (Result, error) {
	schema, err := Schema(t)
	if err != nil {
		return Result{}, err
	}
	sampleColumn := -1
	if onSample != nil {
		if sampleColumn = t.columnIndex("sample_accession"); sampleColumn < 0 {
			return Result{}, fmt.Errorf("table %s has no sample_accession column", t.Name)
		}
	}

	in, err := os.Open(inPath)
	if err != nil {
		return Result{}, err
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", inPath, err)
	}
	defer gz.Close()

	tmpPath := outPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return Result{}, err
	}
	result, err := writeTable(t, schema, gz, filepath.Base(inPath), out, sampleColumn, onSample)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		result.OutputRows, err = verify(tmpPath, t, result.InputRows)
	}
	if err == nil {
		err = os.Rename(tmpPath, outPath)
	}
	if err != nil {
		os.Remove(tmpPath)
		return Result{}, err
	}
	return result, nil
}

// Schema returns the parquet schema for t: one OPTIONAL column per TSV
// column, in TSV order. String columns are UTF8 byte arrays, Int64 columns
// are INT64 and Float64 columns are DOUBLE.
func Schema(t Table) (*parquet.Schema, error) {
	fields := make([]reflect.StructField, len(t.Columns))
	for i, c := range t.Columns {
		goType, ok := goTypes[c.Kind]
		if !ok {
			return nil, fmt.Errorf("table %s: column %s has unknown kind %d", t.Name, c.Name, c.Kind)
		}
		fields[i] = reflect.StructField{
			Name: fmt.Sprintf("C%d", i),
			Type: goType,
			Tag:  reflect.StructTag(fmt.Sprintf(`parquet:"%s,optional"`, c.Name)),
		}
	}
	model := reflect.New(reflect.StructOf(fields)).Interface()
	schema := parquet.NewSchema(t.Name, parquet.SchemaOf(model))

	columns := schema.Columns()
	if len(columns) != len(t.Columns) {
		return nil, fmt.Errorf("table %s: schema has %d columns, want %d", t.Name, len(columns), len(t.Columns))
	}
	for i, path := range columns {
		if len(path) != 1 || path[0] != t.Columns[i].Name {
			return nil, fmt.Errorf("table %s: schema column %d is %v, want %s", t.Name, i, path, t.Columns[i].Name)
		}
	}
	return schema, nil
}

var goTypes = map[Kind]reflect.Type{
	String:  reflect.TypeFor[string](),
	Int64:   reflect.TypeFor[int64](),
	Float64: reflect.TypeFor[float64](),
}

func writeTable(t Table, schema *parquet.Schema, in io.Reader, name string, out io.Writer, sampleColumn int, onSample func(string)) (Result, error) {
	tsv := newTSVReader(in)
	header, err := tsv.next()
	if errors.Is(err, io.EOF) {
		return Result{}, fmt.Errorf("%s: file is empty", name)
	}
	if err != nil {
		return Result{}, fmt.Errorf("%s line 1: %w", name, err)
	}
	if err := checkHeader(header, t.Columns); err != nil {
		return Result{}, fmt.Errorf("%s line 1: %w", name, err)
	}

	w := parquet.NewWriter(out, schema,
		parquet.Compression(&zstd.Codec{}),
		parquet.MaxRowsPerRowGroup(RowsPerRowGroup),
		parquet.DefaultEncodingFor(parquet.ByteArray, &parquet.Plain),
	)
	numColumns := len(t.Columns)
	values := make([]parquet.Value, rowsPerWrite*numColumns)
	rows := make([]parquet.Row, 0, rowsPerWrite)
	var result Result

	flush := func() error {
		if _, err := w.WriteRows(rows); err != nil {
			return fmt.Errorf("%s: write parquet: %w", name, err)
		}
		rows = rows[:0]
		return nil
	}

	for {
		fields, err := tsv.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Result{}, fmt.Errorf("%s line %d: %w", name, tsv.line+1, err)
		}
		if len(fields) != numColumns {
			return Result{}, fmt.Errorf("%s line %d: row has %d fields, want %d", name, tsv.line, len(fields), numColumns)
		}
		end := (len(rows) + 1) * numColumns
		row := values[end-numColumns : end : end]
		for i, c := range t.Columns {
			v, err := c.parse(fields[i])
			if err != nil {
				return Result{}, fmt.Errorf("%s line %d: column %s: %w", name, tsv.line, c.Name, err)
			}
			row[i] = v.Level(0, definitionLevel(v), i)
		}
		if onSample != nil {
			onSample(fields[sampleColumn])
		}
		rows = append(rows, row)
		result.InputRows++
		if len(rows) == rowsPerWrite {
			if err := flush(); err != nil {
				return Result{}, err
			}
		}
	}
	if err := flush(); err != nil {
		return Result{}, err
	}
	if err := w.Close(); err != nil {
		return Result{}, fmt.Errorf("%s: close parquet: %w", name, err)
	}
	return result, nil
}

func definitionLevel(v parquet.Value) int {
	if v.IsNull() {
		return 0
	}
	return 1
}

func checkHeader(header []string, columns []Column) error {
	for i, c := range columns {
		if i >= len(header) {
			return fmt.Errorf("header has %d columns, want %d: missing %s", len(header), len(columns), c.Name)
		}
		if header[i] != c.Name {
			return fmt.Errorf("header column %d is %q, want %q", i+1, header[i], c.Name)
		}
	}
	if len(header) > len(columns) {
		return fmt.Errorf("header has %d columns, want %d: unexpected %q", len(header), len(columns), header[len(columns)])
	}
	return nil
}

// verify checks that the parquet file at path holds want rows and the columns
// of t, and returns its row count.
func verify(path string, t Table, want int64) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		return 0, fmt.Errorf("%s: reopen: %w", path, err)
	}
	if got := pf.NumRows(); got != want {
		return got, fmt.Errorf("%s: footer has %d rows, want %d", path, got, want)
	}
	columns := pf.Schema().Columns()
	if len(columns) != len(t.Columns) {
		return 0, fmt.Errorf("%s: footer has %d columns, want %d", path, len(columns), len(t.Columns))
	}
	for i, column := range columns {
		if column[0] != t.Columns[i].Name {
			return 0, fmt.Errorf("%s: footer column %d is %s, want %s", path, i, column[0], t.Columns[i].Name)
		}
	}
	return pf.NumRows(), nil
}

func (t Table) columnIndex(name string) int {
	for i, c := range t.Columns {
		if c.Name == name {
			return i
		}
	}
	return -1
}

// parse converts one TSV field to a parquet value. An empty field is null.
// NA is null in Int64 and Float64 columns and in String columns with
// NAIsNull set. Every other String value is kept as it is.
func (c Column) parse(s string) (parquet.Value, error) {
	if c.Want != "" && s != c.Want {
		return parquet.Value{}, fmt.Errorf("value %q, want %q", s, c.Want)
	}
	if s == "" || (s == "NA" && (c.Kind != String || c.NAIsNull)) {
		return parquet.NullValue(), nil
	}
	switch c.Kind {
	case String:
		if !utf8.ValidString(s) {
			return parquet.Value{}, fmt.Errorf("invalid UTF-8 value %q", s)
		}
		return parquet.ValueOf(s), nil
	case Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return parquet.Value{}, fmt.Errorf("invalid %s value %q", c.Kind, s)
		}
		return parquet.Int64Value(n), nil
	case Float64:
		f, err := parseFloat(s)
		if err != nil {
			return parquet.Value{}, fmt.Errorf("invalid %s value %q", c.Kind, s)
		}
		return parquet.DoubleValue(f), nil
	}
	return parquet.Value{}, fmt.Errorf("unknown kind %d", c.Kind)
}

// parseFloat parses a decimal number. It rejects the NaN, Inf, hexadecimal
// and underscore forms that strconv.ParseFloat accepts.
func parseFloat(s string) (float64, error) {
	if strings.ContainsFunc(s, isNotDecimalRune) {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseFloat(s, 64)
}

func isNotDecimalRune(r rune) bool {
	return !strings.ContainsRune("0123456789+-.eE", r)
}
