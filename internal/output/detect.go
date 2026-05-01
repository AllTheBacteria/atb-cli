package output

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func ResolveFormat(format string) string {
	if format == "" || format == "auto" {
		if IsTerminal() {
			return "table"
		}
		return "tsv"
	}
	return format
}

// InferFormatFromPath returns "tsv", "csv", "json", or "" based on the file
// extension (after stripping a trailing .gz). Returns "" when the extension is
// missing or unrecognised so callers can fall through to their own defaults.
//
// Exposed as a separate step (rather than baked into ResolveFormat) so each
// command can interleave it with config-driven defaults in the correct
// priority: --format flag > filename hint > configured default > TTY default.
func InferFormatFromPath(path string) string {
	if path == "" {
		return ""
	}
	lower := strings.ToLower(path)
	lower = strings.TrimSuffix(lower, ".gz")
	switch filepath.Ext(lower) {
	case ".tsv":
		return "tsv"
	case ".csv":
		return "csv"
	case ".json":
		return "json"
	}
	return ""
}

// OpenWriter creates the file at path and returns a writer plus a close
// function. When path ends with .gz (case-insensitive) the returned writer is
// a gzip.Writer wrapping the file; the close function flushes the gzip stream
// and closes the underlying file. Callers must invoke close to avoid silently
// truncating the gzip trailer.
func OpenWriter(path string) (io.Writer, func() error, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("opening output file: %w", err)
	}
	if !strings.HasSuffix(strings.ToLower(path), ".gz") {
		return f, f.Close, nil
	}
	gz := gzip.NewWriter(f)
	closer := func() error {
		gzErr := gz.Close()
		fErr := f.Close()
		if gzErr != nil {
			return fmt.Errorf("closing gzip stream: %w", gzErr)
		}
		return fErr
	}
	return gz, closer, nil
}
