# MLST

Use `atb mlst` to query Multi-Locus Sequence Typing data across AllTheBacteria genomes, covering 2.44M samples across 156 typing schemes included in the core metadata fetch.

!!! note "ENA metadata requirement"
    Commands that filter by country, collection date, or sequencing platform require the `ena_20250506.parquet` file. Fetch it with `atb fetch --tables ena_20250506.parquet`. Any ENA filter flag automatically implies `--with-ena`, which appends `country`, `collection_date`, and `instrument_platform` columns to the output. Without `--with-ena`, the ENA table is not read and default MLST queries remain in the millisecond tier.

## Query MLST

```bash
# Get all STs for E. coli (high-quality only)
atb mlst --species "Escherichia coli" --hq-only --limit 20

# Find ST131 E. coli (a globally disseminated high-risk clone)
atb mlst --species "Escherichia coli" --st 131 --hq-only

# Query by MLST scheme name
atb mlst --scheme salmonella --limit 50
atb mlst --scheme ecoli_achtman_4 --limit 20

# Only perfect MLST calls (all alleles matched exactly)
atb mlst --species "Escherichia coli" --status PERFECT --limit 20

# Find novel sequence types (new allele combinations)
atb mlst --species "Salmonella enterica" --status NOVEL --limit 20

# Combine with species and quality filters
atb mlst --species "Klebsiella pneumoniae" --hq-only --status PERFECT --limit 50

# Filter MLST results by ENA metadata -- country, platform, or collection date
# (requires ena_20250506.parquet: run 'atb fetch --tables ena_20250506.parquet')
# Any ENA filter implies --with-ena, so country/collection_date/instrument_platform
# are appended to the output automatically.
atb mlst --species "Escherichia coli" --st 131 --country "United Kingdom"
atb mlst --species "Salmonella enterica" --platform ILLUMINA \
  --collection-date-from 2022-01-01 --limit 100

# Add ENA columns to the output without filtering
# (requires ena_20250506.parquet). Without --with-ena the ENA table is not read,
# so default MLST queries stay in the millisecond tier.
atb mlst --species "Escherichia coli" --st 131 --with-ena

# Output as CSV
atb mlst --species "Escherichia coli" --st 131 --format csv -o st131.csv

# Download assemblies for matching samples
atb mlst --species "Escherichia coli" --st 131 --download -d ./st131

# Preview download, cap at 20 assemblies
atb mlst --species "Salmonella enterica" --status PERFECT --download --dry-run --max-samples 20
```

## Output columns

MLST output columns: `sample_accession`, `sylph_species`, `mlst_scheme`, `mlst_st`, `mlst_status`, `mlst_score`, `mlst_alleles`

With `--with-ena` (or any ENA filter), three extra columns are appended: `country`, `collection_date`, `instrument_platform`.

MLST status values: `PERFECT` (exact match), `NOVEL` (new combination), `OK` (partial), `MIXED`, `BAD`, `MISSING`, `NONE`

## Related pages

- [Downloading assemblies](download.md) — save matched genome FASTA files to disk
