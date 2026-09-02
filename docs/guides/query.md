# Querying genomes

Use `atb query` to search the AllTheBacteria index by species, quality thresholds, geography, sequencing platform, and more. Results stream to stdout (TSV by default) or to a file.

## Query genomes by species

```bash
# Get 10 high-quality E. coli genomes
atb query --species "Escherichia coli" --hq-only --limit 10

# With quality filters
atb query --species "Escherichia coli" \
  --hq-only \
  --min-completeness 99.5 \
  --max-contamination 0.5 \
  --min-n50 200000 \
  --sort-by N50 --sort-desc \
  --limit 20

# Select specific columns
atb query --species "Escherichia coli" --hq-only --limit 5 \
  --columns sample_accession,sylph_species,N50,Completeness_General,aws_url

# Search by genus
atb query --genus Salmonella --hq-only --limit 20

# Wildcard species search
atb query --species-like "Streptococcus%" --hq-only --limit 10
```

In `--species-like` patterns `%` matches any sequence of characters; every other character, including `_`, matches itself, so GTDB clade names such as `Streptococcus_A` work as written.

Run `atb columns` to list every name `--columns` accepts, with the table each one comes from; the same list is in the [column reference](../reference/columns.md). Names are case-sensitive and an unrecognised name is an error, so a typo stops the query instead of producing a blank column.

## Query by sequencing run accession

ATB is keyed by sample accession (`SAMEA…`, `SAMN…`, `SAMD…`). If you have run accessions instead (`ERR…`, `SRR…`, `DRR…`), `--runs` and `--run-file` translate them for you using the `run.parquet` mapping table:

```bash
# A few runs given on the command line
atb query --runs ERR1234567,SRR7654321 --columns sample_accession,sylph_species,aws_url

# A list of runs from a file, one per line
atb query --run-file runs.txt --columns sample_accession,aws_url
```

The run accessions are resolved to sample accessions before the query runs, and the resolution is reported:

```
Resolved 6000 run accession(s) to 5981 sample accession(s)
5,840 result(s)
```

Two things to know about the counts:

- **A run may carry more than one sample.** Multiplexed runs map to several sample accessions, so the sample count can exceed the run count.
- **Not every sample is in ATB.** A run can resolve to a sample the database does not hold, so the result count can be lower than the sample count. Runs that are not in the mapping table at all are listed as a warning. If none of them resolve, the query fails rather than returning the whole database.

To hand a collaborator a download list, add `--has-assembly` so samples with no assembly are dropped:

```bash
atb query --run-file runs.txt --has-assembly \
  --columns sample_accession,aws_url -o urls.tsv

atb download --from urls.tsv --workers 8
```

Without `--has-assembly` the `aws_url` column contains `NA` for samples that were never assembled. `atb download --from` skips those rows and reports how many it skipped.

`--runs` combines with `--samples` as a union, and with every other filter as an AND. `run.parquet` is one of the core tables, so `atb fetch` already downloads it.

## Filter by geography and platform (requires ENA tables)

```bash
# Salmonella from the UK, Illumina only
atb query --species "Salmonella enterica" \
  --country "United Kingdom" \
  --platform "ILLUMINA" \
  --limit 20

# Genomes collected between 2020-2023
atb query --species "Escherichia coli" \
  --collection-date-from 2020-01-01 \
  --collection-date-to 2023-12-31 \
  --limit 50
```

## Use a TOML filter file (reproducible queries)

```bash
# Create a filter file
cat > my_query.toml <<'EOF'
[filter]
species = "Escherichia coli"
hq_only = true
min_completeness = 99.0
max_contamination = 2.0
min_n50 = 100000

[output]
columns = ["sample_accession", "sylph_species", "N50", "Completeness_General", "aws_url"]
sort_by = "N50"
sort_desc = true
limit = 100
format = "tsv"
output = "ecoli_results.tsv"
EOF

# Run the query
atb query --filter my_query.toml

# CLI flags override TOML values
atb query --filter my_query.toml --limit 10
```

## Get sample details

```bash
atb info SAMD00000355
```

Output:
```
=== Assembly ===
  sample_accession:   SAMD00000355
  sylph_species:      Streptococcus pyogenes
  hq_filter:          PASS
  dataset:            661k
  aws_url:            https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMD00000355.fa.gz

=== Assembly Stats ===
  total_length: 1868526
  N50:          148451

=== CheckM2 Quality ===
  completeness_general:  99.06
  contamination:         0.03

=== MLST ===
  scheme:    ecoli_achtman_4
  ST:        131
  status:    PERFECT
  score:     100
  alleles:   adk(53);fumC(40);gyrB(47);icd(13);mdh(36);purA(28);recA(29)

=== ENA Metadata ===
  country:             Japan:Aichi
  collection_date:     1994
  instrument_platform: ILLUMINA
```

!!! note "Notes on species names"
    The database uses GTDB taxonomy (not NCBI), which splits some NCBI species and genera into lettered clades: *Enterococcus faecium* is stored as *Enterococcus_A faecium* and *Enterococcus_B faecium*. `--species` and `--genus` match across these clades automatically, so an NCBI-style name (`--species "Enterococcus faecium"`) finds every clade, while naming an explicit clade (`--species "Enterococcus_A faecium"` or `--genus Campylobacter_D`) restricts the match to it. `--species-like` remains for partial and wildcard matches. If a query still returns 0 results, the tool suggests close matches.

## Related pages

- [Downloading assemblies](download.md) — save matched genomes to disk
- [Fetching & indexing data](fetch-and-index.md) — set up the local data files before querying
