## atb download

Download genome assemblies

### Synopsis

Download genome assembly files from ATB. URLs can be supplied via --url, --urls, --from, or stdin.

```
atb download [flags]
```

### Examples

```
  # Download genomes from a query result file
  atb download --from results.tsv --output-dir ./genomes

  # Pipe query results directly to download
  atb query --species "Escherichia coli" --hq-only --limit 5 --format csv | atb download --from - -o ./ecoli

  # Preview what would be downloaded
  atb download --from results.tsv --dry-run

  # Download a single genome
  atb download --url https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMD00000355.fa.gz -o ./genomes

  # Download with higher parallelism
  atb download --from results.tsv -o ./genomes --parallel 8
```

### Options

```
      --dry-run             print URLs without downloading
      --force               re-download even if file exists; skip disk space check
      --from string         CSV/TSV query result file to extract aws_url column from
  -h, --help                help for download
      --max-samples int     limit number of downloads
  -o, --output-dir string   directory to save downloaded genomes (default ~/atb/genomes; config key download.output_dir)
  -p, --parallel int        parallel downloads (default from config)
      --url string          single URL to download
      --urls string         file with one URL per line
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb](atb.md)	 - Query and download AllTheBacteria genomes

