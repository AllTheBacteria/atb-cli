package amr

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	parquetgo "github.com/parquet-go/parquet-go"

	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

// amrSchema mirrors the 26 columns of AMRFinderPlus v4.2.5 in the same order
// as pq.AMRRow. The amrInsert placeholders, the buildOneIndex Exec args, the
// buildSQL SELECT, and the QueryIndex Scan must stay aligned with this order.
const amrSchema = `
CREATE TABLE amr (
    name TEXT,
    protein_id TEXT,
    contig_id TEXT,
    start INTEGER,
    stop INTEGER,
    strand TEXT,
    gene_symbol TEXT,
    element_name TEXT,
    scope TEXT,
    element_type TEXT,
    element_subtype TEXT,
    class TEXT,
    subclass TEXT,
    method TEXT,
    target_length INTEGER,
    reference_sequence_length INTEGER,
    coverage REAL,
    identity REAL,
    alignment_length INTEGER,
    closest_reference_accession TEXT,
    closest_reference_name TEXT,
    hmm_accession TEXT,
    hmm_description TEXT,
    hierarchy_node TEXT,
    genus TEXT,
    species TEXT
);
CREATE INDEX idx_amr_name ON amr(name);
CREATE INDEX idx_amr_gene ON amr(gene_symbol);
CREATE INDEX idx_amr_class ON amr(class);
CREATE INDEX idx_amr_type ON amr(element_type);
`

const amrInsert = `INSERT INTO amr VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

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

	db, err := sql.Open("sqlite", tmpPath+"?_journal_mode=OFF&_synchronous=OFF&_cache_size=-65536&_temp_store=MEMORY")
	if err != nil {
		return 0, err
	}
	defer db.Close()

	if _, err := db.Exec(amrSchema); err != nil {
		return 0, fmt.Errorf("creating schema: %w", err)
	}

	f, err := os.Open(parquetPath)
	if err != nil {
		return 0, fmt.Errorf("opening parquet: %w", err)
	}
	defer f.Close()

	r := parquetgo.NewGenericReader[pq.AMRRow](f)
	defer r.Close()

	buf := make([]pq.AMRRow, 512)

	var (
		tx    *sql.Tx
		stmt  *sql.Stmt
		count int64
		batch int64
	)

	rollback := func() {
		if stmt != nil {
			_ = stmt.Close()
			stmt = nil
		}
		if tx != nil {
			_ = tx.Rollback()
			tx = nil
		}
	}

	openBatch := func() error {
		var err error
		tx, err = db.Begin()
		if err != nil {
			return err
		}
		stmt, err = tx.Prepare(amrInsert)
		if err != nil {
			_ = tx.Rollback()
			tx = nil
			return err
		}
		return nil
	}

	commitBatch := func() error {
		_ = stmt.Close()
		stmt = nil
		if err := tx.Commit(); err != nil {
			tx = nil
			return err
		}
		tx = nil
		batch = 0
		return nil
	}

	if err := openBatch(); err != nil {
		return 0, err
	}

	for {
		n, readErr := r.Read(buf)
		for i := 0; i < n; i++ {
			row := buf[i]
			if _, err := stmt.Exec(
				row.Name, row.ProteinID, row.ContigID,
				row.Start, row.Stop, row.Strand,
				row.GeneSymbol, row.ElementName, row.Scope,
				row.ElementType, row.ElementSubtype,
				row.Class, row.Subclass, row.Method,
				row.TargetLength, row.ReferenceSequenceLength,
				row.Coverage, row.Identity, row.AlignmentLength,
				row.ClosestReferenceAccession, row.ClosestReferenceName,
				row.HMMAccession, row.HMMDescription, row.HierarchyNode,
				row.Genus, row.Species,
			); err != nil {
				rollback()
				return 0, err
			}
			count++
			batch++
			if batch >= amrBatchSize {
				if err := commitBatch(); err != nil {
					return 0, err
				}
				if err := openBatch(); err != nil {
					return 0, err
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			rollback()
			return 0, fmt.Errorf("reading parquet: %w", readErr)
		}
	}

	if err := commitBatch(); err != nil {
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
		return nil, wrapStaleIndexError(dbPath, err)
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(
			&r.SampleAccession, &r.ProteinID, &r.ContigID,
			&r.Start, &r.Stop, &r.Strand,
			&r.GeneSymbol, &r.ElementName, &r.Scope,
			&r.ElementType, &r.ElementSubtype,
			&r.Class, &r.Subclass, &r.Method,
			&r.TargetLength, &r.ReferenceSequenceLength,
			&r.Coverage, &r.Identity, &r.AlignmentLength,
			&r.ClosestReferenceAccession, &r.ClosestReferenceName,
			&r.HMMAccession, &r.HMMDescription, &r.HierarchyNode,
			&r.Genus, &r.Species,
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
	if len(f.Species) > 0 {
		placeholders := make([]string, len(f.Species))
		for i, s := range f.Species {
			placeholders[i] = "?"
			args = append(args, strings.ToLower(s))
		}
		clauses = append(clauses, "LOWER(species) IN ("+strings.Join(placeholders, ",")+")")
	}

	q := `SELECT name, protein_id, contig_id, start, stop, strand,
	       gene_symbol, element_name, scope, element_type, element_subtype,
	       class, subclass, method, target_length, reference_sequence_length,
	       coverage, identity, alignment_length,
	       closest_reference_accession, closest_reference_name,
	       hmm_accession, hmm_description, hierarchy_node, genus, species
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

// wrapStaleIndexError annotates SQLite errors caused by querying an index
// built from the older AMRFinderPlus v3.12.8 schema. The "no such column"
// error is the loud failure mode (the new SQL references columns like
// gene_symbol/element_type/element_subtype that don't exist in v3 indexes).
func wrapStaleIndexError(dbPath string, err error) error {
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "no such column") {
		return err
	}
	return fmt.Errorf("AMR SQLite index %s appears stale (built from an older "+
		"AMRFinderPlus schema): %w. Run 'atb fetch --force' to rebuild.",
		filepath.Base(dbPath), err)
}
