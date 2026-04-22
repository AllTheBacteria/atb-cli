package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	pq "github.com/allthebacteria/atb-cli/internal/parquet"
)

type readResult[T any] struct {
	rows []T
	err  error
}

const IndexFileName = "atb_index.sqlite"

const createSchema = `
CREATE TABLE samples (
    sample_accession TEXT PRIMARY KEY,
    run_accession TEXT,
    assembly_accession TEXT,
    sylph_species TEXT,
    hq_filter TEXT,
    asm_fasta_on_osf INTEGER,
    dataset TEXT,
    scientific_name TEXT,
    aws_url TEXT,
    osf_tarball_url TEXT,
    total_length INTEGER,
    num_contigs INTEGER,
    n50 INTEGER,
    n90 INTEGER,
    completeness REAL,
    contamination REAL,
    genome_size REAL,
    gc_content REAL,
    mlst_scheme TEXT,
    mlst_st TEXT,
    mlst_status TEXT,
    mlst_score INTEGER,
    mlst_alleles TEXT
);
CREATE INDEX idx_species ON samples(sylph_species);
CREATE INDEX idx_hq ON samples(hq_filter);
CREATE INDEX idx_dataset ON samples(dataset);
CREATE INDEX idx_mlst_st ON samples(mlst_st);
`

const insertAssembly = `
INSERT INTO samples (
    sample_accession, run_accession, assembly_accession,
    sylph_species, hq_filter, asm_fasta_on_osf, dataset,
    scientific_name, aws_url, osf_tarball_url
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(sample_accession) DO UPDATE SET
    run_accession = excluded.run_accession,
    assembly_accession = excluded.assembly_accession,
    sylph_species = excluded.sylph_species,
    hq_filter = excluded.hq_filter,
    asm_fasta_on_osf = excluded.asm_fasta_on_osf,
    dataset = excluded.dataset,
    scientific_name = excluded.scientific_name,
    aws_url = excluded.aws_url,
    osf_tarball_url = excluded.osf_tarball_url
`

const updateStats = `
UPDATE samples SET
    total_length = ?,
    num_contigs = ?,
    n50 = ?,
    n90 = ?
WHERE sample_accession = ?
`

const updateCheckM2 = `
UPDATE samples SET
    completeness = ?,
    contamination = ?,
    genome_size = ?,
    gc_content = ?
WHERE sample_accession = ?
`

const updateMLST = `
UPDATE samples SET
    mlst_scheme = ?,
    mlst_st = ?,
    mlst_status = ?,
    mlst_score = ?,
    mlst_alleles = ?
WHERE sample_accession = ?
`

const batchSize = 10_000

// Build creates or rebuilds the SQLite index from parquet files in dataDir.
func Build(dataDir string, logf func(string, ...any)) error {
	start := time.Now()

	tmpPath := filepath.Join(dataDir, IndexFileName+".tmp")
	finalPath := filepath.Join(dataDir, IndexFileName)

	_ = os.Remove(tmpPath)

	db, err := sql.Open("sqlite", tmpPath+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return fmt.Errorf("opening sqlite: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(createSchema); err != nil {
		return fmt.Errorf("creating schema: %w", err)
	}

	// Count available secondary tables to determine total steps.
	type secondaryTable struct {
		name string
		path string
	}
	var secondaries []secondaryTable
	for _, s := range []secondaryTable{
		{"assembly_stats", filepath.Join(dataDir, "assembly_stats.parquet")},
		{"checkm2", filepath.Join(dataDir, "checkm2.parquet")},
		{"mlst", filepath.Join(dataDir, "mlst.parquet")},
	} {
		if _, err := os.Stat(s.path); err == nil {
			secondaries = append(secondaries, s)
		}
	}
	totalSteps := 2 + len(secondaries) // read assembly + index assembly + each secondary
	step := 0

	stepLog := func(label string, args ...any) {
		step++
		logf("  [%d/%d] %s", step, totalSteps, fmt.Sprintf(label, args...))
	}

	// Step 1: Read assembly.
	t := time.Now()
	assemblies, err := pq.ReadAll[pq.AssemblyRow](filepath.Join(dataDir, "assembly.parquet"))
	if err != nil {
		return fmt.Errorf("reading assembly: %w", err)
	}
	assemblyCount := len(assemblies)
	stepLog("Reading assembly.parquet (%s rows) ... %s", formatRowCount(assemblyCount), elapsed(t))

	// Start reading secondary tables in parallel while assembly insert runs.
	statsCh := make(chan readResult[pq.AssemblyStatsRow], 1)
	checkm2Ch := make(chan readResult[pq.CheckM2Row], 1)
	mlstCh := make(chan readResult[pq.MLSTRow], 1)

	go func() {
		path := filepath.Join(dataDir, "assembly_stats.parquet")
		if _, err := os.Stat(path); err != nil {
			statsCh <- readResult[pq.AssemblyStatsRow]{}
			return
		}
		rows, err := pq.ReadAll[pq.AssemblyStatsRow](path)
		statsCh <- readResult[pq.AssemblyStatsRow]{rows, err}
	}()

	go func() {
		path := filepath.Join(dataDir, "checkm2.parquet")
		if _, err := os.Stat(path); err != nil {
			checkm2Ch <- readResult[pq.CheckM2Row]{}
			return
		}
		rows, err := pq.ReadAll[pq.CheckM2Row](path)
		checkm2Ch <- readResult[pq.CheckM2Row]{rows, err}
	}()

	go func() {
		path := filepath.Join(dataDir, "mlst.parquet")
		if _, err := os.Stat(path); err != nil {
			mlstCh <- readResult[pq.MLSTRow]{}
			return
		}
		rows, err := pq.ReadAll[pq.MLSTRow](path)
		mlstCh <- readResult[pq.MLSTRow]{rows, err}
	}()

	// Step 2: Index assembly rows.
	t = time.Now()
	if err := insertInBatches(db, insertAssembly, assemblyCount, func(stmt *sql.Stmt, i int) error {
		a := assemblies[i]
		_, err := stmt.Exec(
			a.SampleAccession, a.RunAccession, a.AssemblyAccession,
			a.SylphSpecies, a.HQFilter, a.AsmFastaOnOSF, a.Dataset,
			a.ScientificName, a.AWSUrl, a.OSFTarballURL,
		)
		return err
	}); err != nil {
		return fmt.Errorf("inserting assembly: %w", err)
	}
	assemblies = nil
	stepLog("Indexing assembly rows ... %s", elapsed(t))

	// Steps 3+: Merge secondary tables.
	statsResult := <-statsCh
	if statsResult.err != nil {
		return fmt.Errorf("reading assembly_stats: %w", statsResult.err)
	}
	if len(statsResult.rows) > 0 {
		t = time.Now()
		if err := insertInBatches(db, updateStats, len(statsResult.rows), func(stmt *sql.Stmt, i int) error {
			s := statsResult.rows[i]
			_, err := stmt.Exec(s.TotalLength, s.Number, s.N50, s.N90, s.SampleAccession)
			return err
		}); err != nil {
			return fmt.Errorf("updating assembly_stats: %w", err)
		}
		stepLog("Merging assembly_stats (%s rows) ... %s", formatRowCount(len(statsResult.rows)), elapsed(t))
	}

	checkm2Result := <-checkm2Ch
	if checkm2Result.err != nil {
		return fmt.Errorf("reading checkm2: %w", checkm2Result.err)
	}
	if len(checkm2Result.rows) > 0 {
		t = time.Now()
		if err := insertInBatches(db, updateCheckM2, len(checkm2Result.rows), func(stmt *sql.Stmt, i int) error {
			c := checkm2Result.rows[i]
			_, err := stmt.Exec(c.CompletenessGeneral, c.Contamination, c.GenomeSize, c.GCContent, c.SampleAccession)
			return err
		}); err != nil {
			return fmt.Errorf("updating checkm2: %w", err)
		}
		stepLog("Merging checkm2 (%s rows) ... %s", formatRowCount(len(checkm2Result.rows)), elapsed(t))
	}

	mlstResult := <-mlstCh
	if mlstResult.err != nil {
		return fmt.Errorf("reading mlst: %w", mlstResult.err)
	}
	if len(mlstResult.rows) > 0 {
		t = time.Now()
		if err := insertInBatches(db, updateMLST, len(mlstResult.rows), func(stmt *sql.Stmt, i int) error {
			m := mlstResult.rows[i]
			_, err := stmt.Exec(m.Scheme, m.ST, m.Status, m.Score, m.Alleles, m.Sample)
			return err
		}); err != nil {
			return fmt.Errorf("updating mlst: %w", err)
		}
		stepLog("Merging mlst (%s rows) ... %s", formatRowCount(len(mlstResult.rows)), elapsed(t))
	}

	// Flush WAL and close before rename.
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		logf("  warning: wal checkpoint: %v", err)
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("closing sqlite: %w", err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("renaming index: %w", err)
	}

	logf("Index ready: %s samples (%s)", formatRowCount(assemblyCount), elapsed(start))
	return nil
}

func elapsed(t time.Time) string {
	d := time.Since(t)
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return d.Truncate(100 * time.Millisecond).String()
}

func formatRowCount(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%d,%03d,%03d", n/1_000_000, (n/1_000)%1_000, n%1_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%d,%03d", n/1_000, n%1_000)
	}
	return fmt.Sprintf("%d", n)
}

// insertInBatches runs fn for each index [0, count) using batched transactions
// with prepared statements for the given SQL query.
func insertInBatches(db *sql.DB, query string, count int, fn func(*sql.Stmt, int) error) error {
	for start := 0; start < count; start += batchSize {
		end := start + batchSize
		if end > count {
			end = count
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}

		stmt, err := tx.Prepare(query)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		for i := start; i < end; i++ {
			if err := fn(stmt, i); err != nil {
				_ = stmt.Close()
				_ = tx.Rollback()
				return err
			}
		}

		if err := stmt.Close(); err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
