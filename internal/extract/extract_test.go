package extract

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ulikunitz/xz"
)

// writeTar builds an archive at path containing name->content regular files.
// kind selects the outer compression: "xz", "gz", or "" (plain .tar).
func writeTar(t *testing.T, path, kind string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()

	var w io.Writer = f
	var closer io.Closer
	switch kind {
	case "xz":
		xw, err := xz.NewWriter(f)
		if err != nil {
			t.Fatalf("xz writer: %v", err)
		}
		w, closer = xw, xw
	case "gz":
		gw := gzip.NewWriter(f)
		w, closer = gw, gw
	}

	tw := tar.NewWriter(w)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %s: %v", name, err)
		}
		if _, err := io.WriteString(tw, content); err != nil {
			t.Fatalf("write content %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if closer != nil {
		if err := closer.Close(); err != nil {
			t.Fatalf("outer close: %v", err)
		}
	}
}

func readGz(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open gz: %v", err)
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gr.Close()
	b, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("read gz: %v", err)
	}
	return string(b)
}

func readXz(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open xz: %v", err)
	}
	defer f.Close()
	xr, err := xz.NewReader(f)
	if err != nil {
		t.Fatalf("xz reader: %v", err)
	}
	b, err := io.ReadAll(xr)
	if err != nil {
		t.Fatalf("read xz: %v", err)
	}
	return string(b)
}

func TestArchiveNoneFromXz(t *testing.T) {
	dir := t.TempDir()
	arc := filepath.Join(dir, "in.tar.xz")
	writeTar(t, arc, "xz", map[string]string{"SAMD1.fa": ">s\nACGT\n"})

	out := filepath.Join(dir, "out")
	res, err := Archive(arc, out, "none")
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if res.Files != 1 {
		t.Errorf("Files = %d, want 1", res.Files)
	}
	if res.Bytes != int64(len(">s\nACGT\n")) {
		t.Errorf("Bytes = %d, want %d", res.Bytes, len(">s\nACGT\n"))
	}
	got, err := os.ReadFile(filepath.Join(out, "SAMD1.fa"))
	if err != nil {
		t.Fatalf("read extracted: %v", err)
	}
	if string(got) != ">s\nACGT\n" {
		t.Errorf("content = %q", string(got))
	}
}

func TestArchiveGzFromXz(t *testing.T) {
	dir := t.TempDir()
	arc := filepath.Join(dir, "in.tar.xz")
	writeTar(t, arc, "xz", map[string]string{"SAMD1.fa": ">s\nACGT\n"})

	out := filepath.Join(dir, "out")
	res, err := Archive(arc, out, "gz")
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if res.Bytes <= 0 {
		t.Errorf("Bytes = %d, want > 0", res.Bytes)
	}
	gzPath := filepath.Join(out, "SAMD1.fa.gz")
	if _, err := os.Stat(gzPath); err != nil {
		t.Fatalf("expected %s: %v", gzPath, err)
	}
	if c := readGz(t, gzPath); c != ">s\nACGT\n" {
		t.Errorf("gz content = %q", c)
	}
}

func TestArchiveXzFromGz(t *testing.T) {
	dir := t.TempDir()
	arc := filepath.Join(dir, "in.tar.gz")
	writeTar(t, arc, "gz", map[string]string{"SAMD1.fa": ">s\nACGT\n"})

	out := filepath.Join(dir, "out")
	if _, err := Archive(arc, out, "xz"); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	xzPath := filepath.Join(out, "SAMD1.fa.xz")
	if _, err := os.Stat(xzPath); err != nil {
		t.Fatalf("expected %s: %v", xzPath, err)
	}
	if c := readXz(t, xzPath); c != ">s\nACGT\n" {
		t.Errorf("xz content = %q", c)
	}
}

func TestArchivePlainTar(t *testing.T) {
	dir := t.TempDir()
	arc := filepath.Join(dir, "in.tar")
	writeTar(t, arc, "", map[string]string{"a.fa": "X", "b.fa": "YY"})

	out := filepath.Join(dir, "out")
	res, err := Archive(arc, out, "none")
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if res.Files != 2 {
		t.Errorf("Files = %d, want 2", res.Files)
	}
	if res.Bytes != int64(len("X")+len("YY")) {
		t.Errorf("Bytes = %d, want %d", res.Bytes, len("X")+len("YY"))
	}
}

func TestArchiveRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	arc := filepath.Join(dir, "evil.tar.xz")
	writeTar(t, arc, "xz", map[string]string{"../escape.fa": "PWNED"})

	out := filepath.Join(dir, "out")
	if _, err := Archive(arc, out, "none"); err == nil {
		t.Fatal("expected error for traversal member, got nil")
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.fa")); !os.IsNotExist(err) {
		t.Errorf("traversal wrote outside destDir: %v", err)
	}
}

func TestIsArchive(t *testing.T) {
	cases := map[string]bool{
		"x.tar.xz": true, "x.tar.gz": true, "x.tgz": true, "x.tar": true,
		"x.fa": false, "x.fa.gz": false, "x.zip": false,
	}
	for name, want := range cases {
		if got := IsArchive(name); got != want {
			t.Errorf("IsArchive(%q) = %v, want %v", name, got, want)
		}
	}
}
