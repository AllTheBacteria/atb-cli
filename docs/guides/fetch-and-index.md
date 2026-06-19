# Fetching & indexing data

Use `atb fetch` to download the AllTheBacteria parquet tables into your **data directory** — the local metadata store that `atb query`, `atb amr`, `atb mlst`, and `atb summarise` all read from.

!!! note "Data directory vs download directory"
    The **data directory** (`general.data_dir` / `--data-dir` / `$ATB_DATA_DIR`, default `~/.local/share/atb/data`) holds the downloaded parquet tables and the SQLite query index. The **download / output directory** (`download.output_dir` / `--output-dir`, default `~/atb/genomes`) is where `atb download` writes genome FASTA files. These are independent paths — changing one does not affect the other. See [Configuration](configuration.md) to set either.

## Fetch the database

```bash
# Download core tables including AMR and MLST (~1.8 GB)
# Includes: assembly, assembly_stats, checkm2, sylph, run, mlst, amrfinderplus
atb fetch

# Download all tables including ENA metadata (~4.3 GB)
atb fetch --all

# Download specific tables only
atb fetch --tables ena_20250506.parquet

# Force re-download
atb fetch --force

# Rebuild the SQLite query index (runs automatically after fetch)
atb index --force
```

## Related pages

- [Configuration](configuration.md) — set the data directory and other defaults
- [Querying genomes](query.md) — run queries once the data is in place
- [`atb fetch` reference](../reference/cli/atb_fetch.md)
- [`atb index` reference](../reference/cli/atb_index.md)
