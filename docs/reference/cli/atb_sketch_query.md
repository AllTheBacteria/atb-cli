## atb sketch query

Find closest ATB genomes for your input sequences

### Synopsis

Sketch your input FASTA file(s) and find the closest matches in the
AllTheBacteria sketch database. Results include ANI (Average Nucleotide
Identity) and metadata from the local ATB index.

```
atb sketch query <fasta> [fasta...] [flags]
```

### Examples

```
  # Find 10 closest genomes (default)
  atb sketch query my_genome.fasta

  # Top 50 closest
  atb sketch query my_genome.fasta --knn 50

  # Multiple input files
  atb sketch query sample1.fasta sample2.fasta

  # Batch from file list (one path per line)
  atb sketch query -f input_list.txt

  # Raw sketchlib distances without metadata enrichment
  atb sketch query my_genome.fasta --raw

  # Find closest and download their assemblies
  atb sketch query my_genome.fasta --download ./closest_genomes

  # Preview downloads without actually downloading
  atb sketch query my_genome.fasta --download ./closest_genomes --dry-run
```

### Options

```
      --download string    download matched genomes to this directory
      --dry-run            show what would be downloaded without downloading
  -f, --file-list string   file with one FASTA path per line
      --format string      output format: tsv, csv, json, table (default: auto)
  -h, --help               help for query
      --knn int            number of closest matches to return (default 10)
      --raw                output raw distances without metadata enrichment
      --threads int        CPU threads (default: all cores minus one)
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb sketch](atb_sketch.md)	 - Find closest ATB genomes using sketch distances

