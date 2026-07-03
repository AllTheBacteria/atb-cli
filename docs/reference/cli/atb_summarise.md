## atb summarise

Print summary statistics for the ATB database

### Synopsis

Print summary statistics for the ATB database.

By default prints total counts, HQ fraction, top species, and dataset breakdown
for assemblies actually published on OSF (asm_fasta_on_osf == 1). To include
unprocessed or rejected samples, run an unfiltered query and pipe it in:

  atb query --format csv | atb summarise --from -

Use --by to group results by a specific column (e.g. --by sylph_species).

```
atb summarise [flags]
```

### Examples

```
  # Default summary of the full database
  atb summarise

  # Group by species (top 20)
  atb summarise --by sylph_species --top 20

  # Summarise a previous query result
  atb summarise --from salmonella.tsv

  # Pipe query to summarise
  atb query --genus Salmonella --hq-only --limit 100 --format csv | atb summarise --from -
```

### Options

```
      --by strings    group results by column(s) (e.g. --by sylph_species)
      --from string   read rows from a CSV/TSV query result file (use "-" for stdin)
  -h, --help          help for summarise
      --top int       number of top entries to show per group (default 10)
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb](atb.md)	 - Query and download AllTheBacteria genomes

