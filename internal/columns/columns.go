// Package columns is the registry of columns available to atb query --columns.
package columns

// Source is the table a column is read from.
type Source string

const (
	Assembly      Source = "assembly"
	AssemblyStats Source = "assembly_stats"
	CheckM2       Source = "checkm2"
	Sylph         Source = "sylph"
	MLST          Source = "mlst"
	ENA           Source = "ena_20250506"
)

// Column is one column that --columns accepts.
type Column struct {
	Name        string
	Source      Source
	Description string
	// InIndex reports whether the SQLite index can answer the column. Columns
	// without it force the query onto the parquet files, which is slower.
	InIndex bool
}

var registry = []Column{
	{"sample_accession", Assembly, "BioSample accession identifying the sample", true},
	{"run_accession", Assembly, "ENA run accession the assembly was built from", true},
	{"assembly_accession", Assembly, "ENA assembly accession, NA when not submitted", true},
	{"sylph_species", Assembly, "Species assigned by sylph against GTDB r214", true},
	{"scientific_name", Assembly, "Species name as recorded by the submitter", true},
	{"hq_filter", Assembly, "PASS when the sample meets the high-quality criteria", true},
	{"dataset", Assembly, "Release the sample first appeared in", true},
	{"asm_fasta_on_osf", Assembly, "1 when the assembly FASTA is hosted on OSF", true},
	{"aws_url", Assembly, "S3 URL of the assembly FASTA", true},
	{"osf_tarball_url", Assembly, "OSF URL of the tarball holding the assembly", true},

	{"total_length", AssemblyStats, "Total assembly length in base pairs", true},
	{"number", AssemblyStats, "Number of contigs", true},
	{"mean_length", AssemblyStats, "Mean contig length in base pairs", false},
	{"longest", AssemblyStats, "Longest contig length in base pairs", false},
	{"shortest", AssemblyStats, "Shortest contig length in base pairs", false},
	{"N50", AssemblyStats, "Contig length at which half the assembly sits in longer contigs", true},
	{"N90", AssemblyStats, "Contig length at which 90% of the assembly sits in longer contigs", true},

	{"Completeness_General", CheckM2, "Estimated completeness percentage, general model", true},
	{"Contamination", CheckM2, "Estimated contamination percentage", true},
	{"Completeness_Specific", CheckM2, "Estimated completeness percentage, lineage-specific model", false},
	{"Genome_Size", CheckM2, "Predicted genome size in base pairs", true},
	{"GC_Content", CheckM2, "Fraction of G and C bases", true},

	{"Adjusted_ANI", Sylph, "Nucleotide identity to the species reference, coverage-adjusted", false},
	{"Taxonomic_abundance", Sylph, "Percentage of the sample's cells estimated to be the species", false},
	{"Sequence_abundance", Sylph, "Percentage of the sample's reads assigned to the species", false},
	{"Median_cov", Sylph, "Median k-mer coverage of the species reference", false},

	{"mlst_scheme", MLST, "MLST scheme applied, - when the species has none", true},
	{"mlst_st", MLST, "Sequence type assigned by the scheme", true},
	{"mlst_status", MLST, "Typing outcome: PERFECT, NOVEL, OK or NONE", true},
	{"mlst_score", MLST, "Typing confidence score out of 100", true},
	{"mlst_alleles", MLST, "Per-locus allele calls, semicolon separated", true},

	{"country", ENA, "Country the sample was collected in", false},
	{"collection_date", ENA, "Collection date, often only a year or month", false},
	{"instrument_platform", ENA, "Sequencing platform", false},
	{"instrument_model", ENA, "Sequencing instrument model", false},
	{"read_count", ENA, "Number of reads in the run", false},
	{"base_count", ENA, "Number of bases in the run", false},
	{"library_strategy", ENA, "Library preparation strategy", false},
	{"study_accession", ENA, "ENA study accession", false},
	{"fastq_ftp", ENA, "FTP URLs of the FASTQ files", false},
}

// All returns every column, grouped by source in canonical order.
func All() []Column {
	return registry
}

// Names returns every column name in the same order as All.
func Names() []string {
	names := make([]string, len(registry))
	for i, c := range registry {
		names[i] = c.Name
	}
	return names
}

// Lookup finds a column by its exact name.
func Lookup(name string) (Column, bool) {
	for _, c := range registry {
		if c.Name == name {
			return c, true
		}
	}
	return Column{}, false
}
