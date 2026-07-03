# Available columns

The columns available in query results depend on which parquet files are loaded. Core columns are always present; additional columns are loaded on demand when you request them.

### assembly.parquet (always loaded)
`sample_accession`, `run_accession`, `assembly_accession`, `sylph_species`, `scientific_name`, `hq_filter`, `dataset`, `asm_fasta_on_osf`, `aws_url`, `osf_tarball_url`

### assembly_stats.parquet (loaded when N50/length columns requested)
`total_length`, `number`, `mean_length`, `longest`, `shortest`, `N50`, `N90`

### checkm2.parquet (loaded when quality columns requested)
`Completeness_General`, `Contamination`, `Completeness_Specific`, `Genome_Size`, `GC_Content`

### ena_20250506.parquet (loaded when geography/platform columns requested)
`country`, `collection_date`, `instrument_platform`, `instrument_model`, `read_count`, `base_count`, `library_strategy`, `study_accession`, `fastq_ftp`

See also: [Output formats](output-formats.md) for how to control the output format and [Fetching & indexing data](../guides/fetch-and-index.md) for how these parquet files are downloaded.
