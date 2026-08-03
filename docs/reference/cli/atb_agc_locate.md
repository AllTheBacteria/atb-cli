## atb agc locate

Look up which AGC batch (species and OSF node) holds each accession

### Synopsis

Resolve sample accessions to the AGC batch that contains them, without
downloading anything. This is the search half of 'atb agc download': it answers
"which batch is my sample in, and is that batch available yet?".

Accessions come from positional arguments, a --from file (a query result with a
sample_accession column, or one accession per line; - for stdin), or piped stdin.
The accession->batch map and the batch index are fetched and cached exactly as
'atb agc download' fetches them; no agc binary is required.

```
atb agc locate [accession...] [flags]
```

### Examples

```
  # One accession
  atb agc locate SAMEA2247573

  # A whole query result, as JSON for a pipeline
  atb query --species "Escherichia coli" --limit 5 --format tsv | \
    atb agc locate --from - --format json
```

### Options

```
      --format string   output format: tsv or json (default "tsv")
      --from string     CSV/TSV with a sample_accession column, or one accession per line (- for stdin)
  -h, --help            help for locate
      --refresh         re-fetch the accession->batch map and the batch index even if cached
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb agc](atb_agc.md)	 - Download and inspect genomes in AGC archives

