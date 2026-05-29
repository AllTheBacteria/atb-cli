# Changelog

All notable changes to `atb-cli` are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added

- `atb osf download` can now extract downloaded tar archives with new
  `--extract`, `--compress` (none/gz/xz, default gz), and `--delete-archive`
  flags — including AllTheBacteria `.tar.xz` assembly tarballs (#18).

## [v0.16.1](https://github.com/allthebacteria/atb-cli/releases/tag/v0.16.1) - 2026-05-24

### Fixed

- `--data-dir` help text now reflects the effective default and surfaces the `ATB_DATA_DIR` override. Previously hardcoded to `default $HOME/.atb/data`, the usage string ignored the OS-specific data directory (e.g. `~/Library/Application Support/atb/data` on macOS) and gave no indication that `ATB_DATA_DIR` was honored, leaving shared-install users unable to confirm via `--help` that the env var was wired up (#15). The string is now built at runtime from `config.DefaultDataDir()` and explicitly mentions `$ATB_DATA_DIR`.

## [v0.16.0](https://github.com/allthebacteria/atb-cli/releases/tag/v0.16.0) - 2026-05-20

### Added

- Environment-variable overrides for shared installations. `ATB_DATA_DIR` (alias: `ATB_DATADIR`) overrides the data directory and `ATB_CONFIG` overrides the config file path. Both precede the loaded config but yield to an explicit `--data-dir` flag, so admins on multi-user servers can point every user at a shared data volume via `/etc/profile.d` without writing per-user config files (#12). All CLI subcommands route through a single `resolveDataDir` helper so the precedence is consistent.
- `atb info` now prints `run_accession` as the first field of the ENA Metadata section, surfacing the SRA run accession directly instead of asking users to scrape it out of the `fastq_ftp` URL (#13).

### Fixed

- Release binaries are built with `CGO_ENABLED=0`, producing statically linked executables that run on RHEL 7/8/9/10, Debian 11, and other older-glibc systems. The previous dynamic builds failed at startup with `GLIBC_2.32`/`GLIBC_2.34 not found` (#10). All dependencies are pure Go (including `modernc.org/sqlite`), so nothing is lost from dropping CGO; the Makefile sets the same flag for local builds.
- `atb config show` and `config.Save` no longer emit the BurntSushi/toml default two-space indent. Long URL values like `base_url` were wrapping in narrow terminals and looked like they had unmatched quotes (e.g. a visible line of `AllTheBacteria"`); standard unindented TOML keeps values column-aligned (#11).
- `atb info` no longer pauses ~24s after the MLST section. The ENA join was using `ReadFiltered`, which materialised the entire ~2M-row ENA parquet into memory before applying the sample predicate. All five single-sample lookups in `atb info` (ENA, Assembly, Stats, CheckM2, MLST) now use `ReadStreamFiltered` with `limit=1`, filtering during deserialization and exiting on the first match (#14).

### Docs

- README's `curl ... | bash` install example now places `ATB_INSTALL_DIR=...` after the pipe so it scopes to the install script rather than to curl. Adds a short note explaining why the placement matters (#9).
- New **Shared installations (multi-user servers)** section documenting `ATB_DATA_DIR` and `ATB_CONFIG`.

## [v0.15.2](https://github.com/allthebacteria/atb-cli/releases/tag/v0.15.2) - 2026-05-16

### Fixed

- Summary statistics now exclude unprocessed/rejected samples (`asm_fasta_on_osf == 0`) so totals match the published OSF release (#7). The fix applies to both code paths: `atb summarise` (default, no `--from`) and the MCP-facing `atb_stats` / `atb_species_list` tools backed by `internal/index.QueryStats` and `SpeciesList`. On v0.15.1 production data this drops `Total genomes` by ~460k and removes the spurious `NA` entry from the top-species list. `atb summarise --from <file>` is unchanged — user-supplied rows are summarised as-is.
- `QueryStats.TopSpecies` now retains the `unknown` category (assembled samples sylph could not classify) since it's a real published bucket; `SpeciesList` still drops it because its purpose is to enumerate named species.

## [v0.15.1](https://github.com/allthebacteria/atb-cli/releases/tag/v0.15.1) - 2026-05-05

### Fixed

- `atb summarise` with no arguments now prints the default summary instead of the help screen. The earlier guard treated zero flags + zero positional args as "show help", contradicting the documented `Default summary of the full database` example (#6). Stray positional args (e.g. `atb summarise foo`) now error via `cobra.NoArgs` rather than being silently ignored.

## [v0.15.0](https://github.com/allthebacteria/atb-cli/releases/tag/v0.15.0) - 2026-05-01

### Added

- `atb amr` now accepts `--samples` (comma-separated accessions) and `--sample-file` (one accession per line, `#` comments allowed) for parity with `atb query`. They intersect with `--hq-only` and ENA filters so combined restrictions tighten rather than override the sample set.
- Output writers across `atb amr`, `atb query`, and `atb mlst` auto-gzip when `-o` ends in `.gz`. Format is inferred from the destination filename — `results.csv.gz` produces gzipped CSV regardless of the configured default — and explicit `--format` still wins.

### Fixed

- Per-genus AMR partition and SQLite-index lookup is now case-insensitive. GTDB letter clades (e.g. `Legionella_C`) were previously normalised to `Legionella_c`, missing the on-disk `Legionella_C.parquet` partition; `atb amr --genus Legionella_C` silently fell back to a full monolithic scan. Results were correct but the optimisation was bypassed for any genus name where casing changed after the first character.
- `atb amr --type` help text no longer claims `amr` is the default. The default is unset (matches all element types); the help string now reads `default: all types`.

## [v0.14.2](https://github.com/allthebacteria/atb-cli/releases/tag/v0.14.2) - 2026-04-30

### Added

- `atb fetch` now prompts before downloading `amrfinderplus.parquet`, surfacing that the ~1.2 GB download expands to ~35 GB of per-genus partitions and SQLite indexes during the post-fetch index build. Answering **n** drops only AMR from the target list and continues fetching the other core tables, so users on small home volumes can opt out without aborting the whole download. Non-interactive runs (no TTY) and `--yes` skip the prompt.

### Docs

- README install section now spells out that `ATB_INSTALL_DIR` controls the binary location only, not the data directory, and documents `--data-dir` / `atb config set general.data_dir` for steering the database to a larger volume. The default-disk figure ("~3.5 GB typical install") was stale and is corrected to ~35 GB for the default install (core parquet + AMR indexes).

## [v0.14.1](https://github.com/allthebacteria/atb-cli/releases/tag/v0.14.1) - 2026-04-30

### Performance

- **AMR index build now streams rows from parquet into SQLite** instead of loading the full per-genus partition into memory first. Combined with `journal_mode=OFF` + in-memory temp store during the build (the `.tmp`→final rename keeps crash safety), peak heap during partition→SQLite conversion drops from O(partition size) to O(buffer ≈ 512 rows). Large genera that previously needed multiple GB now run in tens of MB.
- **Per-genus partition writers cap row groups at 100k rows** (`parquet-go` `MaxRowsPerRowGroup`). The previous unlimited default kept all compressed pages of every open writer in memory until close, which was the dominant cost during `atb fetch --tables amrfinderplus.parquet`. Each writer now flushes a row group to disk every 100k rows.

### Fixed

- Stale-data errors from `atb amr` now suggest `atb fetch --tables amrfinderplus.parquet --force --yes` instead of a bare `atb fetch --force`. The previous suggestion re-downloaded every core table (~hundreds of MB to GBs of unrelated assembly/checkm2/mlst data) when only the AMR artifact was stale.

### Added

- `BenchmarkBuildOneIndex` and `BenchmarkBuildOneIndexPeakRSS` in `internal/amr` track allocs/op and peak heap during a 1M-row build, so future memory regressions in the streamed indexer surface in CI bench output.

## [v0.14.0](https://github.com/allthebacteria/atb-cli/releases/tag/v0.14.0) - 2026-04-29

### Changed

- **AMRFinderPlus dataset upgraded from v3.12.8 to v4.2.5** (~25.6M → ~58.5M rows). `atb amr` now reads the new OSF artifact (`amrfinderplus.parquet`, AMRFP v4.2.5) by default.
- **AMR output now exposes all 26 AMRFinderPlus columns** in TSV, CSV, JSON, and table format (previously a curated subset). Column headers in TSV/CSV/table and JSON keys for `atb amr --format json` and the `atb_amr` MCP tool match the AMRFinderPlus v4.2.5 TSV verbatim — including spaces and the leading `%` in `% Coverage of reference` / `% Identity to reference`.

### Added

- Schema guard on every `atb amr` invocation: detects an older v3.12.8 parquet on disk and returns an actionable error pointing to `atb fetch --force` instead of silently zeroing out renamed columns.
- SQLite index error wrapping: stale per-genus indexes (built from a v3 dataset) now surface a clear "stale index, run `atb fetch --force` to rebuild" error rather than the raw `no such column` SQLite message.

### Breaking changes

- The AMR column set, header names, and JSON key names have changed. Pipelines that grep specific columns by name (e.g. `Gene symbol`) must update to the v4.2.5 names (e.g. `Element symbol`). Consumers that select columns by index also need to update — the column count went from a smaller curated set to the full 26.
- On-disk v3.12.8 parquet files and indexes are not forward-compatible with this release.

### Migration notes

- Run `atb fetch --force` to download the v4.2.5 AMR parquet and rebuild genus partitions and SQLite indexes. No metadata re-download is required; only the AMR artifact changed.
- If you skip the refetch, `atb amr` will fail fast with a friendly error rather than returning silently corrupted rows.

## [v0.13.2](https://github.com/allthebacteria/atb-cli/releases/tag/v0.13.2) - 2026-04-29

### Fixed

- `atb amr` invoked with no flags now reaches the unfiltered-scan confirmation prompt instead of short-circuiting to help text. Regression introduced in v0.13.1.

## [v0.13.1](https://github.com/allthebacteria/atb-cli/releases/tag/v0.13.1) - 2026-04-29

### Added

- `--genus` flag on `atb amr` for case-insensitive bacterial genus filtering, independent of `--species`.
- Confirmation prompt on `atb amr` when invoked without any filter, to guard against accidental full-table scans of the AMR dataset.

### Fixed

- `atb amr --species "Genus species"` now applies a true species-level filter on the row's species column; previously the species token was being conflated with the genus partition lookup.
- `install.sh` now points at the `allthebacteria/atb-cli` repository URL (post-migration permanent target). Closes the temporary v0.12.4 redirect that targeted the old repo while releases were being cut on the new one.

## [v0.13.0](https://github.com/allthebacteria/atb-cli/releases/tag/v0.13.0) - 2026-04-22

### Changed

- **Repository moved to [allthebacteria/atb-cli](https://github.com/allthebacteria/atb-cli).** This is the first release under the new home.
- Go module path renamed from `github.com/AMR-genomics-hackathon-2026/atb-cli-claude` to `github.com/allthebacteria/atb-cli`. Users running `go install github.com/AMR-genomics-hackathon-2026/atb-cli-claude/cmd/atb@latest` must switch to `go install github.com/allthebacteria/atb-cli/cmd/atb@latest`.
- Documentation URLs (`README.md`, `docs/deployment.md`) updated to reference the new repository.
- Container image path updated to `ghcr.io/allthebacteria/atb-cli`.

### Migration notes

- Users on `v0.12.3` or `v0.12.4` (the migration bridge) auto-discover this release via `atb update`; no manual reinstall required.
- Users on `v0.12.2` or earlier should first run `atb update` to reach the bridge (`v0.12.4`), then run `atb update` again to reach this release. Alternatively: reinstall via `curl -fsSL https://raw.githubusercontent.com/allthebacteria/atb-cli/main/install.sh | bash`.
- Historical release assets for `v0.1.0`–`v0.12.4` remain downloadable from the archived old repo at `github.com/AMR-genomics-hackathon-2026/atb-cli-claude`.

## [v0.12.4](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.12.4) - 2026-04-22

### Fixed

- `install.sh` now fetches the latest release from this repository during the migration gap. In v0.12.3 the installer was pointed at the new repository before it had any releases, causing `curl | bash` fresh installs to fail with a 404. Installed binaries continue to self-update to the new repository on `atb update`; only the `curl | bash` path was affected.

## [v0.12.3](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.12.3) - 2026-04-22

### Changed

- **Project has moved to [allthebacteria/atb-cli](https://github.com/allthebacteria/atb-cli).** This release redirects the built-in self-update mechanism (`atb update`) and the `curl | bash` installer to the new repository. Running `atb update` from this version onward will discover future releases published on the new repo — no action required, users will upgrade automatically.
- New installations should use the new URL:
  ```
  curl -fsSL https://raw.githubusercontent.com/allthebacteria/atb-cli/main/install.sh | bash
  ```
- Historical releases (v0.1.0–v0.12.2) remain downloadable from this archived repository.

## [v0.12.2](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.12.2) - 2026-04-20

### Fixed
- `atb mlst` now excludes samples that have no MLST scheme assigned (`mlst_scheme` empty or `-`) by default. Previously, default queries returned rows with blank ST and alleles columns. Use `--status NONE` to inspect untyped samples explicitly.

## [v0.12.1](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.12.1) - 2026-04-17

### Fixed
- `atb query`, `atb mlst`, and `atb amr` date filters now accept ISO 8601 interval `collection_date` values (e.g. `2020-01-01/2020-06-30`). Previously ~50k ENA rows using interval notation were silently excluded.

### Changed
- Help text for `--collection-date-from` / `--collection-date-to` clarifies that rows with missing or unparseable dates are excluded from date-filtered results.

## [v0.12.0](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.12.0) - 2026-04-17

### Added
- `--with-ena` flag on `atb mlst` and `atb amr` appends `country`, `collection_date`, and `instrument_platform` columns from the ENA table
- Any ENA filter (`--country`, `--platform`, `--collection-date-from`, `--collection-date-to`) now implies `--with-ena`, so the columns driving the filter are visible in the output

### Changed
- `atb mlst` and `atb amr` no longer scan `ena_20250506.parquet` unless `--with-ena` or an ENA filter is set — default queries stay in the millisecond tier even when the ENA table is installed

## [v0.9.0](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.9.0) - 2026-03-28

### Added
- `atb sketch query` -- find closest ATB genomes via MinHash sketch distances (ANI)
- `atb sketch install` -- auto-download sketchlib binary from GitHub releases (Linux/macOS)
- `atb sketch fetch` -- download ATB sketch database from OSF (~4.2 GB, ~3.2M genomes)
- `atb sketch query --download` -- fetch matched genome assemblies alongside results
- `atb sketch info` -- show sketch database stats (sample count, k-mer sizes, size)
- `--dry-run` flag on sketch query download for previewing
- `--knn` and `--threads` flags for controlling match count and parallelism

### Changed
- Consolidate all external URLs into `internal/sources/sources.go` (single source of truth)
- Format large numbers with thousand separators across all commands (e.g. `1,868,526`)
- Default sketch threads to NumCPU - 1 for faster queries

## [v0.8.2](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.8.2) - 2026-03-28

### Changed
- `atb osf ls` defaults to table format in terminal, TSV when piped
- Show human-readable sizes (KB/MB/GB) instead of raw MB in OSF listings

## [v0.8.1](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.8.1) - 2026-03-28

### Fixed
- Background update check now saves state before process exit (was racing against `main()`)
- Update notice shows on every run when newer version exists (not just once per 24h)

## [v0.8.0](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.8.0) - 2026-03-28

### Added
- `atb osf ls` -- browse ~3,000 ATB files across 75+ OSF project categories
- `atb osf download` -- download files by regex pattern with MD5 verification
- `--grep`, `--project`, `--sort`, `--refresh` flags for flexible file discovery
- OSF file index cached locally with 7-day TTL

### Changed
- Download module: transport-level timeouts (no more killed large transfers)
- `DownloadAllFiles()` accepts `FileTask` structs with filenames and MD5 checksums
- Progress reporting via callback during downloads
- Handle HTTP 416 by clearing stale `.part` files

## [v0.7.3](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.7.3) - 2026-03-27

### Added
- Comprehensive benchmarks in `docs/BENCHMARKS.md` with 3-tier AMR query results

### Fixed
- SQL LIKE pattern escaping for underscores in gene names

## [v0.7.2](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.7.2) - 2026-03-27

### Changed
- Default output format is now TSV (override with `--format table`)
- Update check shows release notes and changelog

## [v0.7.1](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.7.1) - 2026-03-27

### Fixed
- SQLite variable limit error when using `--hq-only` with AMR queries

### Changed
- Show help when subcommands are called without flags

## [v0.7.0](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.7.0) - 2026-03-27

### Added
- Per-genus SQLite indexes for instant AMR queries (<10ms)
- Interactive prompt before building indexes after fetch
- `--yes`/`-y` flag for non-interactive index builds
- Database summary with file sizes after fetch

### Performance
- Index build uses up to 8 workers concurrently

## [v0.6.0](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.6.0) - 2026-03-27

### Added
- Comma-separated species in `--species` flag for multi-species queries
- `--gene`/`--class` queries across all genera without `--species`

### Performance
- Stream-filter parquet rows during deserialization (276-556x faster with `--limit`)
- Genus-partitioned parquet files eliminate 65-96% of I/O per query

## [v0.5.2](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.5.2) - 2026-03-27

### Fixed
- Auto-detect CSV/TSV delimiter from file content instead of file extension

## [v0.5.1](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.5.1) - 2026-03-27

### Performance
- Parallelize parquet reads during index build

### Fixed
- AMR test fixtures missing from repo
- Self-update "text file busy" on running binary

## [v0.5.0](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.5.0) - 2026-03-27

### Added
- `atb mlst` -- query MLST data with scheme/ST/status filters (2.44M records, 156 schemes)
- MLST section in `atb info` output
- `atb_mlst` MCP tool for LLM integration

### Changed
- AMR data unified into single `amrfinderplus.parquet` (81 MB, 25.6M rows)
- `atb fetch` now downloads 7 core tables including AMR and MLST
- All data sourced from OSF

## [v0.4.1](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.4.1) - 2026-03-27

### Fixed
- Self-update: `rm` before `cp` to avoid "Text file busy" error in `/usr/local/bin`

## [v0.4.0](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.4.0) - 2026-03-27

### Added
- `atb mlst` -- query Multi-Locus Sequence Typing data
- Filter by species, sequence type (`--st`), scheme, status
- MLST data in sample info (`atb info`)
- `atb_mlst` MCP tool
- `mlst.parquet` added to core fetch tables

## [v0.3.1](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.3.1) - 2026-03-26

### Added
- HTTP/SSE transport for MCP server (`atb mcp --http :8080`)
- CORS middleware for cross-origin access
- Codex CLI setup instructions
- MIT license

## [v0.3.0](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.3.0) - 2026-03-26

### Added
- SQLite query index (700-4000x faster queries, <10ms vs 7-40s)
- `atb mcp` -- Model Context Protocol server for LLM integration
- 5 MCP tools: `atb_query`, `atb_amr`, `atb_info`, `atb_stats`, `atb_species_list`
- `atb index` command to manually rebuild index
- Transparent fallback to parquet scan when index is absent

### Performance
- Single sample info: <10ms, 14 MB RAM (was 39.5s, 2.2 GB)
- Species query: <10ms, 15 MB RAM (was 7.3s, 2.1 GB)

## [v0.2.2](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.2.2) - 2026-03-26

### Changed
- Practical examples in every command's `--help` output

## [v0.2.1](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.2.1) - 2026-03-26

### Fixed
- Self-update uses sudo when binary is in a privileged directory

## [v0.2.0](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.2.0) - 2026-03-26

### Added
- `atb amr` -- query AMRFinderPlus gene hits by species, drug class, gene symbol
- AMR data fetch from GitHub by genus (hive-partitioned parquet)
- Support for AMR, stress, and virulence element types
- Filters: `--class`, `--gene` (wildcards), `--min-coverage`, `--min-identity`

### Fixed
- Install script: version parsing on macOS/BSD
- Install script: unbound variable in cleanup trap

## [v0.1.0](https://github.com/AMR-genomics-hackathon-2026/atb-cli-claude/releases/tag/v0.1.0) - 2026-03-26

### Added
- Query 3.2M bacterial genomes by species, genus, quality, N50, country, platform
- Filter via CLI flags or reproducible TOML filter files
- Multi-table joins: assembly + checkm2 + assembly_stats + ENA metadata
- Fuzzy species name suggestion (Levenshtein distance)
- Parallel HTTP download with resume and retry
- Disk space checking before downloads
- Summary statistics with group-by dimensions
- Detailed sample info across all tables
- Auto-detect output format (table for terminal, TSV for pipes)
- TSV, CSV, JSON, and table output formats
- TOML configuration with OS-standard paths
- Self-update from GitHub releases with background version check
- One-line install script (`curl | bash`)
