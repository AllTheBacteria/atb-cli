# AMR genes

Use `atb amr` to query AMR (antimicrobial resistance), stress, and virulence gene hits across AllTheBacteria genomes, using [AMRFinderPlus](https://github.com/ncbi/amr) v4.2.5 results run across all ATB genomes. All AMR, stress, and virulence data is in `amrfinderplus.parquet` (~58.5M rows, ~1.18 GB), downloaded automatically by `atb fetch` and partitioned by genus for fast queries.

!!! note "ENA metadata requirement"
    Commands that filter by country, collection date, or sequencing platform require the `ena_20250506.parquet` file. Fetch it with `atb fetch --tables ena_20250506.parquet`. Any ENA filter flag automatically implies `--with-ena`, which appends `country`, `collection_date`, and `instrument_platform` columns to the output. Without `--with-ena`, the ENA table is not read and default AMR queries remain in the millisecond tier.

## Query AMR genes

```bash
# Get all AMR gene hits for E. coli (high-quality genomes only)
atb amr --species "Escherichia coli" --hq-only --limit 100

# Filter by drug class
atb amr --species "Escherichia coli" --hq-only --class "BETA-LACTAM"

# Wildcard gene search (all beta-lactamase genes)
atb amr --species "Escherichia coli" --gene "bla%"

# Compare resistance across multiple species
atb amr --species "Escherichia coli,Klebsiella pneumoniae" --class "BETA-LACTAM"

# Wildcard species search -- GTDB names keep their underscores
atb amr --species-like "Campylobacter_D jej%" --hq-only --format tsv

# Match a species across GTDB naming variants
atb amr --species-like "Enterococcus%faecium" --hq-only --limit 50

# Find a gene across ALL genera (no species filter needed)
atb amr --gene "blaCTX-M-15" --limit 100

# Filter by ENA metadata -- country, platform, or collection date
# (requires ena_20250506.parquet: run 'atb fetch --tables ena_20250506.parquet')
# Any ENA filter implies --with-ena, so country/collection_date/instrument_platform
# are appended to the output automatically.
atb amr --species "Escherichia coli" --class "BETA-LACTAM" \
  --country "United Kingdom" --platform ILLUMINA --limit 50
atb amr --species "Salmonella enterica" --gene "blaCTX-M-15" \
  --collection-date-from 2022-01-01

# Add ENA columns to the output without filtering
# (requires ena_20250506.parquet). Without --with-ena the ENA table is not read,
# so default AMR queries stay in the millisecond tier.
atb amr --species "Escherichia coli" --class "BETA-LACTAM" --with-ena --limit 50

# Search by drug class across all genera
atb amr --class "CARBAPENEM" --limit 50

# Filter by detection quality
atb amr --species "Escherichia coli" --min-coverage 95 --min-identity 98

# Query stress response genes
atb amr --species "Escherichia coli" --type stress

# Query virulence factors
atb amr --species "Escherichia coli" --type virulence

# Query all three categories at once
atb amr --species "Escherichia coli" --type all

# Output to file (CSV)
atb amr --species "Klebsiella pneumoniae" --hq-only --format csv -o kpn_amr.csv

# Auto-gzip when -o ends in .gz; format inferred from filename (.csv.gz -> CSV)
atb amr --species "Escherichia coli" --hq-only -o ecoli_amr.tsv.gz

# Restrict to a specific list of samples (comma-separated)
atb amr --samples SAMD00093868,SAMEA104470147 --type all

# Or read accessions from a file (one per line, '#' comments allowed)
atb amr --sample-file my_isolates.txt --class "BETA-LACTAM"

# Download matching assemblies directly
atb amr --species "Escherichia coli" --class "BETA-LACTAM" --hq-only --download -d ./genomes

# Preview what would be downloaded (no actual download)
atb amr --species "Klebsiella pneumoniae" --gene "blaCTX-M-15" --download --dry-run

# Cap number of assemblies to download
atb amr --species "Escherichia coli" --gene "bla%" --download --max-samples 20 -d ./bla_genomes
```

`--species` accepts comma-separated values for multi-species comparison. When omitted, `--gene`, `--class`, or `--species-like` is required to search across all genera.

`--species-like` treats `%` as "any sequence of characters"; every other character, including `_`, matches itself. GTDB splits some genera into lettered clades -- `Campylobacter_D jejuni`, `Enterococcus_B faecium` -- so an underscore in a pattern is taken literally. `atb query --species-like` follows the same rule.

The literal text before the first `%` tells `atb amr` which genus partitions can hold a match: `"Campylobacter_D jej%"` reads one partition, `"Streptococcus%"` reads every `Streptococcus*` partition plus the shared partition holding genera too small for their own file. A pattern that starts with `%` names no genus and scans the full 1.18 GB file; `atb amr` prints a note when that happens.

The `--download` flag downloads the FASTA assembly for each unique sample in the results. Query output is always printed first. Use `--dry-run` to preview URLs without downloading, and `--max-samples` to cap the number of assemblies.

## Output columns

AMR output preserves the AMRFinderPlus v4.2.5 TSV header verbatim, so all 26 columns are present in the original case and ordering: `Name`, `Protein id`, `Contig id`, `Start`, `Stop`, `Strand`, `Element symbol`, `Element name`, `Scope`, `Type`, `Subtype`, `Class`, `Subclass`, `Method`, `Target length`, `Reference sequence length`, `% Coverage of reference`, `% Identity to reference`, `Alignment length`, `Closest reference accession`, `Closest reference name`, `HMM accession`, `HMM description`, `Hierarchy node`, `genus`, `species`.

With `--with-ena` (or any ENA filter), three extra columns are appended: `country`, `collection_date`, `instrument_platform`.

## Related pages

- [Downloading assemblies](download.md) — save matched genome FASTA files to disk
