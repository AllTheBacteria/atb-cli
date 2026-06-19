# Output formats

`atb` supports several output formats for query results; the default adapts to whether you are writing to a terminal or a pipe.

By default, output is a pretty table when writing to a terminal, and TSV when piped. Override with `--format`:

```bash
atb query --species "Escherichia coli" --limit 5 --format tsv     # tab-separated
atb query --species "Escherichia coli" --limit 5 --format csv     # comma-separated
atb query --species "Escherichia coli" --limit 5 --format json    # JSON array
atb query --species "Escherichia coli" --limit 5 --format table   # pretty table
```

When writing to a file with `-o`, the format is inferred from the filename if `--format` is not given (`.csv` -> csv, `.tsv` -> tsv, `.json` -> json). Append `.gz` to any of these and the output is gzipped on the fly:

```bash
atb query --species "Escherichia coli" --hq-only -o results.csv.gz   # gzipped CSV
atb amr   --species "Escherichia coli" --hq-only -o ecoli_amr.tsv.gz # gzipped TSV
```

See also: [Available columns](columns.md) for the full list of fields that can appear in query output.
