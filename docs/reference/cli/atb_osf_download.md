## atb osf download

Download files from OSF matching a pattern

### Synopsis

Download files from the AllTheBacteria OSF repository.

Patterns are matched as regex against "project/filename".
Use --project to filter by project prefix first.

```
atb osf download <pattern> [pattern...] [flags]
```

### Examples

```
  # Download files matching a pattern
  atb osf download "AMRFinderPlus.*results.*latest"

  # Download all files in a project
  atb osf download --project AllTheBacteria/MLST --all

  # Preview what would be downloaded
  atb osf download --dry-run "bakta.*batch.*1\."

  # Download to a specific directory
  atb osf download -o ./data "Assembly.*batch1"

  # Download every assembly batch tarball and extract FASTAs as .fa.gz
  atb osf download --project AllTheBacteria/Assembly --all --extract --compress gz --delete-archive -o ./assemblies

  # Download one batch and extract raw (uncompressed) FASTAs
  atb osf download "Assembly.*batch.*1\." --extract --compress none
```

### Options

```
      --all                 download all matching files
      --compress string     compression for extracted files: none, gz, or xz (requires --extract) (default "gz")
      --delete-archive      delete each source archive after successful extraction (requires --extract)
      --dry-run             print files without downloading
      --extract             extract downloaded tar archives (.tar.xz/.tar.gz/.tar)
      --force               re-download even if file exists
  -h, --help                help for download
  -o, --output-dir string   directory to save downloads (default: data dir)
  -p, --parallel int        parallel downloads (default from config)
      --project string      filter by project prefix
      --refresh             re-download the index even if cached
      --verify              verify MD5 after download
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb osf](atb_osf.md)	 - Browse and download files from the AllTheBacteria OSF repository

