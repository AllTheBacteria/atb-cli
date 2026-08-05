package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// Row is a single result row, mapping column name to string value.
type Row map[string]string

// Format writes rows to w in the requested format.
// Supported formats: tsv, csv, json, table.
func Format(w io.Writer, rows []Row, columns []string, format string) error {
	switch strings.ToLower(format) {
	case "tsv":
		return formatTSV(w, rows, columns)
	case "csv":
		return formatCSV(w, rows, columns)
	case "json":
		return formatJSON(w, rows, columns)
	case "table":
		return formatTable(w, rows, columns)
	default:
		return fmt.Errorf("unknown format %q: must be tsv, csv, json, or table", format)
	}
}

// InferColumns collects all keys from rows, sorts them alphabetically,
// and puts sample_accession first if present.
func InferColumns(rows []Row) []string {
	seen := make(map[string]bool)
	for _, row := range rows {
		for k := range row {
			seen[k] = true
		}
	}

	hasSampleAccession := seen["sample_accession"]
	delete(seen, "sample_accession")

	rest := make([]string, 0, len(seen))
	for k := range seen {
		rest = append(rest, k)
	}
	sort.Strings(rest)

	if hasSampleAccession {
		return append([]string{"sample_accession"}, rest...)
	}
	return rest
}

func rowValues(row Row, columns []string) []string {
	vals := make([]string, len(columns))
	for i, col := range columns {
		vals[i] = row[col]
	}
	return vals
}

func formatTSV(w io.Writer, rows []Row, columns []string) error {
	tw := csv.NewWriter(w)
	tw.Comma = '\t'

	if err := tw.Write(columns); err != nil {
		return err
	}
	for _, row := range rows {
		if err := tw.Write(rowValues(row, columns)); err != nil {
			return err
		}
	}
	tw.Flush()
	return tw.Error()
}

func formatCSV(w io.Writer, rows []Row, columns []string) error {
	cw := csv.NewWriter(w)

	if err := cw.Write(columns); err != nil {
		return err
	}
	for _, row := range rows {
		if err := cw.Write(rowValues(row, columns)); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func formatJSON(w io.Writer, rows []Row, columns []string) error {
	out := make([]map[string]string, len(rows))
	for i, row := range rows {
		obj := make(map[string]string, len(columns))
		for _, col := range columns {
			obj[col] = row[col]
		}
		out[i] = obj
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func formatTable(w io.Writer, rows []Row, columns []string) error {
	table := tablewriter.NewTable(w, tablewriter.WithBorders(tw.Border{
		Left:   tw.Off,
		Right:  tw.Off,
		Top:    tw.Off,
		Bottom: tw.Off,
	}))
	table.Header(columns)

	for _, row := range rows {
		table.Append(rowValues(row, columns))
	}

	return table.Render()
}
