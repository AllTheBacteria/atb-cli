## atb sketch

Find closest ATB genomes using sketch distances

### Synopsis

Find the closest genomes in the AllTheBacteria database to your input
sequences using MinHash sketch distances (via sketchlib).

Requires sketchlib (Linux/macOS only). Run 'atb sketch install' to download it.

### Options

```
  -h, --help   help for sketch
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb](atb.md)	 - Query and download AllTheBacteria genomes
* [atb sketch fetch](atb_sketch_fetch.md)	 - Download the ATB sketch database from OSF
* [atb sketch info](atb_sketch_info.md)	 - Show information about the local sketch database
* [atb sketch install](atb_sketch_install.md)	 - Download the sketchlib binary (Linux/macOS only)
* [atb sketch query](atb_sketch_query.md)	 - Find closest ATB genomes for your input sequences

