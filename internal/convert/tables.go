// Package convert converts the AllTheBacteria release 1 metadata TSVs into the
// parquet files the atb CLI reads.
package convert

// Kind is the parquet type a TSV column is written as.
type Kind int

const (
	String Kind = iota
	Int64
	Float64
)

func (k Kind) String() string {
	switch k {
	case String:
		return "string"
	case Int64:
		return "int64"
	case Float64:
		return "float64"
	}
	return "unknown"
}

// Column is one TSV column and the parquet column written from it.
type Column struct {
	Name string
	Kind Kind
	// NAIsNull makes the literal NA null in a String column. Int64 and
	// Float64 columns always read NA as null.
	NAIsNull bool
	// Want, when not empty, is the only value the column may hold.
	Want string
}

// Table is one input TSV and the parquet file written from it. Columns are
// in TSV header order.
type Table struct {
	Name    string
	Input   string
	Output  string
	Columns []Column
}

// Tables lists every table in conversion order. assembly comes first because
// the coverage report compares the other tables against its samples.
var Tables = []Table{assembly, assemblyStats, checkM2, sylph, run}

// Lookup returns the table with the given name.
func Lookup(name string) (Table, bool) {
	for _, t := range Tables {
		if t.Name == name {
			return t, true
		}
	}
	return Table{}, false
}

// assembly is written from the release_1 export, which holds only the
// release 1 samples. The assembly export also holds samples outside the
// release.
var assembly = Table{
	Name:   "assembly",
	Input:  "atb.metadata.r1.release_1.tsv.gz",
	Output: "assembly.parquet",
	Columns: []Column{
		{Name: "sample_accession", Kind: String},
		{Name: "run_accession", Kind: String},
		{Name: "assembly_accession", Kind: String},
		{Name: "assembly_seqkit_sum", Kind: String},
		{Name: "duplicate_seqkit_sum_status", Kind: String},
		{Name: "asm_pipe_filter", Kind: String},
		{Name: "asm_fasta_on_osf", Kind: Int64},
		{Name: "dataset", Kind: String},
		{Name: "in_release_1", Kind: Int64, Want: "1"},
		{Name: "scientific_name", Kind: String},
		{Name: "sylph_species_pre_202505", Kind: String},
		{Name: "in_hq_pre_202505", Kind: String},
		{Name: "sylph_species", Kind: String},
		{Name: "sylph_filter", Kind: String},
		{Name: "hq_filter", Kind: String},
		{Name: "osf_tarball_filename", Kind: String},
		{Name: "osf_tarball_url", Kind: String},
		{Name: "aws_url", Kind: String},
		{Name: "comments", Kind: String},
		{Name: "agc_filename", Kind: String},
		{Name: "agc_url", Kind: String},
		{Name: "agc_md5", Kind: String},
	},
}

var assemblyStats = Table{
	Name:   "assembly_stats",
	Input:  "atb.metadata.r1.assembly_stats.tsv.gz",
	Output: "assembly_stats.parquet",
	Columns: []Column{
		{Name: "sample_accession", Kind: String},
		{Name: "total_length", Kind: Int64},
		{Name: "number", Kind: Int64},
		{Name: "mean_length", Kind: Float64},
		{Name: "longest", Kind: Int64},
		{Name: "shortest", Kind: Int64},
		{Name: "N_count", Kind: Int64},
		{Name: "Gaps", Kind: Int64},
		{Name: "N50", Kind: Int64},
		{Name: "N50n", Kind: Int64},
		{Name: "N70", Kind: Int64},
		{Name: "N70n", Kind: Int64},
		{Name: "N90", Kind: Int64},
		{Name: "N90n", Kind: Int64},
	},
}

var checkM2 = Table{
	Name:   "checkm2",
	Input:  "atb.metadata.r1.checkm2.tsv.gz",
	Output: "checkm2.parquet",
	Columns: []Column{
		{Name: "sample_accession", Kind: String},
		{Name: "Completeness_General", Kind: Float64},
		{Name: "Contamination", Kind: Float64},
		{Name: "Completeness_Specific", Kind: Float64},
		{Name: "Completeness_Model_Used", Kind: String, NAIsNull: true},
		{Name: "Translation_Table_Used", Kind: Float64},
		{Name: "Coding_Density", Kind: Float64},
		{Name: "Contig_N50", Kind: Float64},
		{Name: "Average_Gene_Length", Kind: Float64},
		{Name: "Genome_Size", Kind: Float64},
		{Name: "GC_Content", Kind: Float64},
		{Name: "Total_Coding_Sequences", Kind: Float64},
		{Name: "Additional_Notes", Kind: String, NAIsNull: true},
	},
}

var sylph = Table{
	Name:   "sylph",
	Input:  "atb.metadata.r1.sylph.tsv.gz",
	Output: "sylph.parquet",
	Columns: []Column{
		{Name: "sample_accession", Kind: String},
		{Name: "run_accession", Kind: String},
		{Name: "Genome_file", Kind: String},
		{Name: "Taxonomic_abundance", Kind: Float64},
		{Name: "Sequence_abundance", Kind: Float64},
		{Name: "Adjusted_ANI", Kind: Float64},
		{Name: "Eff_cov", Kind: Float64},
		{Name: "ANI_5_95_percentile", Kind: String},
		{Name: "Eff_lambda", Kind: String},
		{Name: "Lambda_5_95_percentile", Kind: String},
		{Name: "Median_cov", Kind: Int64},
		{Name: "Mean_cov_geq1", Kind: Float64},
		{Name: "Containment_ind", Kind: String},
		{Name: "Naive_ANI", Kind: Float64},
		{Name: "Contig_name", Kind: String},
		{Name: "Species", Kind: String},
	},
}

var run = Table{
	Name:   "run",
	Input:  "atb.metadata.r1.run.tsv.gz",
	Output: "run.parquet",
	Columns: []Column{
		{Name: "run_accession", Kind: String},
		{Name: "sample_accession", Kind: String},
		{Name: "in_661k", Kind: Int64},
		{Name: "in_ena_20240625", Kind: Int64},
		{Name: "in_ena_20240801", Kind: Int64},
		{Name: "in_ena_20250506", Kind: Int64},
		{Name: "in_ena_20260317", Kind: Int64},
		{Name: "ena_202505_batch", Kind: String},
		{Name: "fastq_md5", Kind: String},
		{Name: "meta_pass_atb", Kind: Int64},
		{Name: "meta_pass_661k", Kind: Int64},
		{Name: "pass", Kind: Int64},
		{Name: "failure_reasons", Kind: String},
		{Name: "latest_ena_source", Kind: String},
		{Name: "latest_ena_observed_date", Kind: String},
		{Name: "first_seen_date", Kind: String},
		{Name: "last_seen_date", Kind: String},
		{Name: "comments", Kind: String},
	},
}
