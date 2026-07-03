## atb osf ls

List files and projects in the ATB index

### Synopsis

List files from the AllTheBacteria OSF index.

Without arguments, shows project categories with file counts and total sizes.
With a positional argument, shows files in projects matching that substring.

```
atb osf ls [filter] [flags]
```

### Examples

```
  # List all project categories
  atb osf ls

  # Show files in projects matching "AMR"
  atb osf ls AMR

  # Show files in an exact project
  atb osf ls --project AllTheBacteria/Assembly

  # Regex search across project and filename
  atb osf ls --grep "bakta.*batch"

  # Sort by size descending
  atb osf ls AMR --sort size

  # JSON output
  atb osf ls AMR --format json

  # Force refresh of the cached index
  atb osf ls --refresh
```

### Options

```
      --format string    output format: tsv, csv, json, table (default: table in terminal, tsv in pipe)
      --grep string      regex filter across project+filename
  -h, --help             help for ls
      --project string   filter by exact project prefix
      --refresh          re-download the index even if cached
      --sort string      sort order: name (default) or size
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb osf](atb_osf.md)	 - Browse and download files from the AllTheBacteria OSF repository

