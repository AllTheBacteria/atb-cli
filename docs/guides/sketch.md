# Finding closest genomes

Use `atb sketch` to find the closest genomes in the ATB database to your input sequences using MinHash sketch distances via [sketchlib](https://github.com/bacpop/sketchlib.rust). Results include ANI (Average Nucleotide Identity) and enriched metadata.

**Linux/macOS only** -- sketchlib binaries are not available for Windows.

## One-time setup

1. Install sketchlib (downloads binary next to atb):
   ```bash
   atb sketch install
   ```
2. Download the ATB sketch database (~4.2 GB):
   ```bash
   atb sketch fetch
   ```

## Query genomes

```bash
# 3. Query your genome against ~3.2M ATB genomes
atb sketch query my_genome.fasta

# Top 50 closest matches
atb sketch query my_genome.fasta --knn 50

# Multiple input files
atb sketch query sample1.fasta sample2.fasta

# Batch from a file list
atb sketch query -f input_list.txt

# Find closest genomes AND download their assemblies
atb sketch query my_genome.fasta --download ./closest_genomes

# Preview downloads without actually downloading
atb sketch query my_genome.fasta --download ./closest --dry-run

# Raw sketchlib output (no metadata enrichment)
atb sketch query my_genome.fasta --raw

# JSON output
atb sketch query my_genome.fasta --format json
```

Output columns: `query`, `sample_accession`, `ani`, `species`, `N50`, `completeness`, `mlst_st`

When `--download` is used, the output directory will contain:
- Downloaded genome FASTA files (`.fa.gz`)
- `sketch_results.tsv` -- full query results with download URLs
- `manifest.json` -- download summary (consistent with `atb download`)

```bash
# Show info about the local sketch database
atb sketch info
```

See [Downloading assemblies](download.md) for more on the `--download` flag and output directory options.

## Related pages

- [Downloading assemblies](download.md) — save matched genome FASTA files to disk
