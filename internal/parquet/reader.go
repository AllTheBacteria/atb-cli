package parquet

import (
	"fmt"
	"io"
	"os"

	parquetgo "github.com/parquet-go/parquet-go"
)

// ColumnNames returns the leaf column names declared in the parquet footer.
// Reads only the file metadata, so it's cheap regardless of file size.
func ColumnNames(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening parquet file %q: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat parquet file %q: %w", path, err)
	}

	pf, err := parquetgo.OpenFile(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("reading parquet metadata for %q: %w", path, err)
	}

	paths := pf.Schema().Columns()
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if len(p) == 0 {
			continue
		}
		out = append(out, p[len(p)-1])
	}
	return out, nil
}

// ReadAll reads all rows from a parquet file into a slice of T.
// Column projection is performed based on the struct's parquet tags.
func ReadAll[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening parquet file %q: %w", path, err)
	}
	defer f.Close()

	r := parquetgo.NewGenericReader[T](f)
	defer r.Close()

	rows := make([]T, 0, r.NumRows())
	buf := make([]T, 512)

	for {
		n, err := r.Read(buf)
		if n > 0 {
			rows = append(rows, buf[:n]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading parquet file %q: %w", path, err)
		}
	}

	return rows, nil
}

// ReadFiltered reads rows from a parquet file, keeping only those where fn returns true.
// Column projection is performed based on the struct's parquet tags.
func ReadFiltered[T any](path string, fn func(T) bool) ([]T, error) {
	all, err := ReadAll[T](path)
	if err != nil {
		return nil, err
	}

	result := make([]T, 0)
	for _, row := range all {
		if fn(row) {
			result = append(result, row)
		}
	}

	return result, nil
}

// ReadStreamFiltered reads rows from a parquet file, applying the predicate
// during deserialization and returning only matching rows. If limit > 0,
// reading stops as soon as limit matching rows have been collected.
func ReadStreamFiltered[T any](path string, fn func(T) bool, limit int) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening parquet file %q: %w", path, err)
	}
	defer f.Close()

	r := parquetgo.NewGenericReader[T](f)
	defer r.Close()

	var results []T
	buf := make([]T, 512)

	for {
		n, err := r.Read(buf)
		for i := 0; i < n; i++ {
			if fn(buf[i]) {
				results = append(results, buf[i])
				if limit > 0 && len(results) >= limit {
					return results, nil
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading parquet file %q: %w", path, err)
		}
	}

	return results, nil
}
