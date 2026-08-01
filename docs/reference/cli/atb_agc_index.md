## atb agc index

Crawl the OSF collection nodes and join metadata into a searchable TSV index

### Synopsis

Crawl every OSF collection node's agc_archives/ folder and join the batch
metadata to write a separate AGC index (atb_agc_files.tsv): one row per .agc
batch with its species, OSF download URL, md5, and size. This is the index that
'atb agc download --species' searches to decide which batches to download -
generate it once and commit it for offline use (pass it back via --agc-index),
or let 'atb agc download' crawl and cache it on demand. It fails if any batch has
no species in the metadata, so a published index is never partial.

The index is a 6-column TSV (project, project_id, filename, url, md5, size_mb) -
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
  -h, --help            help for index
  -o, --output string   write the index TSV to this file (default stdout)
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb agc](atb_agc.md)	 - Download and inspect genomes in AGC archives

