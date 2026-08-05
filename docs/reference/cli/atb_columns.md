## atb columns

List the columns available to atb query --columns

### Synopsis

List every column name accepted by atb query --columns, grouped by the table
it comes from.

Columns marked with * are not held in the SQLite index. Asking for one sends the
query to the parquet files, which is slower and needs those files downloaded.

This command reads no data, so it works before anything has been fetched.

```
atb columns [flags]
```

### Examples

```
  # List every column
  atb columns

  # Just the names, for a script
  atb columns --format tsv | cut -f1 | tail -n +2

  # Use them
  atb query --species "Escherichia coli" --columns sample_accession,N50,Contamination
```

### Options

```
      --format string   output format: table, tsv, csv, json
  -h, --help            help for columns
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb](atb.md)	 - Query and download AllTheBacteria genomes

