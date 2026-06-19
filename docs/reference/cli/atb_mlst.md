## atb mlst

Query MLST (Multi-Locus Sequence Typing) data for bacterial genomes

```
atb mlst [flags]
```

### Examples

```
  # Get all STs for E. coli
  atb mlst --species "Escherichia coli" --hq-only --limit 20

  # Find ST131 E. coli
  atb mlst --species "Escherichia coli" --st 131

  # Query by scheme name
  atb mlst --scheme salmonella --limit 50

  # Only perfect MLST calls
  atb mlst --species "Escherichia coli" --status PERFECT --limit 20

  # Download assemblies for ST131 E. coli
  atb mlst --species "Escherichia coli" --st 131 --download -d ./st131

  # Preview download, cap at 20 assemblies
  atb mlst --species "Salmonella enterica" --status PERFECT --download --dry-run --max-samples 20

  # Filter MLST results by ENA metadata (requires ena_20250506.parquet).
  # Any ENA filter also appends country/collection_date/instrument_platform columns.
  atb mlst --species "Escherichia coli" --st 131 --country "UK"
  atb mlst --species "Salmonella enterica" --platform ILLUMINA --collection-date-from 2022-01-01 --limit 100

  # Append ENA columns without filtering (requires ena_20250506.parquet)
  atb mlst --species "Escherichia coli" --st 131 --with-ena
```

### Options

```
      --collection-date-from string   earliest ENA collection_date, YYYY-MM-DD (requires ena_20250506.parquet); rows with missing or unparseable dates are excluded
      --collection-date-to string     latest ENA collection_date, YYYY-MM-DD (requires ena_20250506.parquet); rows with missing or unparseable dates are excluded
      --country string                filter by ENA country (requires ena_20250506.parquet)
      --download                      download FASTA assemblies for matching samples
  -d, --download-dir string           directory to save downloaded assemblies (default from config)
      --dry-run                       print download URLs without downloading
      --format string                 output format: tsv, csv, json, table (default "tsv")
  -h, --help                          help for mlst
      --hq-only                       only include high-quality genomes (hq_filter=PASS)
      --limit int                     maximum number of results (0 = unlimited)
      --max-samples int               limit number of assemblies to download
  -o, --output string                 write output to file instead of stdout
      --platform string               filter by ENA instrument platform, e.g. ILLUMINA (requires ena_20250506.parquet)
      --scheme string                 filter by MLST scheme name
      --sequence-type string          filter by sequence type (ST number)
      --species string                filter by species name (case-insensitive)
      --st string                     filter by sequence type (shorthand for --sequence-type)
      --status string                 filter by MLST status (PERFECT, NOVEL, OK, MIXED, BAD, NONE, MISSING)
      --with-ena                      include country/collection_date/instrument_platform from the ENA table (requires ena_20250506.parquet)
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb](atb.md)	 - Query and download AllTheBacteria genomes

