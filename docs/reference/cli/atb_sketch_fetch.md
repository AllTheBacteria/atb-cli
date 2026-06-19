## atb sketch fetch

Download the ATB sketch database from OSF

### Synopsis

Download the AllTheBacteria sketch database (~4.2 GB) from OSF.
This is required before running 'atb sketch query'.

```
atb sketch fetch [flags]
```

### Examples

```
  atb sketch fetch
  atb sketch fetch --force
```

### Options

```
      --force   re-download even if database exists
  -h, --help    help for fetch
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb sketch](atb_sketch.md)	 - Find closest ATB genomes using sketch distances

