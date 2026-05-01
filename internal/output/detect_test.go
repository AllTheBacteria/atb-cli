package output

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInferFormatFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"empty path", "", ""},
		{"csv", "out.csv", "csv"},
		{"tsv", "out.tsv", "tsv"},
		{"json", "out.json", "json"},
		{"gzipped csv", "out.csv.gz", "csv"},
		{"gzipped tsv", "results.TSV.GZ", "tsv"},
		{"gzipped json", "out.json.gz", "json"},
		{"unknown extension", "out.dat", ""},
		{"no extension", "out", ""},
		{"bare gz", "out.gz", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := InferFormatFromPath(tc.path)
			if got != tc.expected {
				t.Errorf("InferFormatFromPath(%q) = %q, want %q",
					tc.path, got, tc.expected)
			}
		})
	}
}

func TestOpenWriterPlain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.tsv")

	w, closeFn, err := OpenWriter(path)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	if _, err := io.WriteString(w, "hello\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatalf("close: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "hello\n" {
		t.Errorf("plain content mismatch: got %q", got)
	}
}

func TestOpenWriterGzip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compressed.tsv.gz")
	payload := strings.Repeat("Name\tElement symbol\nSAMD1\tblaCTX-M-15\n", 1000)

	w, closeFn, err := OpenWriter(path)
	if err != nil {
		t.Fatalf("OpenWriter: %v", err)
	}
	if _, err := io.WriteString(w, payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// File must be a valid gzip stream that decompresses to the original payload.
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gr.Close()
	got, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if string(got) != payload {
		t.Error("decompressed content does not match input")
	}

	// Compression should actually shrink this repetitive payload — sanity check
	// that we wrote a real gzip stream rather than the raw bytes.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if int(info.Size()) >= len(payload) {
		t.Errorf("expected compressed file (%d B) smaller than payload (%d B)",
			info.Size(), len(payload))
	}
}
