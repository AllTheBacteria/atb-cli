## atb query

Query the ATB database

### Synopsis

Query bacterial genome metadata from local parquet tables.

```
atb query [flags]
```

### Examples

```
  # Get 10 high-quality E. coli genomes
  atb query --species "Escherichia coli" --hq-only --limit 10

  # Filter by quality and sort by N50
  atb query --species "Escherichia coli" --min-completeness 99 --min-n50 200000 --sort-by N50 --sort-desc --limit 20

  # Search by genus with specific columns
  atb query --genus Salmonella --hq-only --columns sample_accession,sylph_species,N50 --limit 20

  # Wildcard species search
  atb query --species-like "Streptococcus%" --hq-only --limit 10

  # Filter by country (requires ENA data)
  atb query --species "Salmonella enterica" --country "United Kingdom" --limit 20

  # Use a TOML filter file for reproducible queries
  atb query --filter my_query.toml

  # Output as CSV to a file
  atb query --species "Escherichia coli" --hq-only --format csv -o results.csv
```

### Options

```
      --collection-date-from string   collection date lower bound (YYYY-MM-DD); rows with missing or unparseable dates are excluded
      --collection-date-to string     collection date upper bound (YYYY-MM-DD); rows with missing or unparseable dates are excluded
      --columns strings               columns to include in output
      --country string                filter by country of origin (ENA metadata)
      --dataset string                filter by dataset name
      --filter string                 TOML filter file
      --format string                 output format: tsv, csv, json, table, auto
      --genus string                  filter by genus
      --has-assembly                  only samples with assembly FASTA on OSF
  -h, --help                          help for query
      --hq-only                       only return high-quality genomes (HQFilter=PASS)
      --limit int                     maximum number of rows to return
      --max-contamination float       maximum CheckM2 contamination
      --min-completeness float        minimum CheckM2 completeness
      --min-n50 int                   minimum assembly N50
      --offset int                    number of rows to skip
  -o, --output string                 write output to file instead of stdout
      --platform string               filter by sequencing platform (ENA metadata)
      --sample-file string            file with one sample accession per line
      --samples strings               comma-separated sample accessions
      --sort-by string                column to sort by
      --sort-desc                     sort in descending order
      --species string                exact species name (case-insensitive)
      --species-like string           wildcard species match (% is the wildcard, _ is literal)
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb](atb.md)	 - Query and download AllTheBacteria genomes

