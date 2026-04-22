package amr

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

const amrSchema = `
CREATE TABLE amr (
    name TEXT,
    gene_symbol TEXT,
    hierarchy_node TEXT,
    element_type TEXT,
    element_subtype TEXT,
    coverage REAL,
    identity REAL,
    method TEXT,
    class TEXT,
    subclass TEXT,
    species TEXT,
    genus TEXT
);
CREATE INDEX idx_amr_name ON amr(name);
CREATE INDEX idx_amr_gene ON amr(gene_symbol);
CREATE INDEX idx_amr_class ON amr(class);
CREATE INDEX idx_amr_type ON amr(element_type);
`

const amrInsert = `INSERT INTO amr VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`

const amrBatchSize = 10_000

// BuildIndexes reads each .parquet file in the partition directory and builds
// a corresponding .sqlite index. Builds run in parallel (one goroutine per file,
// bounded by NumCPU).
func BuildIndexes(dataDir string, logFn func(string, ...any)) error {
	if logFn == nil {
		logFn = func(string, ...any) {}
	}

	partDir := filepath.Join(dataDir, PartitionDir)
	entries, err := os.ReadDir(partDir)
	if err != nil {
		return fmt.Errorf("reading partition dir: %w", err)
	}

	var parquetFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".parquet") {
			parquetFiles = append(parquetFiles, e.Name())
		}
	}

	if len(parquetFiles) == 0 {
		return nil
	}

	logFn("Building AMR indexes (%d files, %d workers)...", len(parquetFiles), workers())
	start := time.Now()

	sem := make(chan struct{}, workers())
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errList []error
	var built int

	for _, name := range parquetFiles {
		wg.Add(1)
		go func(pqName string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			pqPath := filepath.Join(partDir, pqName)
			sqliteName := strings.TrimSuffix(pqName, ".parquet") + ".sqlite"
			sqlitePath := filepath.Join(partDir, sqliteName)

			count, err := buildOneIndex(pqPath, sqlitePath)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errList = append(errList, fmt.Errorf("%s: %w", pqName, err))
				logFn("  %s ... FAILED: %v", sqliteName, err)
			} else {
				built++
				logFn("  %s (%s rows)", sqliteName, formatCount(count))
			}
		}(name)
	}

	wg.Wait()

	elapsed := time.Since(start).Truncate(time.Millisecond)
	logFn("Indexed %d files (%s)", built, elapsed)

	if len(errList) > 0 {
		return fmt.Errorf("%d index builds failed; first: %w", len(errList), errList[0])
	}
	return nil
}

func buildOneIndex(parquetPath, sqlitePath string) (int64, error) {
	tmpPath := sqlitePath + ".tmp"
	_ = os.Remove(tmpPath)

	db, err := sql.Open("sqlite", tmpPath+"?_journal_mode=WAL&_synchronous=OFF")
	if err != nil {
		return 0, err
	}
	defer db.Close()

	if _, err := db.Exec(amrSchema); err != nil {
		return 0, fmt.Errorf("creating schema: %w", err)
	}

	rows, err := pq.ReadAll[pq.AMRRow](parquetPath)
	if err != nil {
		return 0, fmt.Errorf("reading parquet: %w", err)
	}

	count := int64(len(rows))

	for start := 0; start < len(rows); start += amrBatchSize {
		end := start + amrBatchSize
		if end > len(rows) {
			end = len(rows)
		}

		tx, err := db.Begin()
		if err != nil {
			return 0, err
		}

		stmt, err := tx.Prepare(amrInsert)
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}

		for i := start; i < end; i++ {
			r := rows[i]
			if _, err := stmt.Exec(
				r.Name, r.GeneSymbol, r.HierarchyNode,
				r.ElementType, r.ElementSubtype,
				r.Coverage, r.Identity, r.Method,
				r.Class, r.Subclass, r.Species, r.Genus,
			); err != nil {
				_ = stmt.Close()
				_ = tx.Rollback()
				return 0, err
			}
		}

		_ = stmt.Close()
		if err := tx.Commit(); err != nil {
			return 0, err
		}
	}

	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return 0, err
	}
	if err := db.Close(); err != nil {
		return 0, err
	}

	if err := os.Rename(tmpPath, sqlitePath); err != nil {
		return 0, err
	}

	return count, nil
}

// IndexPath returns the path to a genus SQLite index if it exists.
func IndexPath(dataDir, genus string) string {
	normalized := normalizeGenus(genus)
	path := filepath.Join(dataDir, PartitionDir, normalized+".sqlite")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

// QueryIndex runs a SQL query against a genus SQLite index and returns results.
func QueryIndex(dbPath string, filters Filters) ([]Result, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// For large sample sets, load them into a temp table to avoid SQLite variable limits.
	if len(filters.Samples) > 0 {
		if _, err := db.Exec("CREATE TEMP TABLE _samples (name TEXT PRIMARY KEY)"); err != nil {
			return nil, fmt.Errorf("creating temp table: %w", err)
		}
		tx, err := db.Begin()
		if err != nil {
			return nil, err
		}
		stmt, err := tx.Prepare("INSERT OR IGNORE INTO _samples (name) VALUES (?)")
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		for s := range filters.Samples {
			if _, err := stmt.Exec(s); err != nil {
				_ = stmt.Close()
				_ = tx.Rollback()
				return nil, err
			}
		}
		_ = stmt.Close()
		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}

	query, args := buildSQL(filters)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(
			&r.SampleAccession, &r.GeneSymbol,
			&r.ElementType, &r.ElementSubtype,
			&r.Coverage, &r.Identity, &r.Method,
			&r.Class, &r.Subclass, &r.Species, &r.Genus,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func buildSQL(f Filters) (string, []any) {
	var clauses []string
	var args []any

	if len(f.Samples) > 0 {
		clauses = append(clauses, "name IN (SELECT name FROM _samples)")
	}
	if f.Class != "" {
		clauses = append(clauses, "UPPER(class) LIKE ?")
		args = append(args, "%"+strings.ToUpper(f.Class)+"%")
	}
	if f.GenePattern != "" {
		sqlPattern := convertToSQLLike(f.GenePattern)
		clauses = append(clauses, "LOWER(gene_symbol) LIKE ? ESCAPE '\\'")
		args = append(args, sqlPattern)
	}
	if f.MinCoverage > 0 {
		clauses = append(clauses, "coverage >= ?")
		args = append(args, f.MinCoverage)
	}
	if f.MinIdentity > 0 {
		clauses = append(clauses, "identity >= ?")
		args = append(args, f.MinIdentity)
	}
	if f.ElementType != "" && !strings.EqualFold(f.ElementType, "all") {
		clauses = append(clauses, "UPPER(element_type) = UPPER(?)")
		args = append(args, f.ElementType)
	}

	q := `SELECT name, gene_symbol, element_type, element_subtype,
	       coverage, identity, method, class, subclass, species, genus
	       FROM amr`
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	return q, args
}

// convertToSQLLike converts our % wildcard pattern to a SQL LIKE pattern.
// Escapes SQL special chars (_ and %) in the literal parts, preserving our % as SQL %.
func convertToSQLLike(pattern string) string {
	pattern = strings.ToLower(pattern)
	var b strings.Builder
	for _, ch := range pattern {
		switch ch {
		case '%':
			b.WriteRune('%') // our wildcard → SQL wildcard
		case '_':
			b.WriteString("\\_") // escape SQL single-char wildcard
		default:
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func workers() int {
	n := runtime.NumCPU()
	if n > 8 {
		n = 8
	}
	if n < 2 {
		n = 2
	}
	return n
}
