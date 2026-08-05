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
    The database uses GTDB taxonomy (not NCBI). Some species names differ from common usage. If a query returns 0 results, the tool suggests close matches. Example: *Enterococcus faecium* in GTDB may be *Enterococcus_B faecium*. Use `--species-like "Enterococcus%faecium"` to search across GTDB naming variants.

## Related pages

- [Downloading assemblies](download.md) — save matched genomes to disk
- [Fetching & indexing data](fetch-and-index.md) — set up the local data files before querying
