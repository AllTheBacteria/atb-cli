package query

// QueryPlan describes which parquet tables to read for a query.
type QueryPlan struct {
	Tables []string
}

var checkm2Fields = map[string]bool{
	"Completeness_General":  true,
	"Contamination":         true,
	"Completeness_Specific": true,
	"Genome_Size":           true,
	"GC_Content":            true,
}

var assemblyStatsFields = map[string]bool{
	"total_length": true,
	"number":       true,
	"mean_length":  true,
	"longest":      true,
	"shortest":     true,
	"N50":          true,
	"N90":          true,
}

var sylphFields = map[string]bool{
	"Adjusted_ANI":        true,
	"Taxonomic_abundance": true,
	"Sequence_abundance":  true,
	"Median_cov":          true,
}

var mlstFields = map[string]bool{
	"mlst_scheme":  true,
	"mlst_st":      true,
	"mlst_status":  true,
	"mlst_score":   true,
	"mlst_alleles": true,
}

var enaFields = map[string]bool{
	"country":             true,
	"collection_date":     true,
	"instrument_platform": true,
	"instrument_model":    true,
	"read_count":          true,
	"base_count":          true,
	"library_strategy":    true,
	"study_accession":     true,
	"fastq_ftp":           true,
}

func columnsContainAny(columns []string, fields map[string]bool) bool {
	for _, col := range columns {
		if fields[col] {
			return true
		}
	}
	return false
}

// Plan determines which parquet tables are needed based on filters and output columns.
// Tables are returned in canonical order: assembly, checkm2, assembly_stats, sylph, mlst, run, ena_20250506.
func Plan(filters Filters, columns []string) QueryPlan {
	tables := []string{"assembly"}

	if filters.NeedsCheckM2() || columnsContainAny(columns, checkm2Fields) {
		tables = append(tables, "checkm2")
	}

	if filters.NeedsAssemblyStats() || columnsContainAny(columns, assemblyStatsFields) {
		tables = append(tables, "assembly_stats")
	}

	if columnsContainAny(columns, sylphFields) {
		tables = append(tables, "sylph")
	}

	if columnsContainAny(columns, mlstFields) {
		tables = append(tables, "mlst")
	}

	if filters.NeedsENA() || columnsContainAny(columns, enaFields) {
		tables = append(tables, "run", "ena_20250506")
	}

	return QueryPlan{Tables: tables}
}
