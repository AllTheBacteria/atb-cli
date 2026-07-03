## atb agc index

Crawl the OSF node's AGC batches into a searchable TSV index

### Synopsis

Crawl an OSF node's agc_batches/ folder and write a separate AGC index
(atb_agc_files.tsv): one row per .agc batch with its species, OSF download URL,
md5, and size. This is the index that 'atb agc download --species' searches to
decide which batches to download — generate it once and commit it for offline use
(pass it back via --agc-index), or let 'atb agc download' crawl and cache it on demand.

The index is a 6-column TSV (project, project_id, filename, url, md5, size_mb) —
the same layout as the master OSF index, so the existing parser round-trips it.

```
atb agc index [flags]
```

### Examples

```
  # Write the index to a file you can commit
  atb agc index -o atb_agc_files.tsv

  # Print it to stdout
  atb agc index
```

### Options

```
  -h, --help              help for index
      --osf-node string   OSF node to crawl (default from config or the staging node)
  -o, --output string     write the index TSV to this file (default stdout)
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb agc](atb_agc.md)	 - Download and inspect genomes in AGC archives

