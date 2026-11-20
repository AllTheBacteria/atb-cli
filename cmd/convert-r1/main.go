// Command convert-r1 converts the AllTheBacteria release 1 metadata TSVs
// (atb.metadata.r1.*.tsv.gz) into the parquet files the atb CLI reads, and
// writes an MD5SUMS file for them.
//
// Usage:
//
//	go run ./cmd/convert-r1 --in-dir DIR --out-dir DIR [--tables assembly,checkm2]
package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/allthebacteria/atb-cli/internal/convert"
)

const md5SumsFile = "MD5SUMS"

func main() {
	inDir := flag.String("in-dir", ".", "directory holding the atb.metadata.r1.*.tsv.gz files")
	outDir := flag.String("out-dir", "", "directory to write the parquet files and MD5SUMS to (required)")
	tableList := flag.String("tables", strings.Join(tableNames(), ","), "comma separated tables to convert")
	flag.Parse()

	if *outDir == "" {
		fail(errors.New("--out-dir is required"))
	}
	tables, err := selectTables(*tableList)
	if err != nil {
		fail(err)
	}
	if err := run(os.Stdout, *inDir, *outDir, tables); err != nil {
		fail(err)
	}
}

func tableNames() []string {
	names := make([]string, len(convert.Tables))
	for i, t := range convert.Tables {
		names[i] = t.Name
	}
	return names
}

// selectTables returns the named tables in conversion order.
func selectTables(list string) ([]convert.Table, error) {
	wanted := map[string]bool{}
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := convert.Lookup(name); !ok {
			return nil, fmt.Errorf("unknown table %q (known: %s)", name, strings.Join(tableNames(), ", "))
		}
		wanted[name] = true
	}
	var tables []convert.Table
	for _, t := range convert.Tables {
		if wanted[t.Name] {
			tables = append(tables, t)
		}
	}
	if len(tables) == 0 {
		return nil, errors.New("--tables names no table")
	}
	return tables, nil
}

type tableCoverage struct {
	name     string
	coverage *convert.Coverage
}

// run converts tables from inDir into outDir, prints row counts and timings,
// prints the coverage report when assembly is converted, and writes MD5SUMS
// for every table output present in outDir.
func run(w io.Writer, inDir, outDir string, tables []convert.Table) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	var samples *convert.SampleSet
	var coverages []tableCoverage
	for _, t := range tables {
		var onSample func(string)
		switch {
		case t.Name == "assembly":
			samples = convert.NewSampleSet()
			onSample = samples.Add
		case samples != nil:
			c := samples.NewCoverage()
			coverages = append(coverages, tableCoverage{t.Name, c})
			onSample = c.Add
		}
		start := time.Now()
		result, err := convert.Convert(t, filepath.Join(inDir, t.Input), filepath.Join(outDir, t.Output), onSample)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%-15s input rows %9d  output rows %9d  %s\n",
			t.Name, result.InputRows, result.OutputRows, time.Since(start).Round(time.Millisecond))
	}
	if samples != nil {
		printCoverage(w, samples, coverages)
	}
	return writeMD5Sums(outDir)
}

func printCoverage(w io.Writer, samples *convert.SampleSet, coverages []tableCoverage) {
	fmt.Fprintf(w, "\nassembly: %d rows, %d distinct samples\n", samples.Rows(), samples.Len())
	if len(coverages) == 0 {
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "table\trows\tsamples\trows not in assembly\tassembly samples missing\t")
	for _, c := range coverages {
		r := c.coverage.Report()
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t\n", c.name, r.Rows, r.Samples, r.RowsNotInAssembly, r.AssemblyMissing)
	}
	tw.Flush()
}

// writeMD5Sums writes "<md5>  <file>" lines for each table output found in
// dir, in conversion order.
func writeMD5Sums(dir string) error {
	var b strings.Builder
	for _, t := range convert.Tables {
		sum, err := md5File(filepath.Join(dir, t.Output))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s  %s\n", sum, t.Output)
	}
	tmp := filepath.Join(dir, md5SumsFile+".tmp")
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, md5SumsFile))
}

func md5File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "convert-r1:", err)
	os.Exit(1)
}
