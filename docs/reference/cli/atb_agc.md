## atb agc

Download and inspect genomes in AGC archives

### Synopsis

Work with AllTheBacteria genomes distributed as AGC (Assembled Genomes
Compressor) archives.

Most users want 'atb agc download': it finds the right archive(s) for an accession
or species, downloads them (cache-first, MD5-verified), and extracts FASTA.

The other subcommands are lower-level: 'ls', 'info', and 'get' operate on .agc
files you already have on disk; 'install' fetches the upstream agc binary and
'index' builds the by-species index that 'download --species' searches.

Uses the upstream 'agc' binary. Run 'atb agc install' to download it
(Linux/macOS x64+arm64, Windows x64).

### Options

```
  -h, --help   help for agc
```

### Options inherited from parent commands

```
      --config string     config file (default $HOME/.atb/config.toml)
      --data-dir string   directory for the local metadata index (default ~/.local/share/atb/data; override with $ATB_DATA_DIR)
```

### SEE ALSO

* [atb](atb.md)	 - Query and download AllTheBacteria genomes
* [atb agc download](atb_agc_download.md)	 - Download genome FASTA from AGC archives by accession or species
* [atb agc get](atb_agc_get.md)	 - Extract sequences as FASTA
* [atb agc index](atb_agc_index.md)	 - Crawl the OSF collection nodes and join metadata into a searchable TSV index
* [atb agc info](atb_agc_info.md)	 - Show archive metadata
* [atb agc install](atb_agc_install.md)	 - Download the agc binary (Linux/macOS x64+arm64, Windows x64)
* [atb agc locate](atb_agc_locate.md)	 - Look up which AGC batch (species and OSF node) holds each accession
* [atb agc ls](atb_agc_ls.md)	 - List samples in an archive, or contigs in a sample

