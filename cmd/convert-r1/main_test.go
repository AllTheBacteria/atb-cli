package main

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/allthebacteria/atb-cli/internal/convert"
)

// writeTable writes a gzipped TSV for table with one row per sample. Every
// column holds a valid value for its kind.
func writeTable(t *testing.T, dir string, table convert.Table, samples []string) {
	t.Helper()
	var b strings.Builder
	for i, c := range table.Columns {
		if i > 0 {
			b.WriteByte('\t')
		}
		b.WriteString(c.Name)
	}
	b.WriteByte('\n')
	for _, sample := range samples {
		for i, c := range table.Columns {
			if i > 0 {
				b.WriteByte('\t')
			}
			switch {
			case c.Name == "sample_accession":
				b.WriteString(sample)
			case c.Want != "":
				b.WriteString(c.Want)
			case c.Kind == convert.Int64:
				b.WriteString("1")
			case c.Kind == convert.Float64:
				b.WriteString("1.5")
			default:
				b.WriteString("x")
			}
		}
		b.WriteByte('\n')
	}
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	w.Write([]byte(b.String()))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, table.Input), gz.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun(t *testing.T) {
	inDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "out")
	samples := map[string][]string{
		"assembly":       {"A", "B", "C"},
		"assembly_stats": {"A", "B", "C", "Z"},
		"checkm2":        {"A", "B"},
		"sylph":          {"A", "A", "B", "C"},
		"run":            {"A", "Y", "Y"},
	}
	for _, table := range convert.Tables {
		writeTable(t, inDir, table, samples[table.Name])
	}

	var out bytes.Buffer
	if err := run(&out, inDir, outDir, convert.Tables); err != nil {
		t.Fatal(err)
	}

	report := out.String()
	for _, pattern := range []string{
		`assembly +input rows +3 +output rows +3 `,
		`sylph +input rows +4 +output rows +4 `,
		`assembly: 3 rows, 3 distinct samples`,
		`assembly_stats +4 +4 +1 +0`,
		`checkm2 +2 +2 +0 +1`,
		`sylph +4 +3 +0 +0`,
		`run +3 +2 +2 +2`,
	} {
		if !regexp.MustCompile(pattern).MatchString(report) {
			t.Errorf("output does not match %q:\n%s", pattern, report)
		}
	}

	sums, err := os.ReadFile(filepath.Join(outDir, md5SumsFile))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(sums), "\n"), "\n")
	if len(lines) != len(convert.Tables) {
		t.Fatalf("MD5SUMS has %d lines, want %d:\n%s", len(lines), len(convert.Tables), sums)
	}
	for i, table := range convert.Tables {
		data, err := os.ReadFile(filepath.Join(outDir, table.Output))
		if err != nil {
			t.Fatal(err)
		}
		sum := md5.Sum(data)
		if want := hex.EncodeToString(sum[:]) + "  " + table.Output; lines[i] != want {
			t.Errorf("MD5SUMS line %d = %q, want %q", i+1, lines[i], want)
		}
	}
}

func TestRunStopsOnBadTable(t *testing.T) {
	inDir := t.TempDir()
	outDir := t.TempDir()
	table, _ := convert.Lookup("checkm2")
	writeTable(t, inDir, table, []string{"A"})
	path := filepath.Join(inDir, table.Input)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data[:len(data)-4], 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run(&bytes.Buffer{}, inDir, outDir, []convert.Table{table}); err == nil {
		t.Fatal("run succeeded on a truncated gzip file")
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("out dir has %d entries after a failed run", len(entries))
	}
}

func TestSelectTables(t *testing.T) {
	tests := []struct {
		list    string
		want    string
		wantErr bool
	}{
		{"run,assembly", "assembly,run", false},
		{" checkm2 , checkm2 ", "checkm2", false},
		{"assembly,mlst", "", true},
		{",", "", true},
	}
	for _, tc := range tests {
		tables, err := selectTables(tc.list)
		if (err != nil) != tc.wantErr {
			t.Errorf("selectTables(%q) error = %v, wantErr %v", tc.list, err, tc.wantErr)
			continue
		}
		var names []string
		for _, table := range tables {
			names = append(names, table.Name)
		}
		if got := strings.Join(names, ","); got != tc.want {
			t.Errorf("selectTables(%q) = %s, want %s", tc.list, got, tc.want)
		}
	}
}
