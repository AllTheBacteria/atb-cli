## atb agc download

Download genome FASTA from AGC archives by accession or species

### Synopsis

Download assembled-genome FASTA from AGC archives, two ways.

By accession (the default): each accession is mapped to its AGC batch via a
cached sample->archive map, and that batch is located in the AGC batch index to
obtain its download URL; the needed .agc archives are downloaded (cache-first)
to <data-dir>/agc, then each sample is extracted with the agc binary. Accessions
come from positional arguments, a --from file (a query result with a
sample_accession column, or one accession per line; - for stdin), or piped
stdin. Batches named by the map but not yet listed in the index are reported as
"not yet available" while the collection is still being published.

By species (--species "Escherichia coli"): every batch named
<Species>_global_ordered_* is downloaded and extracted whole, with no
sample->archive map needed. Both modes share the AGC batch index, which by
default is crawled from the full OSF collection and cached (or read from a local
--agc-index file). This is the bulk "give me all of species X" path.

Run 'atb agc install' once to install the agc binary. By default each sample is
written to <output-dir>/<accession>.fa; use --combine to stream everything to one
file (or stdout).

```
atb agc download [accession...] [flags]
```

### Examples

```
  # One sample to the default output directory
  atb agc download SAMD00000344

  # Every Acinetobacter baylyi batch, combined into one FASTA
  atb agc download --species "Acinetobacter baylyi" --combine -o baylyi.fa

  # Pipe a query straight into retrieval
  atb query --species "Escherichia coli" --hq-only --limit 5 --format tsv | \
    atb agc download --from - -o ./ecoli

  # Combine many samples into one gzipped FASTA
  atb agc download --from accessions.txt --combine --gzip 6 -o all.fa.gz

  # Preview which archives would be downloaded
  atb agc download --from accessions.txt --dry-run
```

### Options

```
      --agc-index string     local AGC batch index TSV; the default crawls the full OSF collection (or the published index with --osf-node)
      --archive-dir string   directory to cache .agc archives (default <data-dir>/agc)
      --combine              write all samples to one stream/file instead of per-sample files
      --dry-run              resolve and list archives without downloading or extracting
      --from string          CSV/TSV with a sample_accession column, or one accession per line (- for stdin)
      --gzip int             gzip output at this level (0 = uncompressed)
  -h, --help                 help for download
      --keep-going           continue past unresolved/failed samples (still exits non-zero if any) (default true)
      --line-length int      FASTA line wrap length (default: agc's 80)
  -o, --output-dir string    per-sample output directory; with --combine, the single output file (default stdout)
  -p, --parallel int         parallel archive downloads (default from config)
      --refresh              re-download the archive map and archives even if cached
      --species string       fetch every batch of this species (whole-batch mode; no accession map needed)
  -t, --threads int          agc threads (default: all cores minus one)
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb agc](atb_agc.md)	 - Download and inspect genomes in AGC archives

