package cli

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/allthebacteria/atb-cli/internal/query"
)

// readResultsFromFile reads a CSV or TSV file into []query.ResultRow.
// Auto-detects delimiter by checking for tabs in the first line.
// Pass "-" as path to read from stdin.
func readResultsFromFile(path string) ([]query.ResultRow, error) {
	var rc io.ReadCloser
	if path == "-" {
		rc = io.NopCloser(os.Stdin)
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening file: %w", err)
		}
		rc = f
	}
	defer rc.Close()

	// Buffer the reader so we can peek at the first line to detect delimiter.
	br := bufio.NewReader(rc)
	firstLine, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	delim := ','
	if strings.ContainsRune(firstLine, '\t') {
		delim = '\t'
	}

	// Reconstruct a reader that includes the already-consumed first line.
	combined := io.MultiReader(strings.NewReader(firstLine), br)

	r := csv.NewReader(combined)
	r.Comma = delim
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}

	var rows []query.ResultRow
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading CSV row: %w", err)
		}
		row := make(query.ResultRow, len(header))
		for i, col := range header {
			if i < len(record) {
				row[col] = record[i]
			}
		}
		rows = append(rows, row)
	}

	return rows, nil
}
