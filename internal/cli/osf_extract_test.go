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
	"strings"
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

func readXzFile(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	xr, err := xz.NewReader(f)
	if err != nil {
		t.Fatalf("xz reader: %v", err)
	}
	b, _ := io.ReadAll(xr)
	return string(b)
}

func TestOSFDownloadExtractNone(t *testing.T) {
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
		"--extract", "--compress", "none", "--force")
	if err != nil {
		t.Fatalf("osf download --compress none failed: %v", err)
	}

	extracted := filepath.Join(outDir, "SAMD00000001.fa")
	got, err := os.ReadFile(extracted)
	if err != nil {
		t.Fatalf("expected extracted %s: %v", extracted, err)
	}
	if string(got) != ">s\nACGT\n" {
		t.Errorf("extracted content = %q, want %q", string(got), ">s\nACGT\n")
	}
}

func TestOSFDownloadExtractXz(t *testing.T) {
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
		"--extract", "--compress", "xz", "--force")
	if err != nil {
		t.Fatalf("osf download --compress xz failed: %v", err)
	}

	xzPath := filepath.Join(outDir, "SAMD00000001.fa.xz")
	if _, err := os.Stat(xzPath); err != nil {
		t.Fatalf("expected extracted %s: %v", xzPath, err)
	}
	if c := readXzFile(t, xzPath); c != ">s\nACGT\n" {
		t.Errorf("extracted content = %q, want %q", c, ">s\nACGT\n")
	}
}

func TestOSFDownloadExtractSkipsNonArchive(t *testing.T) {
	archivePayload := buildTarXz(t, map[string]string{"SAMD00000001.fa": ">s\nACGT\n"})
	plainPayload := []byte(">plain\nTTTT\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/batch1.tar.xz":
			_, _ = w.Write(archivePayload)
		case "/plain.fa.gz":
			_, _ = w.Write(plainPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	tsv := "project\tproject_id\tfilename\turl\tmd5\tsize\n" +
		fmt.Sprintf("AllTheBacteria/Assembly\tabc123\tbatch1.tar.xz\t%s\t\t0.1\n", srv.URL+"/batch1.tar.xz") +
		fmt.Sprintf("AllTheBacteria/Assembly\tabc123\tplain.fa.gz\t%s\t\t0.1\n", srv.URL+"/plain.fa.gz")
	if err := os.WriteFile(filepath.Join(dataDir, "all_atb_files.tsv"), []byte(tsv), 0o644); err != nil {
		t.Fatalf("seed index: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := runCmd("osf", "download", "batch1|plain",
		"--data-dir", dataDir, "-o", outDir,
		"--extract", "--compress", "gz", "--force")
	if err != nil {
		t.Fatalf("osf download with mixed entries failed: %v", err)
	}

	// Non-archive file must be present and untouched (downloaded as-is).
	plainPath := filepath.Join(outDir, "plain.fa.gz")
	if _, err := os.Stat(plainPath); err != nil {
		t.Fatalf("expected non-archive file %s to exist: %v", plainPath, err)
	}

	// Archive member must have been extracted.
	extractedPath := filepath.Join(outDir, "SAMD00000001.fa.gz")
	if _, err := os.Stat(extractedPath); err != nil {
		t.Fatalf("expected extracted archive member %s: %v", extractedPath, err)
	}
	if c := readGzFile(t, extractedPath); c != ">s\nACGT\n" {
		t.Errorf("extracted content = %q, want %q", c, ">s\nACGT\n")
	}
}

func TestOSFDownloadExtractFailure(t *testing.T) {
	// Serve corrupt bytes that are not a valid tar.xz.
	corrupt := []byte("this is not a valid tar.xz archive")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(corrupt)
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
		"--extract", "--compress", "gz", "--force")
	if err == nil {
		t.Fatal("expected error for corrupt archive, got nil")
	}
	if msg := err.Error(); !strings.Contains(msg, "extraction(s) failed") {
		t.Errorf("error %q does not contain %q", msg, "extraction(s) failed")
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
