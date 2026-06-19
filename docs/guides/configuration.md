# Configuration

`atb` stores its settings in `~/.config/atb/config.toml`. Use `atb config` to create, view, and update them.

## Create and view config

```bash
# Create default config
atb config init

# View config
atb config show

# Set the data directory (metadata index: parquet tables + SQLite)
atb config set general.data_dir /path/to/parquet/files

# Set where downloaded genomes go — a SEPARATE directory from the data dir
atb config set download.output_dir /path/to/genomes

# Set default download parallelism
atb config set download.parallel 8
```

!!! note "Data directory vs download directory"
    `general.data_dir` is where `atb fetch` places the parquet tables and SQLite index. `download.output_dir` is where `atb download` writes genome FASTA files. They are independent — see [Fetching & indexing data](fetch-and-index.md) for details.

## Shared installations (multi-user servers)

On a shared cluster you usually want every user to read from a single large data volume without each person editing their own config. Two env vars give an admin that without touching `$HOME`:

| Variable | Effect |
|----------|--------|
| `ATB_DATA_DIR` (alias: `ATB_DATADIR`) | Overrides `general.data_dir` from the config file. Takes precedence over the config but loses to an explicit `--data-dir` flag. |
| `ATB_CONFIG`  | Path to the config file. Useful for shipping a site-wide config (e.g. `/etc/atb/config.toml`) via `/etc/profile.d`. |

Example: drop a one-liner into `/etc/profile.d/atb.sh` and every login shell picks up the shared paths:

```sh
# /etc/profile.d/atb.sh
export ATB_DATA_DIR=/srv/atb/data
export ATB_CONFIG=/etc/atb/config.toml
```

Resolution order for the data directory is `--data-dir` flag → `ATB_DATA_DIR` env → config file → OS default.

## Related pages

- [Fetching & indexing data](fetch-and-index.md) — download the database into the configured data directory
- [`atb config` reference](../reference/cli/atb_config.md)
