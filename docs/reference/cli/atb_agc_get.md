## atb agc get

Extract sequences as FASTA

### Synopsis

Extract sequences from an AGC archive as FASTA.

Three mutually exclusive selections:
  - one or more contig queries (positional): contig[@sample][:from-to]
  - --sample: extract whole sample(s)
  - --all:    extract the entire collection

Output goes to stdout unless -o is given.

```
atb agc get <file.agc> [contig...] [flags]
```

### Examples

```
  # One contig region to stdout
  atb agc get genomes.agc "contig_1@SAMD00000344:1000-2000"

  # Whole samples to a file
  atb agc get genomes.agc --sample SAMD00000344 --sample SAMD00000345 -o out.fa

  # Entire collection, gzip level 6, 8 threads
  atb agc get genomes.agc --all --gzip 6 -t 8 -o all.fa.gz
```

### Options

```
      --all               extract the entire collection
      --gzip int          gzip the output at this level (0 = uncompressed)
  -h, --help              help for get
  -l, --line-length int   FASTA line wrap length (default: agc's 80)
  -o, --output string     write FASTA to this file (default stdout)
      --sample strings    extract whole sample(s) by name (repeatable)
  -s, --streaming         streaming mode: slower, lower memory (ignored with --all)
  -t, --threads int       CPU threads (default: all cores minus one)
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb agc](atb_agc.md)	 - Download and inspect genomes in AGC archives

