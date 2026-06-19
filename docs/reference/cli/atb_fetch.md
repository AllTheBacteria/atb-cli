## atb fetch

Download ATB parquet metadata tables

### Synopsis

Download parquet metadata tables from AllTheBacteria.

By default the core tables are downloaded, which includes amrfinderplus.parquet.
Note: AMR data is ~1.2 GB to download but expands to ~35 GB of per-genus indexes,
so you'll be prompted to confirm before download and again before index build.
Use --yes to skip both prompts and proceed automatically.

Use --all to download all available tables.
Use --tables to specify exact tables by name.

```
atb fetch [flags]
```

### Examples

```
  # Download core tables (includes AMR data)
  atb fetch

  # Download and build indexes automatically (no prompt)
  atb fetch --yes

  # Download all tables including ENA metadata
  atb fetch --all

  # Force re-download
  atb fetch --force

  # Download specific tables
  atb fetch --tables assembly.parquet,amrfinderplus.parquet
```

### Options

```
      --all              download all available tables
      --force            re-download even if table already exists
  -h, --help             help for fetch
      --parallel int     parallel downloads (default from config)
      --tables strings   specific tables to download (comma-separated names)
  -y, --yes              build indexes without prompting
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb](atb.md)	 - Query and download AllTheBacteria genomes

