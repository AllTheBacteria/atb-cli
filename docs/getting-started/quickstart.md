# Quick Start

Download the core database tables and run your first query in two commands.

```bash
# 1. Download the database (~540 MB core tables)
atb fetch

# 2. Query
atb query --species "Escherichia coli" --hq-only --limit 10
```

If you don't have the parquet files yet:

```bash
# Download core tables from OSF (~540MB)
./bin/atb fetch

# Or download all tables including ENA metadata (~4.3 GB)
./bin/atb fetch --all
```

!!! tip "Core tables are ~540 MB"
    The default `atb fetch` downloads the core parquet tables (~540 MB), which is enough for most queries. For full ENA metadata or AMR tables, see [Fetching & indexing data](../guides/fetch-and-index.md) for all available options and how to select specific tables.
