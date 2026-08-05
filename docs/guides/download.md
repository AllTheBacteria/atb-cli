# Downloading assemblies

Use `atb download` to save genome FASTA files to disk, either from a query results file or directly from URLs.

## Download genomes

```bash
# Query, then download
atb query --species "Klebsiella pneumoniae" --hq-only --limit 10 \
  --columns sample_accession,aws_url --format csv -o results.csv
atb download --from results.csv --output-dir ./genomes

# Pipe query directly to download
atb query --species "Escherichia coli" --hq-only --limit 5 \
  --columns sample_accession,aws_url --format csv | \
  atb download --from - --output-dir ./ecoli_genomes

# Preview what would be downloaded
atb download --from results.csv --dry-run

# Download from a URL list
atb download --urls my_urls.txt --output-dir ./genomes --parallel 8

# Download a single file
atb download --url https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMD00000355.fa.gz \
  --output-dir ./genomes
```

Genomes are saved to `--output-dir` when given, otherwise to `download.output_dir` (default `~/atb/genomes`). That is a separate location from the metadata directory (`general.data_dir` / `--data-dir` / `$ATB_DATA_DIR`), so the data-dir setting does not change where downloads land.

## Samples with no assembly

Around 464,000 samples in ATB have no assembly. Their `aws_url` is the literal string `NA`. `atb download --from` skips those rows and reports the count:

```
Skipping 1065 row(s) with no assembly (aws_url is NA)
Downloading 4775 file(s) to /home/ubuntu/atb/genomes
```

To leave them out of the query results in the first place, add `--has-assembly`:

```bash
atb query --species "Escherichia coli" --has-assembly \
  --columns sample_accession,aws_url -o results.tsv
```

## Related pages

- [Querying genomes](query.md) — filter and select genomes to feed into `atb download`
- [AMR genes](amr.md) — use `--download` to fetch assemblies for AMR-matched samples
- [MLST](mlst.md) — use `--download` to fetch assemblies for MLST-matched samples
- [Finding closest genomes](sketch.md) — use `--download` to fetch assemblies for sketch-matched genomes
