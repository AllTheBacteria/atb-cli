package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ulikunitz/xz"
)

func buildTarXz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	xw, err := xz.NewWriter(&buf)
	if err != nil {
		t.Fatalf("xz writer: %v", err)
	}
	tw := tar.NewWriter(xw)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("header: %v", err)
		}
		if _, err := io.WriteString(tw, content); err != nil {
			t.Fatalf("content: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := xw.Close(); err != nil {
		t.Fatalf("xz close: %v", err)
	}
	return buf.Bytes()
}

func seedIndex(t *testing.T, dataDir, filename, url string) {
	t.Helper()
	tsv := "project\tproject_id\tfilename\turl\tmd5\tsize\n" +
		fmt.Sprintf("AllTheBacteria/Assembly\tabc123\t%s\t%s\t\t0.1\n", filename, url)
	if err := os.WriteFile(filepath.Join(dataDir, "all_atb_files.tsv"), []byte(tsv), 0o644); err != nil {
		t.Fatalf("seed index: %v", err)
	}
}

func readGzFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer gr.Close()
	b, _ := io.ReadAll(gr)
	return string(b)
}

func TestOSFDownloadExtract(t *testing.T) {
	payload := buildTarXz(t, map[string]string{"SAMD00000001.fa": ">s\nACGT\n"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	seedIndex(t, dataDir, "batch1.tar.xz", srv.URL+"/batch1.tar.xz")

	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := runCmd("osf", "download", "batch1",
		"--data-dir", dataDir, "-o", outDir,
		"--extract", "--compress", "gz", "--delete-archive", "--force")
	if err != nil {
		t.Fatalf("osf download --extract failed: %v", err)
	}

	gzPath := filepath.Join(outDir, "SAMD00000001.fa.gz")
	if _, err := os.Stat(gzPath); err != nil {
		t.Fatalf("expected extracted %s: %v", gzPath, err)
	}
	if c := readGzFile(t, gzPath); c != ">s\nACGT\n" {
		t.Errorf("extracted content = %q", c)
	}
	if _, err := os.Stat(filepath.Join(outDir, "batch1.tar.xz")); !os.IsNotExist(err) {
		t.Errorf("expected tarball deleted by --delete-archive")
	}
}

func TestOSFDownloadCompressRequiresExtract(t *testing.T) {
	dataDir := t.TempDir()
	seedIndex(t, dataDir, "batch1.tar.xz", "http://example.invalid/batch1.tar.xz")
	_, _, err := runCmd("osf", "download", "batch1",
		"--data-dir", dataDir, "--compress", "xz")
	if err == nil {
		t.Fatal("expected error: --compress without --extract")
	}
}

func TestOSFDownloadBadCompress(t *testing.T) {
	dataDir := t.TempDir()
	seedIndex(t, dataDir, "batch1.tar.xz", "http://example.invalid/batch1.tar.xz")
	_, _, err := runCmd("osf", "download", "batch1",
		"--data-dir", dataDir, "--extract", "--compress", "bzip2")
	if err == nil {
		t.Fatal("expected error: invalid --compress value")
	}
}

func TestOSFDownloadDeleteRequiresExtract(t *testing.T) {
	dataDir := t.TempDir()
	seedIndex(t, dataDir, "batch1.tar.xz", "http://example.invalid/batch1.tar.xz")
	_, _, err := runCmd("osf", "download", "batch1",
		"--data-dir", dataDir, "--delete-archive")
	if err == nil {
		t.Fatal("expected error: --delete-archive without --extract")
	}
}
