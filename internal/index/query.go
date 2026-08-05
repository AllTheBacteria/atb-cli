package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/allthebacteria/atb-cli/internal/match"
)

// DB wraps a read-only SQLite connection to the index.
type DB struct {
	db *sql.DB
}

// Open opens the index database. Returns error if it doesn't exist.
func Open(dataDir string) (*DB, error) {
	p := filepath.Join(dataDir, IndexFileName)
	if _, err := os.Stat(p); err != nil {
		return nil, fmt.Errorf("index not found at %s: run 'atb index' to build it", p)
	}
	db, err := sql.Open("sqlite", p+"?mode=ro&_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("opening index: %w", err)
	}
	// Verify it's readable.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pinging index: %w", err)
	}
	return &DB{db: db}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// Exists checks if the index file exists in dataDir.
func Exists(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, IndexFileName))
	return err == nil
}

// QueryParams controls what the Query method returns.
type QueryParams struct {
	Species          string
	SpeciesLike      string
	Genus            string
	HQOnly           bool
	MinCompleteness  float64
	MaxContamination float64
	MinN50           int64
	Dataset          string
	HasAssembly      bool
	Samples          []string
	Columns          []string
	SortBy           string
	SortDesc         bool
	Limit            int
	Offset           int
	SequenceType     string // filter by mlst_st
	Scheme           string // filter by mlst_scheme
	MLSTStatus       string // filter by mlst_status
	MLSTOnly         bool   // exclude samples with no MLST scheme (empty or "-")
}

// userColToSQL maps user-facing column names to SQLite column names.
var userColToSQL = map[string]string{
	"sample_accession":     "sample_accession",
	"run_accession":        "run_accession",
	"assembly_accession":   "assembly_accession",
	"sylph_species":        "sylph_species",
	"hq_filter":            "hq_filter",
	"asm_fasta_on_osf":     "asm_fasta_on_osf",
	"dataset":              "dataset",
	"scientific_name":      "scientific_name",
	"aws_url":              "aws_url",
	"osf_tarball_url":      "osf_tarball_url",
	"total_length":         "total_length",
	"number":               "num_contigs",
	"N50":                  "n50",
	"N90":                  "n90",
	"Completeness_General": "completeness",
	"Contamination":        "contamination",
	"Genome_Size":          "genome_size",
	"GC_Content":           "gc_content",
	"mlst_scheme":          "mlst_scheme",
	"mlst_st":              "mlst_st",
	"mlst_status":          "mlst_status",
	"mlst_score":           "mlst_score",
	"mlst_alleles":         "mlst_alleles",
}

// sqlToUserCol is the reverse mapping, used when building result maps.
var sqlToUserCol = map[string]string{
	"num_contigs":   "number",
	"n50":           "N50",
	"n90":           "N90",
	"completeness":  "Completeness_General",
	"contamination": "Contamination",
	"genome_size":   "Genome_Size",
	"gc_content":    "GC_Content",
}

// allSQLCols is the full list of columns in SELECT order.
var allSQLCols = []string{
	"sample_accession",
	"run_accession",
	"assembly_accession",
	"sylph_species",
	"hq_filter",
	"asm_fasta_on_osf",
	"dataset",
	"scientific_name",
	"aws_url",
	"osf_tarball_url",
	"total_length",
	"num_contigs",
	"n50",
	"n90",
	"completeness",
	"contamination",
	"genome_size",
	"gc_content",
	"mlst_scheme",
	"mlst_st",
	"mlst_status",
	"mlst_score",
	"mlst_alleles",
}

// InfoRow returns all indexed columns for a single sample as a map.
func (d *DB) InfoRow(sampleAccession string) (map[string]string, error) {
	cols := strings.Join(allSQLCols, ", ")
	query := fmt.Sprintf("SELECT %s FROM samples WHERE sample_accession = ?", cols)

	row := d.db.QueryRow(query, sampleAccession)
	result, err := scanRow(row, allSQLCols)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("sample %q not found in index", sampleAccession)
	}
	return result, err
}

// MLSTForSample returns MLST fields for a single sample, or nil if not indexed.
func (d *DB) MLSTForSample(sampleAccession string) (map[string]string, error) {
	mlstCols := []string{"mlst_scheme", "mlst_st", "mlst_status", "mlst_score", "mlst_alleles"}
	q := fmt.Sprintf("SELECT %s FROM samples WHERE sample_accession = ?", strings.Join(mlstCols, ", "))
	row := d.db.QueryRow(q, sampleAccession)
	result, err := scanRow(row, mlstCols)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return result, err
}

// Query runs a filtered query returning result rows as map[string]string.
func (d *DB) Query(params QueryParams) ([]map[string]string, error) {
	var conditions []string
	var args []any

	if params.Species != "" {
		conditions = append(conditions, "lower(sylph_species) = lower(?)")
		args = append(args, params.Species)
	}
	if params.SpeciesLike != "" {
		conditions = append(conditions, "lower(sylph_species) LIKE ? ESCAPE '\\'")
		args = append(args, match.ToSQLLike(params.SpeciesLike))
	}
	if params.Genus != "" {
		conditions = append(conditions, "lower(substr(sylph_species, 1, instr(sylph_species, ' ') - 1)) = lower(?)")
		args = append(args, params.Genus)
	}
	if params.HQOnly {
		conditions = append(conditions, "hq_filter = 'PASS'")
	}
	if params.MinCompleteness > 0 {
		conditions = append(conditions, "completeness >= ?")
		args = append(args, params.MinCompleteness)
	}
	if params.MaxContamination > 0 {
		conditions = append(conditions, "contamination <= ?")
		args = append(args, params.MaxContamination)
	}
	if params.MinN50 > 0 {
		conditions = append(conditions, "n50 >= ?")
		args = append(args, params.MinN50)
	}
	if params.Dataset != "" {
		conditions = append(conditions, "lower(dataset) = lower(?)")
		args = append(args, params.Dataset)
	}
	if params.HasAssembly {
		conditions = append(conditions, "asm_fasta_on_osf != 0")
	}
	if len(params.Samples) > 0 {
		placeholders := make([]string, len(params.Samples))
		for i, s := range params.Samples {
			placeholders[i] = "?"
			args = append(args, s)
		}
		conditions = append(conditions, fmt.Sprintf("sample_accession IN (%s)", strings.Join(placeholders, ", ")))
	}
	if params.SequenceType != "" {
		conditions = append(conditions, "mlst_st = ?")
		args = append(args, params.SequenceType)
	}
	if params.Scheme != "" {
		conditions = append(conditions, "lower(mlst_scheme) = lower(?)")
		args = append(args, params.Scheme)
	}
	if params.MLSTStatus != "" {
		conditions = append(conditions, "upper(mlst_status) = upper(?)")
		args = append(args, params.MLSTStatus)
	}
	if params.MLSTOnly {
		conditions = append(conditions, "mlst_scheme != '' AND mlst_scheme != '-'")
	}

	// Determine which SQL columns to SELECT.
	selectCols := allSQLCols
	if len(params.Columns) > 0 {
		mapped := make([]string, 0, len(params.Columns))
		for _, c := range params.Columns {
			if sqlCol, ok := userColToSQL[c]; ok {
				mapped = append(mapped, sqlCol)
			} else {
				mapped = append(mapped, c)
			}
		}
		selectCols = mapped
	}

	q := fmt.Sprintf("SELECT %s FROM samples", strings.Join(selectCols, ", "))
	if len(conditions) > 0 {
		q += " WHERE " + strings.Join(conditions, " AND ")
	}

	// ORDER BY
	if params.SortBy != "" {
		sqlCol := params.SortBy
		if mapped, ok := userColToSQL[params.SortBy]; ok {
			sqlCol = mapped
		}
		dir := "ASC"
		if params.SortDesc {
			dir = "DESC"
		}
		q += fmt.Sprintf(" ORDER BY %s %s", sqlCol, dir)
	}

	// LIMIT / OFFSET
	if params.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", params.Limit)
		if params.Offset > 0 {
			q += fmt.Sprintf(" OFFSET %d", params.Offset)
		}
	} else if params.Offset > 0 {
		q += fmt.Sprintf(" LIMIT -1 OFFSET %d", params.Offset)
	}

	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	defer rows.Close()

	var results []map[string]string
	for rows.Next() {
		result, err := scanSQLRows(rows, selectCols)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

// SpeciesCount holds a species name and its sample count.
type SpeciesCount struct {
	Species string
	Count   int
}

// SpeciesList returns species sorted by descending sample count. Only samples
// with an assembly actually published on OSF (asm_fasta_on_osf == 1) are
// counted, matching the published-release semantics used by atb summarise.
func (d *DB) SpeciesList(limit int) ([]SpeciesCount, error) {
	q := `SELECT sylph_species, COUNT(*) as cnt FROM samples WHERE asm_fasta_on_osf != 0 AND sylph_species != '' AND sylph_species != 'unknown' GROUP BY sylph_species ORDER BY cnt DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := d.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("species list query: %w", err)
	}
	defer rows.Close()

	var results []SpeciesCount
	for rows.Next() {
		var sc SpeciesCount
		if err := rows.Scan(&sc.Species, &sc.Count); err != nil {
			return nil, fmt.Errorf("scanning species row: %w", err)
		}
		results = append(results, sc)
	}
	return results, rows.Err()
}

// Stats holds summary statistics for the database.
type Stats struct {
	Total      int
	HQCount    int
	TopSpecies []SpeciesCount
	Datasets   map[string]int
}

// QueryStats returns summary statistics, optionally filtered by species and/or HQ.
// Stats always exclude samples that aren't published on OSF (asm_fasta_on_osf == 0),
// so the four numbers match what users see in the published release tables.
func (d *DB) QueryStats(species string, hqOnly bool) (Stats, error) {
	// asm_fasta_on_osf gate goes first so callers see consistent totals
	// across the four sub-queries below.
	conditions := []string{"asm_fasta_on_osf != 0"}
	var args []any

	if species != "" {
		conditions = append(conditions, "lower(sylph_species) = lower(?)")
		args = append(args, species)
	}
	if hqOnly {
		conditions = append(conditions, "hq_filter = 'PASS'")
	}

	where := " WHERE " + strings.Join(conditions, " AND ")

	var stats Stats

	// Total count
	if err := d.db.QueryRow("SELECT COUNT(*) FROM samples"+where, args...).Scan(&stats.Total); err != nil {
		return stats, fmt.Errorf("counting total: %w", err)
	}

	// HQ count
	hqWhere := where
	hqArgs := append([]any{}, args...)
	if !hqOnly {
		hqWhere += " AND hq_filter = 'PASS'"
	}
	if err := d.db.QueryRow("SELECT COUNT(*) FROM samples"+hqWhere, hqArgs...).Scan(&stats.HQCount); err != nil {
		return stats, fmt.Errorf("counting hq: %w", err)
	}

	// Top species (up to 10). Keep "unknown" — it's a legitimate published
	// category (sample assembled but sylph couldn't classify). Skip only
	// truly empty species names.
	speciesQ := `SELECT sylph_species, COUNT(*) as cnt FROM samples` + where + ` AND sylph_species != '' GROUP BY sylph_species ORDER BY cnt DESC LIMIT 10`
	rows, err := d.db.Query(speciesQ, args...)
	if err != nil {
		return stats, fmt.Errorf("top species query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sc SpeciesCount
		if err := rows.Scan(&sc.Species, &sc.Count); err != nil {
			return stats, fmt.Errorf("scanning species: %w", err)
		}
		stats.TopSpecies = append(stats.TopSpecies, sc)
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}

	// Dataset counts
	datasetQ := "SELECT dataset, COUNT(*) as cnt FROM samples" + where + " GROUP BY dataset ORDER BY cnt DESC"
	dRows, err := d.db.Query(datasetQ, args...)
	if err != nil {
		return stats, fmt.Errorf("dataset query: %w", err)
	}
	defer dRows.Close()
	stats.Datasets = make(map[string]int)
	for dRows.Next() {
		var ds string
		var cnt int
		if err := dRows.Scan(&ds, &cnt); err != nil {
			return stats, fmt.Errorf("scanning dataset: %w", err)
		}
		stats.Datasets[ds] = cnt
	}
	return stats, dRows.Err()
}

// scanRow scans a single *sql.Row into a map using the given column names.
func scanRow(row *sql.Row, cols []string) (map[string]string, error) {
	ptrs := makeScanPtrs(len(cols))
	if err := row.Scan(ptrs...); err != nil {
		return nil, err
	}
	return ptrMapToStringMap(ptrs, cols), nil
}

// scanSQLRows scans the current row of *sql.Rows into a map.
func scanSQLRows(rows *sql.Rows, cols []string) (map[string]string, error) {
	ptrs := makeScanPtrs(len(cols))
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}
	return ptrMapToStringMap(ptrs, cols), nil
}

func makeScanPtrs(n int) []any {
	ptrs := make([]any, n)
	for i := range ptrs {
		ptrs[i] = new(any)
	}
	return ptrs
}

func ptrMapToStringMap(ptrs []any, cols []string) map[string]string {
	m := make(map[string]string, len(cols))
	for i, col := range cols {
		val := *(ptrs[i].(*any))
		outKey := col
		if mapped, ok := sqlToUserCol[col]; ok {
			outKey = mapped
		}
		m[outKey] = anyToString(val)
	}
	return m
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", t)
	}
}
