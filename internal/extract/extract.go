// Package extract unpacks tar archives (optionally xz- or gzip-compressed)
// and writes each regular file into a destination directory, optionally
// recompressing members with gzip or xz. Member paths are validated against
// directory traversal.
package extract

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz"
)

// Result summarises a single archive extraction.
type Result struct {
	Files int   // regular files written
	Bytes int64 // total bytes written to disk (post-recompression)
}

// IsArchive reports whether name looks like a tar archive this package can
// extract: .tar, .tar.gz, .tgz, or .tar.xz.
func IsArchive(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".tar") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar.xz")
}

// Archive extracts every regular file from archivePath into destDir,
// recompressing each member according to comp ("none", "gz", or "xz").
// The outer compression is detected from archivePath's extension.
func Archive(archivePath, destDir, comp string) (Result, error) {
	var res Result

	switch comp {
	case "none", "gz", "xz":
	default:
		return res, fmt.Errorf("invalid compression %q: want none, gz, or xz", comp)
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return res, err
	}
	defer f.Close()

	dr, closer, err := decompressReader(archivePath, f)
	if err != nil {
		return res, fmt.Errorf("open %s: %w", filepath.Base(archivePath), err)
	}
	if closer != nil {
		defer closer.Close()
	}

	tr := tar.NewReader(dr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return res, fmt.Errorf("read tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		dest, err := safeDestPath(destDir, hdr.Name)
		if err != nil {
			return res, err
		}

		n, err := writeMember(tr, dest, comp)
		if err != nil {
			return res, fmt.Errorf("extract %s: %w", hdr.Name, err)
		}
		res.Files++
		res.Bytes += n
	}

	return res, nil
}

// decompressReader wraps r with an xz or gzip decoder based on path's
// extension. The returned closer is non-nil only when a wrapper needs
// closing (gzip); it is nil for xz (no Close) and plain tar (the caller
// owns the underlying file).
func decompressReader(path string, r io.Reader) (io.Reader, io.Closer, error) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".tar.xz"):
		xr, err := xz.NewReader(r)
		return xr, nil, err
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		gr, err := gzip.NewReader(r)
		return gr, gr, err
	default:
		return r, nil, nil
	}
}

// safeDestPath validates a tar member name and returns its absolute
// destination path rooted at destDir, rejecting absolute paths and any
// traversal that would escape destDir.
func safeDestPath(destDir, name string) (string, error) {
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("unsafe archive member (absolute path): %s", name)
	}
	dest := filepath.Join(destDir, filepath.Clean(name))
	rel, err := filepath.Rel(destDir, dest)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("unsafe archive member (path traversal): %s", name)
	}
	return dest, nil
}

// writeMember streams src into a file at dest, recompressing per comp, and
// returns the number of bytes written to disk. If any write step fails, the
// partial output file is removed.
func writeMember(src io.Reader, dest, comp string) (int64, error) {
	outPath := dest
	switch comp {
	case "gz":
		if !strings.HasSuffix(strings.ToLower(outPath), ".gz") {
			outPath += ".gz"
		}
	case "xz":
		if !strings.HasSuffix(strings.ToLower(outPath), ".xz") {
			outPath += ".xz"
		}
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return 0, err
	}

	out, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	// Remove the partial file if anything below fails.
	success := false
	defer func() {
		out.Close()
		if !success {
			os.Remove(outPath)
		}
	}()

	var w io.Writer = out
	var closer io.Closer
	switch comp {
	case "gz":
		gw := gzip.NewWriter(out)
		w, closer = gw, gw
	case "xz":
		xw, xerr := xz.NewWriter(out)
		if xerr != nil {
			return 0, xerr
		}
		w, closer = xw, xw
	}

	if _, err := io.Copy(w, src); err != nil {
		return 0, err
	}
	if closer != nil {
		if err := closer.Close(); err != nil {
			return 0, err
		}
	}

	info, err := out.Stat()
	if err != nil {
		return 0, err
	}
	success = true
	return info.Size(), nil
}
