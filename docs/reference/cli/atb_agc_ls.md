## atb agc ls

List samples in an archive, or contigs in a sample

### Synopsis

With one argument, list the sample names in the archive.
With two arguments, list the contig names within the given sample.

```
atb agc ls <file.agc> [sample] [flags]
```

### Examples

```
  atb agc ls genomes.agc
  atb agc ls genomes.agc SAMD00000344
```

### Options

```
  -h, --help   help for ls
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb agc](atb_agc.md)	 - Download and inspect genomes in AGC archives

