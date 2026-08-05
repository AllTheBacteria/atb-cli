## atb amr

Query AMR gene data

### Synopsis

Query AMRFinderPlus gene hits from the merged amrfinderplus.parquet file.

Use --species for a full-species match (e.g. "Escherichia coli"), --species-like
for a wildcard match (e.g. "Campylobacter_D jej%"), or --genus for a broader
genus-level match (e.g. "Escherichia"). --species and --genus accept comma-
separated lists and may be combined.

In --species-like patterns % matches any sequence of characters and _ matches
itself, so GTDB names such as "Campylobacter_D" work as written. A pattern that
starts with % cannot be narrowed to a genus and scans the full dataset.

When no filter is given (--species, --species-like, --genus, --gene, --class),
the full AMR dataset is scanned; you'll be prompted to confirm, or pass --yes to
skip the prompt.

Run 'atb fetch' to download the data before querying.

```
atb amr [flags]
```

### Examples

```
  # Get AMR gene hits for E. coli (HQ only)
  atb amr --species "Escherichia coli" --hq-only --limit 100

  # Wildcard species match (GTDB names keep their underscores)
  atb amr --species-like "Campylobacter_D jej%" --hq-only --format tsv

  # Filter by drug class
  atb amr --species "Escherichia coli" --class "BETA-LACTAM"

  # Search for beta-lactamase genes in E. coli
  atb amr --species "Escherichia coli" --gene "bla%"

  # Compare beta-lactam resistance across species
  atb amr --species "Escherichia coli,Klebsiella pneumoniae" --class "BETA-LACTAM"

  # Find a gene across ALL genera (no species filter)
  atb amr --gene "blaCTX-M-15" --limit 100

  # Restrict to a specific sample list (parity with 'atb query --samples')
  atb amr --samples SAMD00093868,SAMD00093869 --type all
  atb amr --sample-file my_samples.txt --hq-only --class "BETA-LACTAM"

  # Query stress response genes
  atb amr --species "Escherichia coli" --type stress

  # Query all element types (AMR + stress + virulence)
  atb amr --species "Klebsiella pneumoniae" --type all --hq-only

  # Download assemblies with beta-lactam resistance
  atb amr --species "Escherichia coli" --class "BETA-LACTAM" --hq-only --download -d ./genomes

  # Preview assemblies that would be downloaded
  atb amr --species "Klebsiella pneumoniae" --gene "blaCTX-M-15" --download --dry-run

  # Filter by ENA metadata (requires ena_20250506.parquet).
  # Any ENA filter also appends country/collection_date/instrument_platform columns.
  atb amr --species "Escherichia coli" --class "BETA-LACTAM" --country "UK" --platform ILLUMINA
  atb amr --species "Salmonella enterica" --gene "blaCTX-M-15" --collection-date-from 2022-01-01

  # Append ENA columns without filtering (requires ena_20250506.parquet)
  atb amr --species "Escherichia coli" --class "BETA-LACTAM" --with-ena
```

### Options

```
      --class string                  filter by drug class (case-insensitive, substring match)
      --collection-date-from string   earliest ENA collection_date, YYYY-MM-DD (requires ena_20250506.parquet); rows with missing or unparseable dates are excluded
      --collection-date-to string     latest ENA collection_date, YYYY-MM-DD (requires ena_20250506.parquet); rows with missing or unparseable dates are excluded
      --country string                filter by ENA country (requires ena_20250506.parquet)
      --download                      download FASTA assemblies for matching samples
  -d, --download-dir string           directory to save downloaded assemblies (default from config)
      --dry-run                       print download URLs without downloading
      --format string                 output format: tsv, csv, json, table, auto
      --gene string                   filter by gene symbol (supports % wildcards)
      --genus string                  filter by genus, e.g. "Escherichia" (comma-separated for multiple)
  -h, --help                          help for amr
      --hq-only                       only include HQ samples (hq_filter=PASS)
      --limit int                     maximum number of results
      --max-samples int               limit number of assemblies to download
      --min-coverage float            minimum coverage %
      --min-identity float            minimum identity %
  -o, --output string                 write output to file instead of stdout
      --platform string               filter by ENA instrument platform, e.g. ILLUMINA (requires ena_20250506.parquet)
      --sample-file string            file with one sample accession per line (combined with --samples)
      --samples strings               comma-separated sample accessions to restrict the query to
      --species string                filter by full species name, e.g. "Escherichia coli" (comma-separated for multiple)
      --species-like string           wildcard species match, e.g. "Campylobacter_D jej%" (% is the wildcard, _ is literal)
      --type string                   element type: amr, stress, virulence, all (default: all types)
      --with-ena                      include country/collection_date/instrument_platform from the ENA table (requires ena_20250506.parquet)
  -y, --yes                           skip the confirmation prompt for an unfiltered full-dataset scan
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb](atb.md)	 - Query and download AllTheBacteria genomes

