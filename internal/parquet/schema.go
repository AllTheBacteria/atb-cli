package parquet

import "strings"

type AssemblyRow struct {
	SampleAccession   string `parquet:"sample_accession"`
	RunAccession      string `parquet:"run_accession"`
	AssemblyAccession string `parquet:"assembly_accession"`
	SylphSpecies      string `parquet:"sylph_species"`
	HQFilter          string `parquet:"hq_filter"`
	AsmFastaOnOSF     int64  `parquet:"asm_fasta_on_osf"`
	Dataset           string `parquet:"dataset"`
	ScientificName    string `parquet:"scientific_name"`
	AWSUrl            string `parquet:"aws_url"`
	OSFTarballURL     string `parquet:"osf_tarball_url"`
}

type AssemblyStatsRow struct {
	SampleAccession string  `parquet:"sample_accession"`
	TotalLength     int64   `parquet:"total_length"`
	Number          int64   `parquet:"number"`
	MeanLength      float64 `parquet:"mean_length"`
	Longest         int64   `parquet:"longest"`
	Shortest        int64   `parquet:"shortest"`
	N50             int64   `parquet:"N50"`
	N90             int64   `parquet:"N90"`
}

type CheckM2Row struct {
	SampleAccession      string  `parquet:"sample_accession"`
	CompletenessGeneral  float64 `parquet:"Completeness_General"`
	Contamination        float64 `parquet:"Contamination"`
	CompletenessSpecific float64 `parquet:"Completeness_Specific"`
	GenomeSize           float64 `parquet:"Genome_Size"`
	GCContent            float64 `parquet:"GC_Content"`
}

type RunRow struct {
	RunAccession    string `parquet:"run_accession"`
	SampleAccession string `parquet:"sample_accession"`
	Pass            int64  `parquet:"pass"`
}

type SylphRow struct {
	SampleAccession    string  `parquet:"sample_accession"`
	RunAccession       string  `parquet:"run_accession"`
	AdjustedANI        float64 `parquet:"Adjusted_ANI"`
	TaxonomicAbundance float64 `parquet:"Taxonomic_abundance"`
	SequenceAbundance  float64 `parquet:"Sequence_abundance"`
	MedianCov          int64   `parquet:"Median_cov"`
	Species            string  `parquet:"Species"`
}

type ENARow struct {
	RunAccession       string `parquet:"run_accession"`
	SampleAccession    string `parquet:"sample_accession"`
	Country            string `parquet:"country"`
	CollectionDate     string `parquet:"collection_date"`
	InstrumentPlatform string `parquet:"instrument_platform"`
	InstrumentModel    string `parquet:"instrument_model"`
	ReadCount          int64  `parquet:"read_count"`
	BaseCount          int64  `parquet:"base_count"`
	LibraryStrategy    string `parquet:"library_strategy"`
	StudyAccession     string `parquet:"study_accession"`
	FastqFTP           string `parquet:"fastq_ftp"`
}

// AMRRow mirrors every column in the AMRFinderPlus v4.2.5 results parquet.
// The parquet tags are byte-for-byte identical to the AMRFP TSV header line,
// so downstream tooling sees the same column names regardless of source.
// Pre-v4 files (e.g. the old v3.12.8 dump) will not deserialize correctly.
type AMRRow struct {
	Name                      string  `parquet:"Name"`
	ProteinID                 string  `parquet:"Protein id"`
	ContigID                  string  `parquet:"Contig id"`
	Start                     int64   `parquet:"Start"`
	Stop                      int64   `parquet:"Stop"`
	Strand                    string  `parquet:"Strand"`
	GeneSymbol                string  `parquet:"Element symbol"`
	ElementName               string  `parquet:"Element name"`
	Scope                     string  `parquet:"Scope"`
	ElementType               string  `parquet:"Type"`
	ElementSubtype            string  `parquet:"Subtype"`
	Class                     string  `parquet:"Class"`
	Subclass                  string  `parquet:"Subclass"`
	Method                    string  `parquet:"Method"`
	TargetLength              int64   `parquet:"Target length"`
	ReferenceSequenceLength   int64   `parquet:"Reference sequence length"`
	Coverage                  float64 `parquet:"% Coverage of reference"`
	Identity                  float64 `parquet:"% Identity to reference"`
	AlignmentLength           int64   `parquet:"Alignment length"`
	ClosestReferenceAccession string  `parquet:"Closest reference accession"`
	ClosestReferenceName      string  `parquet:"Closest reference name"`
	HMMAccession              string  `parquet:"HMM accession"`
	HMMDescription            string  `parquet:"HMM description"`
	HierarchyNode             string  `parquet:"Hierarchy node"`
	Genus                     string  `parquet:"genus"`
	Species                   string  `parquet:"species"`
}

type MLSTRow struct {
	Sample  string `parquet:"sample"`
	Scheme  string `parquet:"mlst_scheme"`
	ST      string `parquet:"mlst_st"`
	Status  string `parquet:"mlst_status"`
	Score   int32  `parquet:"mlst_score"`
	Alleles string `parquet:"mlst_alleles"`
}

func GenusFromSpecies(species string) string {
	if species == "" {
		return ""
	}
	parts := strings.SplitN(species, " ", 2)
	return parts[0]
}
