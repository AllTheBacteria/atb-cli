## atb index

Build or rebuild the SQLite query index

```
atb index [flags]
```

### Examples

```
  # Build the index (runs automatically after fetch)
  atb index

  # Rebuild the index
  atb index --force
```

### Options

```
      --force   rebuild even if index already exists
  -h, --help    help for index
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb](atb.md)	 - Query and download AllTheBacteria genomes

