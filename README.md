# atb-cli

A command-line tool for querying the [AllTheBacteria](https://osf.io/xv7q9/) genomics database (~3.2M bacterial genomes), searching AMR/stress/virulence genes, finding closest genomes via sketch distances, and downloading genome assemblies.

Single binary, no dependencies.

**Supported platforms:** Linux, macOS, Windows (amd64 and arm64)

`atb-cli` was designed and architected by [Thanh Le Viet](https://github.com/thanhleviet) in his personal capacity, using his own Claude account. The implementation was developed with coding assistance from [Claude](https://claude.ai) (Anthropic), an AI assistant that helped with code generation, testing, and documentation under human direction and review. Thanks to hackathon participants Jane Hawkey, Ahmed M Moustafa, Martin Hunt, and Zamin Iqbal for their input and feedback.

## Table of Contents

- [Download](#download)
- [Install](#install)
- [Quick Start](#quick-start)
- [Updating](#updating)
- [Usage Examples](#usage-examples)
  - [Query genomes by species](#query-genomes-by-species)
  - [Filter by geography and platform](#filter-by-geography-and-platform-requires-ena-tables)
  - [Use a TOML filter file](#use-a-toml-filter-file-reproducible-queries)
  - [Get sample details](#get-sample-details)
  - [Download genome assemblies](#download-genome-assemblies)
  - [Summary statistics](#summary-statistics)
  - [Query AMR genes](#query-amr-genes)
  - [Query MLST](#query-mlst-multi-locus-sequence-typing)
  - [Find closest genomes (sketch)](#find-closest-genomes-sketch)
  - [Browse and download ATB files from OSF](#browse-and-download-atb-files-from-osf)
  - [Fetch the database](#fetch-the-database)
  - [Configuration](#configuration)
- [LLM Integration (MCP)](#llm-integration-mcp)
- [Output Formats](#output-formats)
- [Available Columns](#available-columns)
- [Performance](#performance)
- [Building](#building)
- [Data Sources](#data-sources)
- [License](#license)

## Download

Pre-built binaries for all platforms:

| Platform | Architecture | File |
|----------|-------------|------|
| Linux | x86_64 (amd64) | `atb-cli_<version>_linux_amd64.tar.gz` |
| Linux | ARM64 | `atb-cli_<version>_linux_arm64.tar.gz` |
| macOS | Intel (amd64) | `atb-cli_<version>_darwin_amd64.tar.gz` |
| macOS | Apple Silicon (arm64) | `atb-cli_<version>_darwin_arm64.tar.gz` |
| Windows | x86_64 (amd64) | `atb-cli_<version>_windows_amd64.zip` |
| Windows | ARM64 | `atb-cli_<version>_windows_arm64.zip` |

**Latest release:** [github.com/allthebacteria/atb-cli/releases/latest](https://github.com/allthebacteria/atb-cli/releases/latest)

Download the file for your platform, extract, and place the `atb` binary (or `atb.exe` on Windows) somewhere in your `PATH`.

## Install

**One-line install** (Linux/macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/allthebacteria/atb-cli/main/install.sh | bash
```

This detects your OS and architecture, downloads the latest release, and installs to `~/.local/bin`. It will add the directory to your `PATH` automatically if needed.

```bash
# Install a specific version
curl -fsSL https://raw.githubusercontent.com/allthebacteria/atb-cli/main/install.sh | ATB_VERSION=v0.1.0 bash

# Install the binary to a custom directory
curl -fsSL https://raw.githubusercontent.com/allthebacteria/atb-cli/main/install.sh | ATB_INSTALL_DIR=/usr/local/bin bash
```

> The env var goes **after** the pipe so it applies to `bash`, not `curl`. Writing `ATB_INSTALL_DIR=/path curl ... | bash` looks plausible but only sets the variable for the `curl` process — the install script run by `bash` never sees it.

> **Heads up — pick a data directory before fetching.** `ATB_INSTALL_DIR` only controls where the `atb` binary lands, not where the database goes. A default `atb fetch` pulls the core parquet tables (~600 MB) **and** builds the per-genus AMR indexes, which expand to **~35 GB** on disk. Add another ~4.2 GB if you run `atb sketch fetch`. By default everything goes to `~/.local/share/atb/data`, which is often on a small home volume. To put it elsewhere, either pass `--data-dir` per command or set it once:
>
> ```bash
> atb config set general.data_dir /path/to/large/volume
> # or, ad hoc:
> atb fetch --data-dir /path/to/large/volume
> ```
>
> If you don't need AMR queries, you can skip that table to save the ~35 GB. `atb fetch` will prompt before downloading `amrfinderplus.parquet`; answer **n** to skip just that table and continue with the rest. To bypass the prompt non-interactively, either pre-select tables or pass `--yes`:
>
> ```bash
> # Skip AMR explicitly (non-interactive)
> atb fetch --tables assembly.parquet,assembly_stats.parquet,checkm2.parquet,sylph.parquet,run.parquet,mlst.parquet
>
> # Or accept the AMR download without prompting
> atb fetch --yes
> ```

**Windows:** Download the `.zip` from the [Download](#download) table above, extract, and add `atb.exe` to your PATH.

**Other methods:**

```bash
# Go install (requires Go 1.23+)
go install github.com/allthebacteria/atb-cli/cmd/atb@latest

# From source
git clone https://github.com/allthebacteria/atb-cli.git
cd atb-cli
make build    # binary at ./bin/atb
```

## Quick Start

```bash
# 1. Download the database (~540 MB core tables)
atb fetch

# 2. Query
atb query --species "Escherichia coli" --hq-only --limit 10
```

If you don't have the parquet files yet:

```bash
# Download core tables from OSF (~540MB)
./bin/atb fetch

# Or download all tables including ENA metadata (~4.3 GB)
./bin/atb fetch --all
```

## Updating

`atb` checks for new versions in the background (once every 24 hours). If a newer release is found, you'll see a notice on every run until you upgrade:

```
  A new version of atb is available: v0.9.0 (current: v0.8.0)

  What's new:
    feat: find closest ATB genomes via sketch distances
    ...

  Release: https://github.com/allthebacteria/atb-cli/releases/tag/v0.9.0

  Run 'atb update' to upgrade.
```

To update:

```bash
# Interactive update (asks for confirmation)
atb update

# Non-interactive (for scripts/CI)
atb update --force
```

The updater downloads the correct binary for your OS/architecture from GitHub Releases and replaces the current binary in place.

## Usage Examples

### Query genomes by species

```bash
# Get 10 high-quality E. coli genomes
atb query --species "Escherichia coli" --hq-only --limit 10

# With quality filters
atb query --species "Escherichia coli" \
  --hq-only \
  --min-completeness 99.5 \
  --max-contamination 0.5 \
  --min-n50 200000 \
  --sort-by N50 --sort-desc \
  --limit 20

# Select specific columns
atb query --species "Escherichia coli" --hq-only --limit 5 \
  --columns sample_accession,sylph_species,N50,Completeness_General,aws_url

# Search by genus
atb query --genus Salmonella --hq-only --limit 20

# Wildcard species search
atb query --species-like "Streptococcus%" --hq-only --limit 10
```

### Filter by geography and platform (requires ENA tables)

```bash
# Salmonella from the UK, Illumina only
atb query --species "Salmonella enterica" \
  --country "United Kingdom" \
  --platform "ILLUMINA" \
  --limit 20

# Genomes collected between 2020-2023
atb query --species "Escherichia coli" \
  --collection-date-from 2020-01-01 \
  --collection-date-to 2023-12-31 \
  --limit 50
```

### Use a TOML filter file (reproducible queries)

```bash
# Create a filter file
cat > my_query.toml <<'EOF'
[filter]
species = "Escherichia coli"
hq_only = true
min_completeness = 99.0
max_contamination = 2.0
min_n50 = 100000

[output]
columns = ["sample_accession", "sylph_species", "N50", "Completeness_General", "aws_url"]
sort_by = "N50"
sort_desc = true
limit = 100
format = "tsv"
output = "ecoli_results.tsv"
EOF

# Run the query
atb query --filter my_query.toml

# CLI flags override TOML values
atb query --filter my_query.toml --limit 10
```

### Get sample details

```bash
atb info SAMD00000355
```

Output:
```
=== Assembly ===
  sample_accession:   SAMD00000355
  sylph_species:      Streptococcus pyogenes
  hq_filter:          PASS
  dataset:            661k
  aws_url:            https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMD00000355.fa.gz

=== Assembly Stats ===
  total_length: 1868526
  N50:          148451

=== CheckM2 Quality ===
  completeness_general:  99.06
  contamination:         0.03

=== MLST ===
  scheme:    ecoli_achtman_4
  ST:        131
  status:    PERFECT
  score:     100
  alleles:   adk(53);fumC(40);gyrB(47);icd(13);mdh(36);purA(28);recA(29)

=== ENA Metadata ===
  country:             Japan:Aichi
  collection_date:     1994
  instrument_platform: ILLUMINA
```

### Download genome assemblies

```bash
# Query, then download
atb query --species "Klebsiella pneumoniae" --hq-only --limit 10 \
  --columns sample_accession,aws_url --format csv -o results.csv
atb download --from results.csv --output-dir ./genomes

# Pipe query directly to download
atb query --species "Escherichia coli" --hq-only --limit 5 \
  --columns sample_accession,aws_url --format csv | \
  atb download --from - --output-dir ./ecoli_genomes

# Preview what would be downloaded
atb download --from results.csv --dry-run

# Download from a URL list
atb download --urls my_urls.txt --output-dir ./genomes --parallel 8

# Download a single file
atb download --url https://allthebacteria-assemblies.s3.eu-west-2.amazonaws.com/SAMD00000355.fa.gz \
  --output-dir ./genomes
```

Genomes are saved to `--output-dir` when given, otherwise to `download.output_dir` (default `~/atb/genomes`). That is a separate location from the metadata directory (`general.data_dir` / `--data-dir` / `$ATB_DATA_DIR`), so the data-dir setting does not change where downloads land.

### Summary statistics

```bash
# Default summary of the full database
atb summarise

# Group by species (top 20)
atb summarise --by sylph_species --top 20

# Summarise a previous query result
atb query --genus Salmonella --hq-only --limit 100 \
  --columns sample_accession,sylph_species,hq_filter,dataset -o salmonella.tsv
atb summarise --from salmonella.tsv

# Pipe query to summarise
atb query --species "Escherichia coli" --hq-only --limit 200 \
  --columns sample_accession,sylph_species,dataset --format csv | \
  atb summarise --from -
```

### Query AMR genes

AMR data comes from [AMRFinderPlus](https://github.com/ncbi/amr) v4.2.5 results run across all ATB genomes. All AMR, stress, and virulence data is in `amrfinderplus.parquet` (~58.5M rows, ~1.18 GB), downloaded automatically by `atb fetch` and partitioned by genus for fast queries.

```bash
# Get all AMR gene hits for E. coli (high-quality genomes only)
atb amr --species "Escherichia coli" --hq-only --limit 100

# Filter by drug class
atb amr --species "Escherichia coli" --hq-only --class "BETA-LACTAM"

# Wildcard gene search (all beta-lactamase genes)
atb amr --species "Escherichia coli" --gene "bla%"

# Compare resistance across multiple species
atb amr --species "Escherichia coli,Klebsiella pneumoniae" --class "BETA-LACTAM"

# Find a gene across ALL genera (no species filter needed)
atb amr --gene "blaCTX-M-15" --limit 100

# Filter by ENA metadata -- country, platform, or collection date
# (requires ena_20250506.parquet: run 'atb fetch --tables ena_20250506.parquet')
# Any ENA filter implies --with-ena, so country/collection_date/instrument_platform
# are appended to the output automatically.
atb amr --species "Escherichia coli" --class "BETA-LACTAM" \
  --country "United Kingdom" --platform ILLUMINA --limit 50
atb amr --species "Salmonella enterica" --gene "blaCTX-M-15" \
  --collection-date-from 2022-01-01

# Add ENA columns to the output without filtering
# (requires ena_20250506.parquet). Without --with-ena the ENA table is not read,
# so default AMR queries stay in the millisecond tier.
atb amr --species "Escherichia coli" --class "BETA-LACTAM" --with-ena --limit 50

# Search by drug class across all genera
atb amr --class "CARBAPENEM" --limit 50

# Filter by detection quality
atb amr --species "Escherichia coli" --min-coverage 95 --min-identity 98

# Query stress response genes
atb amr --species "Escherichia coli" --type stress

# Query virulence factors
atb amr --species "Escherichia coli" --type virulence

# Query all three categories at once
atb amr --species "Escherichia coli" --type all

# Output to file (CSV)
atb amr --species "Klebsiella pneumoniae" --hq-only --format csv -o kpn_amr.csv

# Auto-gzip when -o ends in .gz; format inferred from filename (.csv.gz -> CSV)
atb amr --species "Escherichia coli" --hq-only -o ecoli_amr.tsv.gz

# Restrict to a specific list of samples (comma-separated)
atb amr --samples SAMD00093868,SAMEA104470147 --type all

# Or read accessions from a file (one per line, '#' comments allowed)
atb amr --sample-file my_isolates.txt --class "BETA-LACTAM"

# Download matching assemblies directly
atb amr --species "Escherichia coli" --class "BETA-LACTAM" --hq-only --download -d ./genomes

# Preview what would be downloaded (no actual download)
atb amr --species "Klebsiella pneumoniae" --gene "blaCTX-M-15" --download --dry-run

# Cap number of assemblies to download
atb amr --species "Escherichia coli" --gene "bla%" --download --max-samples 20 -d ./bla_genomes
```

`--species` accepts comma-separated values for multi-species comparison. When omitted, `--gene` or `--class` is required to search across all genera.

The `--download` flag downloads the FASTA assembly for each unique sample in the results. Query output is always printed first. Use `--dry-run` to preview URLs without downloading, and `--max-samples` to cap the number of assemblies.

AMR output preserves the AMRFinderPlus v4.2.5 TSV header verbatim, so all 26 columns are present in the original case and ordering: `Name`, `Protein id`, `Contig id`, `Start`, `Stop`, `Strand`, `Element symbol`, `Element name`, `Scope`, `Type`, `Subtype`, `Class`, `Subclass`, `Method`, `Target length`, `Reference sequence length`, `% Coverage of reference`, `% Identity to reference`, `Alignment length`, `Closest reference accession`, `Closest reference name`, `HMM accession`, `HMM description`, `Hierarchy node`, `genus`, `species`.

With `--with-ena` (or any ENA filter), three extra columns are appended: `country`, `collection_date`, `instrument_platform`.

### Query MLST (Multi-Locus Sequence Typing)

MLST data covers 2.44M samples across 156 typing schemes. The data is included in the core metadata fetch.

```bash
# Get all STs for E. coli (high-quality only)
atb mlst --species "Escherichia coli" --hq-only --limit 20

# Find ST131 E. coli (a globally disseminated high-risk clone)
atb mlst --species "Escherichia coli" --st 131 --hq-only

# Query by MLST scheme name
atb mlst --scheme salmonella --limit 50
atb mlst --scheme ecoli_achtman_4 --limit 20

# Only perfect MLST calls (all alleles matched exactly)
atb mlst --species "Escherichia coli" --status PERFECT --limit 20

# Find novel sequence types (new allele combinations)
atb mlst --species "Salmonella enterica" --status NOVEL --limit 20

# Combine with species and quality filters
atb mlst --species "Klebsiella pneumoniae" --hq-only --status PERFECT --limit 50

# Filter MLST results by ENA metadata -- country, platform, or collection date
# (requires ena_20250506.parquet: run 'atb fetch --tables ena_20250506.parquet')
# Any ENA filter implies --with-ena, so country/collection_date/instrument_platform
# are appended to the output automatically.
atb mlst --species "Escherichia coli" --st 131 --country "United Kingdom"
atb mlst --species "Salmonella enterica" --platform ILLUMINA \
  --collection-date-from 2022-01-01 --limit 100

# Add ENA columns to the output without filtering
# (requires ena_20250506.parquet). Without --with-ena the ENA table is not read,
# so default MLST queries stay in the millisecond tier.
atb mlst --species "Escherichia coli" --st 131 --with-ena

# Output as CSV
atb mlst --species "Escherichia coli" --st 131 --format csv -o st131.csv

# Download assemblies for matching samples
atb mlst --species "Escherichia coli" --st 131 --download -d ./st131

# Preview download, cap at 20 assemblies
atb mlst --species "Salmonella enterica" --status PERFECT --download --dry-run --max-samples 20
```

MLST output columns: `sample_accession`, `sylph_species`, `mlst_scheme`, `mlst_st`, `mlst_status`, `mlst_score`, `mlst_alleles`

With `--with-ena` (or any ENA filter), three extra columns are appended: `country`, `collection_date`, `instrument_platform`.

MLST status values: `PERFECT` (exact match), `NOVEL` (new combination), `OK` (partial), `MIXED`, `BAD`, `MISSING`, `NONE`

### Find closest genomes (sketch)

Find the closest genomes in the ATB database to your input sequences using MinHash sketch distances via [sketchlib](https://github.com/bacpop/sketchlib.rust). Results include ANI (Average Nucleotide Identity) and enriched metadata.

**Linux/macOS only** -- sketchlib binaries are not available for Windows.

```bash
# 1. Install sketchlib (one-time, downloads binary next to atb)
atb sketch install

# 2. Download the ATB sketch database (~4.2 GB, one-time)
atb sketch fetch

# 3. Query your genome against ~3.2M ATB genomes
atb sketch query my_genome.fasta

# Top 50 closest matches
atb sketch query my_genome.fasta --knn 50

# Multiple input files
atb sketch query sample1.fasta sample2.fasta

# Batch from a file list
atb sketch query -f input_list.txt

# Find closest genomes AND download their assemblies
atb sketch query my_genome.fasta --download ./closest_genomes

# Preview downloads without actually downloading
atb sketch query my_genome.fasta --download ./closest --dry-run

# Raw sketchlib output (no metadata enrichment)
atb sketch query my_genome.fasta --raw

# JSON output
atb sketch query my_genome.fasta --format json
```

Output columns: `query`, `sample_accession`, `ani`, `species`, `N50`, `completeness`, `mlst_st`

When `--download` is used, the output directory will contain:
- Downloaded genome FASTA files (`.fa.gz`)
- `sketch_results.tsv` -- full query results with download URLs
- `manifest.json` -- download summary (consistent with `atb download`)

```bash
# Show info about the local sketch database
atb sketch info
```

### Read AGC archives (`atb agc`)

[AGC](https://github.com/refresh-bio/agc) (Assembled Genomes Compressor) is a
compact archive format for large collections of assembled genomes. `atb agc`
reads existing `.agc` archives — listing samples and contigs and extracting
sequences as FASTA — by shelling out to the upstream `agc` binary.

`atb` fetches and caches `agc` next to itself; it is never linked into `atb`, so
the `atb` binary stays statically linked. Prebuilt `agc` binaries exist for
Linux, macOS, and Windows x64 (Windows arm64 is not published upstream — use the
x64 build under emulation or WSL).

```bash
# One-time: download the agc helper binary
atb agc install

# List samples, then one sample's contigs
atb agc ls genomes.agc
atb agc ls genomes.agc SAMD00000344

# Archive metadata
atb agc info genomes.agc

# Extract one contig region to stdout
atb agc get genomes.agc "contig_1@SAMD00000344:1000-2000"

# Extract whole samples to a file
atb agc get genomes.agc --sample SAMD00000344 --sample SAMD00000345 -o out.fa

# Extract the entire collection, gzip level 6, 8 threads
atb agc get genomes.agc --all --gzip 6 -t 8 -o all.fa.gz
```

`atb agc get` accepts three mutually exclusive selections: positional contig
queries (`contig[@sample][:from-to]`), `--sample` (whole samples), or `--all`
(the whole collection). Output streams to stdout unless `-o` is given.

> AllTheBacteria does not currently distribute `.agc` archives; this command
> works on `.agc` files you already have. It is read-only — it never creates or
> modifies archives.

### Browse and download ATB files from OSF

The AllTheBacteria project hosts ~3,000 files on [OSF](https://osf.io/h7wzy/) across 75+ categories (assemblies, annotations, AMR, MLST, protein structures, and more). Browse and download them directly:

```bash
# List all project categories
atb osf ls

# Find files matching a keyword
atb osf ls AMR
atb osf ls "Protein Structures"

# Regex search across project and filename
atb osf ls --grep "bakta.*batch"

# Sort by size, different output formats
atb osf ls AMR --sort size --format json

# Preview what would be downloaded
atb osf download --dry-run "AMRFinderPlus.*results.*latest"

# Download with MD5 verification
atb osf download --verify "DefenseFinder.*results"

# Download all files in a project
atb osf download --project AllTheBacteria/MLST --all -o ./mlst_data
```

Extract assembly tarballs after downloading. The `.tar.xz` batches contain
uncompressed FASTA files; `--extract` unpacks them, `--compress` chooses the
on-disk encoding (`none`, `gz` (default), or `xz`), and `--delete-archive`
removes each tarball once it has extracted successfully:

```bash
# Download all assembly batches and extract FASTAs as .fa.gz, removing tarballs
atb osf download --project AllTheBacteria/Assembly --all \
    --extract --compress gz --delete-archive -o ./assemblies

# One batch, raw FASTA, keep the tarball
atb osf download "Assembly.*batch.*1\." --extract --compress none
```

The file index is cached locally and refreshed every 7 days. Use `--refresh` to force an update.

### Fetch the database

```bash
# Download core tables including AMR and MLST (~1.8 GB)
# Includes: assembly, assembly_stats, checkm2, sylph, run, mlst, amrfinderplus
atb fetch

# Download all tables including ENA metadata (~4.3 GB)
atb fetch --all

# Download specific tables only
atb fetch --tables ena_20250506.parquet

# Force re-download
atb fetch --force

# Rebuild the SQLite query index (runs automatically after fetch)
atb index --force
```

### Configuration

```bash
# Create default config
atb config init

# View config
atb config show

# Set the data directory (metadata index: parquet tables + SQLite)
atb config set general.data_dir /path/to/parquet/files

# Set where downloaded genomes go — a SEPARATE directory from the data dir
atb config set download.output_dir /path/to/genomes

# Set default download parallelism
atb config set download.parallel 8
```

Config is stored at `~/.config/atb/config.toml`.

#### Shared installations (multi-user servers)

On a shared cluster you usually want every user to read from a single large data volume without each person editing their own config. Two env vars give an admin that without touching `$HOME`:

| Variable | Effect |
|----------|--------|
| `ATB_DATA_DIR` (alias: `ATB_DATADIR`) | Overrides `general.data_dir` from the config file. Takes precedence over the config but loses to an explicit `--data-dir` flag. |
| `ATB_CONFIG`  | Path to the config file. Useful for shipping a site-wide config (e.g. `/etc/atb/config.toml`) via `/etc/profile.d`. |

Example: drop a one-liner into `/etc/profile.d/atb.sh` and every login shell picks up the shared paths:

```sh
# /etc/profile.d/atb.sh
export ATB_DATA_DIR=/srv/atb/data
export ATB_CONFIG=/etc/atb/config.toml
```

Resolution order for the data directory is `--data-dir` flag → `ATB_DATA_DIR` env → config file → OS default.

## LLM Integration (MCP)

`atb` includes a built-in [Model Context Protocol](https://modelcontextprotocol.io) server, allowing LLMs to query the AllTheBacteria database directly through natural language.

**Two transport modes:**
- **stdio** (default) - for Claude Code, Claude Desktop, Cursor, VS Code Copilot, Windsurf, OpenAI Codex CLI
- **HTTP/SSE** (`--http :8080`) - for ChatGPT, OpenAI Responses API, remote clients

**Tools exposed:**

| Tool | Description |
|------|-------------|
| `atb_query` | Search genomes by species, genus, quality, N50 |
| `atb_amr` | Query AMR resistance genes by species and drug class |
| `atb_mlst` | Query MLST scheme, ST, and allele calls |
| `atb_info` | Get full metadata for a specific sample |
| `atb_stats` | Database summary statistics |
| `atb_species_list` | List available species with genome counts |

### Quick try (no install needed, requires Go)

```bash
# Claude Code - runs directly from GitHub, available in all projects
claude mcp add --scope user atb -- go run github.com/allthebacteria/atb-cli/cmd/atb@latest mcp
```

First call takes ~10s to compile; cached after that.

### Setup

```bash
# 1. Install atb
curl -fsSL https://raw.githubusercontent.com/allthebacteria/atb-cli/main/install.sh | bash

# 2. Fetch the database and build the index
atb fetch
```

### stdio mode (Claude, Cursor, Codex CLI)

```bash
# Claude Code (available globally in all projects)
claude mcp add --scope user atb -- atb mcp

# Claude Code (current project only)
claude mcp add atb -- atb mcp

# Claude Desktop (add to ~/Library/Application Support/Claude/claude_desktop_config.json on macOS
#                  or %APPDATA%\Claude\claude_desktop_config.json on Windows)
{
  "mcpServers": {
    "atb": {
      "command": "atb",
      "args": ["mcp"]
    }
  }
}

# Cursor (Settings > MCP Servers > Add)
# Command: atb mcp

# OpenAI Codex CLI (~/.codex/config.toml)
[mcp_servers.atb]
command = "atb"
args = ["mcp"]
```

> **Note:** After adding, restart your client for the MCP server to become available.

### HTTP/SSE mode (ChatGPT, OpenAI API, remote clients)

```bash
# Start the HTTP/SSE server
atb mcp --http :8080
```

Then configure your client with the SSE endpoint URL:
- **ChatGPT:** Settings > Connected apps > Add MCP server > `http://your-host:8080/sse`
- **OpenAI Responses API:** Use `server_url: "http://your-host:8080/sse"` in the MCP tool config

For public access, deploy with Docker or use a tunnel:

```bash
# Docker (auto-downloads data on first run)
docker compose up -d
# SSE endpoint: http://localhost:8080/sse

# Or quick public URL with ngrok
atb mcp --http :8080 &
ngrok http 8080
```

See [docs/deployment.md](docs/deployment.md) for full deployment guides (Fly.io, Railway, VPS, Docker Compose).

> **Note:** If your data is in a non-default location, add `--data-dir /your/path` to all commands above.

### What you can ask your LLM

Once connected, you can ask natural language questions like:

- "How many Salmonella genomes are in the database?"
- "Find me 20 high-quality E. coli genomes with N50 > 200000"
- "What beta-lactam resistance genes does Klebsiella pneumoniae have?"
- "Show me all metadata for sample SAMD00000355"
- "What are the top 10 species by genome count?"

The LLM will call the appropriate `atb` tools and interpret the results for you.

## Output Formats

By default, output is a pretty table when writing to a terminal, and TSV when piped. Override with `--format`:

```bash
atb query --species "Escherichia coli" --limit 5 --format tsv     # tab-separated
atb query --species "Escherichia coli" --limit 5 --format csv     # comma-separated
atb query --species "Escherichia coli" --limit 5 --format json    # JSON array
atb query --species "Escherichia coli" --limit 5 --format table   # pretty table
```

When writing to a file with `-o`, the format is inferred from the filename if `--format` is not given (`.csv` -> csv, `.tsv` -> tsv, `.json` -> json). Append `.gz` to any of these and the output is gzipped on the fly:

```bash
atb query --species "Escherichia coli" --hq-only -o results.csv.gz   # gzipped CSV
atb amr   --species "Escherichia coli" --hq-only -o ecoli_amr.tsv.gz # gzipped TSV
```

## Available Columns

### assembly.parquet (always loaded)
`sample_accession`, `run_accession`, `assembly_accession`, `sylph_species`, `scientific_name`, `hq_filter`, `dataset`, `asm_fasta_on_osf`, `aws_url`, `osf_tarball_url`

### assembly_stats.parquet (loaded when N50/length columns requested)
`total_length`, `number`, `mean_length`, `longest`, `shortest`, `N50`, `N90`

### checkm2.parquet (loaded when quality columns requested)
`Completeness_General`, `Contamination`, `Completeness_Specific`, `Genome_Size`, `GC_Content`

### ena_20250506.parquet (loaded when geography/platform columns requested)
`country`, `collection_date`, `instrument_platform`, `instrument_model`, `read_count`, `base_count`, `library_strategy`, `study_accession`, `fastq_ftp`

## Performance

Queries are effectively instant after `atb fetch` builds the indexes.

| Query type | Time | Peak RAM |
|------------|------|----------|
| Metadata query (`atb query --species ... --limit 100`) | <10ms | 15 MB |
| Sample info (`atb info SAMN...`) | <10ms | 14 MB |
| AMR query (`atb amr --species ... --limit 100`) | <10ms | 0.1 MB |
| AMR query (`atb amr --species ... --class ...`) | <10ms | 0.1 MB |
| AMR cross-genus gene search (`atb amr --gene ... --limit 100`) | ~2ms | 3 MB |
| Genome download (50 files, parallel=4) | 3.8s | 18 MB |

Post-fetch index build: ~8-11 minutes (one-time). Disk: ~35 GB for the default install (core parquet + per-genus AMR indexes); skip `amrfinderplus.parquet` via `--tables` if you don't need AMR queries.

Full benchmark details, methodology, and comparisons across query tiers: **[docs/BENCHMARKS.md](docs/BENCHMARKS.md)**

### Notes on species names

The database uses GTDB taxonomy (not NCBI). Some species names differ from common usage. If a query returns 0 results, the tool suggests close matches. Example: *Enterococcus faecium* in GTDB may be *Enterococcus_B faecium*. Use `--species-like "Enterococcus%faecium"` to search across GTDB naming variants.

## Building

```bash
# Build for current platform
make build

# Run tests
make test

# Cross-compile for all supported platforms
GOOS=linux   GOARCH=amd64 go build -o bin/atb-linux-amd64   ./cmd/atb
GOOS=linux   GOARCH=arm64 go build -o bin/atb-linux-arm64   ./cmd/atb
GOOS=darwin  GOARCH=amd64 go build -o bin/atb-darwin-amd64  ./cmd/atb
GOOS=darwin  GOARCH=arm64 go build -o bin/atb-darwin-arm64  ./cmd/atb
GOOS=windows GOARCH=amd64 go build -o bin/atb-windows-amd64.exe ./cmd/atb
GOOS=windows GOARCH=arm64 go build -o bin/atb-windows-arm64.exe ./cmd/atb
```

Requires Go 1.23+. Pure Go, no CGO - cross-compilation works out of the box.

## Data Sources

All external URLs are defined in [`internal/sources/sources.go`](internal/sources/sources.go) -- a single file that documents every URL the tool accesses.

| Data | Source | Used by |
|------|--------|---------|
| **Parquet metadata** (assembly, QC, species, MLST, AMR) | [OSF (h7wzy)](https://osf.io/h7wzy/files/osfstorage) `Aggregated/Latest_2025-05/` | `atb fetch` |
| **ENA metadata** (geography, platform, dates) | Same OSF project, optional tables | `atb fetch --all` |
| **AMR/Stress/Virulence genes** | [AMRFinderPlus](https://github.com/ncbi/amr) v4.2.5 results as `amrfinderplus.parquet` (~58.5M rows, ~1.18 GB) | `atb amr` |
| **OSF file index** | [all_atb_files.tsv](https://osf.io/r6gcp/) (~3,000 files, 75+ categories) | `atb osf ls`, `atb osf download` |
| **Sketch database** | Same OSF project, `atb_sketchlib.aggregated.202408` (.skm + .skd, ~4.2 GB) | `atb sketch fetch` |
| **Genome assemblies** | `allthebacteria-assemblies.s3.eu-west-2.amazonaws.com` | `atb download`, `atb sketch query --download` |
| **sketchlib binary** | [bacpop/sketchlib.rust](https://github.com/bacpop/sketchlib.rust/releases) (Linux/macOS) | `atb sketch install` |

## License

[MIT](LICENSE)
