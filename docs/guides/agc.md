# Fetching genomes from AGC archives

AllTheBacteria assemblies are distributed as **AGC** (Assembled Genome Compressor) archives - each `.agc` batch holds many genomes with cross-genome delta compression, so a ~1 MB archive can expand to hundreds of MB of FASTA. The `atb agc` command group works with these archives:

- **`atb agc download`** - the subcommand most people want. It finds the right archive(s) for you, downloads them (cache-first, MD5-verified), and extracts FASTA. Works **by accession** or **by species**.
- **`atb agc ls` / `info` / `get`** - low-level tools for `.agc` files you already have on disk (list, inspect, extract).
- **`atb agc index`** - build a searchable index of the OSF-hosted archives (what `--species` searches).
- **`atb agc install`** - fetch the upstream `agc` binary the others shell out to.

!!! info "Availability"
    The `atb agc` command group is new in **v0.18.0**; it is not in v0.17.x. The 0.18.0 line is currently in beta while the by-species workflow settles, so install it from the [v0.18.0-beta.2 pre-release](https://github.com/AllTheBacteria/atb-cli/releases/tag/v0.18.0-beta.2) (newer betas appear on the [releases page](https://github.com/AllTheBacteria/atb-cli/releases)). The installer's default stays on the latest stable until 0.18.0 is released.

!!! note "Which command do I want?"
    Reach for **`atb agc download`** to get genomes by sample accession or by species - it handles resolution, download, and extraction. Drop down to **`atb agc ls`/`info`/`get`** only when you already have a local `.agc` file, or to **`atb agc index`** when you want to (re)generate the by-species index. All shell out to the upstream `agc` binary, so install it first.

## Install the agc binary

`atb` calls the upstream `agc` binary; install it once (Linux/macOS x64+arm64, Windows x64):

```bash
atb agc install
```

It is installed alongside the `atb` binary. Re-running is a no-op if `agc` is already installed (found next to `atb` or on `PATH`).

## Fetch genomes by species

Download **every** batch of a species and extract it whole - no accession list needed. Batches are matched against the index's species column: `atb agc index` joins each numbered batch (`atb.assembly.202505_all.batch.NNNN.agc`, which carries no species itself) with its species from the batch metadata when the index is built.

```bash
# Every Acinetobacter baylyi batch, combined into one FASTA
atb agc download --species "Acinetobacter baylyi" --combine -o baylyi.fa

# Same, gzipped, using 8 agc threads
atb agc download --species "Mycoplasmoides pneumoniae" \
    --combine --gzip 6 -t 8 -o mpneumoniae.fa.gz

# Preview which batches would be downloaded (no download, no extract)
atb agc download --species "Salmonella enterica" --dry-run
```

The first by-species run builds the batch index (`atb_agc_files.tsv`) and caches it
for 7 days. By default `atb` crawls the three OSF nodes that host the ATB v202505
collection and joins every batch with its species from the batch metadata,
combining the result into one index; a published combined index TSV is downloaded
directly instead when one is configured. The cache records which source produced
it, so a release that repoints the index refreshes your local copy automatically.
Use `--refresh` to force a re-fetch of the index and re-download archives.

!!! tip "Offline or pinned index"
    Pass a local index TSV with `--agc-index` to skip the network fetch entirely - useful for reproducible runs or air-gapped environments. Generate the file once with [`atb agc index`](#build-the-by-species-index):

    ```bash
    atb agc download --species "Acinetobacter baylyi" \
        --agc-index atb_agc_files.tsv --combine -o baylyi.fa
    ```

## Fetch genomes by accession

The default mode. Accessions resolve to AGC archives through a cached
sample→archive map; those archives are then found in the same batch index that
`--species` uses - so `--agc-index` applies here too - then downloaded and each
sample extracted by name. Accessions come from arguments, a `--from` file, or
stdin. A sample whose batch is named but not yet listed in the index is reported
as not yet available (the collection is still being published).

```bash
# One sample to the default per-sample output directory
atb agc download SAMD00000344

# Several samples, each written to ./out/<accession>.fa
atb agc download SAMD00000344 SAMD00000345 -o ./out

# Pipe a query straight into retrieval
atb query --species "Escherichia coli" --hq-only --limit 5 --format tsv | \
    atb agc download --from - -o ./ecoli

# Combine many accessions into one gzipped FASTA
atb agc download --from accessions.txt --combine --gzip 6 -o all.fa.gz

# Preview which archives would be downloaded
atb agc download --from accessions.txt --dry-run
```

`--from` accepts a query result with a `sample_accession` column, or a plain
list of one accession per line (`-` for stdin). By default each sample is
written to `<output-dir>/<accession>.fa`; `--combine` streams everything to a
single file (or stdout when `-o` is omitted).

Useful flags (both modes):

| Flag | Effect |
|------|--------|
| `--combine` | One output stream/file instead of per-sample files |
| `-o`, `--output-dir` | Output directory (per-sample) or file (`--combine`); stdout if omitted |
| `--gzip N` | gzip the output at level N (0 = uncompressed) |
| `--line-length N` | FASTA line wrap width (default: agc's 80) |
| `-t`, `--threads N` | agc extraction threads (default: cores − 1) |
| `-p`, `--parallel N` | Parallel archive downloads |
| `--archive-dir DIR` | Where to cache `.agc` archives (default `<data-dir>/agc`) |
| `--agc-index FILE` | Use a local batch index TSV instead of crawling the collection |
| `--refresh` | Re-download the index/map and archives even if cached |
| `--dry-run` | Resolve and list archives without downloading or extracting |
| `--keep-going` | Continue past unresolved or failed samples (on by default); still exits non-zero if any |

## Locate a sample's batch

`atb agc locate` answers "which AGC batch holds my sample, and is it available
yet?" without downloading anything. It is the search half of `atb agc download`:
the same accession→batch map and batch index, printed instead of fetched. No agc
binary is required.

```bash
# One accession
atb agc locate SAMEA2247573

# A whole query result, as JSON for a pipeline
atb query --species "Escherichia coli" --limit 5 --format tsv | \
  atb agc locate --from - --format json
```

The TSV output has four columns - `accession`, `batch`, `species`, `node` - taken
from the batch's entry in the index. JSON output adds the resolved OSF `url`. An
accession absent from the map prints `<unresolved>` for batch, species, and node; a
sample whose batch is named but not yet listed in the index prints its batch name
with `<not-yet-available>` for species and node.

!!! note "Not-yet-available batches"
    A `<not-yet-available>` result means the accession's batch is in the
    accession→batch map but not yet in the batch index - the collection node
    holding it has not finished publishing that batch. Try again later.

## Build the by-species index

`atb agc index` crawls the `agc_batches/` folder on each of the three OSF nodes
that host the ATB v202505 collection (`4jq8u`, `jmeqg`, `kzcnr`; 1,268 numbered
batches in total) and joins every batch with its species from the batch
metadata, writing a **separate** index TSV - one row per `.agc` batch with its
species, OSF download URL, MD5, and size. This is the file
`atb agc download --species` searches. Generate it once and commit it for
offline use, or let `atb agc download` crawl and cache it on demand.

```bash
# Write the index to a file you can commit / pass back via --agc-index
atb agc index -o atb_agc_files.tsv

# Print it to stdout
atb agc index
```

The index is a 6-column TSV (`project`, `project_id`, `filename`, `url`, `md5`,
`size_mb`) - the same layout as the master OSF index, so the standard parser
round-trips it.

## Inspect and extract local archives

When you already have a `.agc` file on disk, use the low-level `atb agc`
subcommands directly - no download, no index.

```bash
# List the sample names in an archive
atb agc ls genomes.agc

# List the contig names within one sample
atb agc ls genomes.agc SAMD00000344

# Show archive metadata (sample count, reference, etc.)
atb agc info genomes.agc
```

Extract sequences as FASTA with `atb agc get`. Three mutually exclusive
selections: contig queries (positional), whole samples (`--sample`), or the
entire collection (`--all`):

```bash
# One contig region to stdout: contig[@sample][:from-to]
atb agc get genomes.agc "contig_1@SAMD00000344:1000-2000"

# Whole samples to a file
atb agc get genomes.agc --sample SAMD00000344 --sample SAMD00000345 -o out.fa

# Entire collection, gzip level 6, 8 threads
atb agc get genomes.agc --all --gzip 6 -t 8 -o all.fa.gz
```

`get` also accepts `-l`/`--line-length` to set the FASTA wrap width and
`-s`/`--streaming` for lower-memory (slower) extraction.

## Related pages

- [Querying genomes](query.md) - find accessions to feed into `atb agc download`
- [Downloading genomes](download.md) - the parquet/FASTA download path
- [Fetching & indexing data](fetch-and-index.md) - set up the data directory
- [Configuration](configuration.md) - set defaults like the output directory
