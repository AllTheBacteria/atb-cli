# AGC fetch over OSF test data — what was achieved

- **Status:** Working version delivered (test/development only)
- **Date:** 2026-06-26
- **Branch:** `feat/agc-osf-test` (off `feat/agc-reader` @ `67bbab2`) — **do not merge to main**
- **Companions:** `agc-osf-migration.md` (the ADR), `agc-osf-test-implementation.md` (the impl spec)

> **Update (2026-06-28):** The `atb fetch-genomes` command described here has since
> moved to **`atb agc download`** (see
> [`agc-download-command.md`](agc-download-command.md)). This summary is preserved
> as the original point-in-time record; the command and file names below describe
> the pre-rename CLI.

> **Update (v0.18.1):** What this summary delivered against the single `z7q5y` test
> node has been superseded. atb now defaults to a hosted OSF index TSV over the
> finalized balanced **v202505** collection (crawling its three OSF nodes is the
> fallback), and accession-mode `atb agc download` plus the read-only `atb agc locate`
> are officially released, current in **v0.18.1** (2026-08-04). See
> [`agc-osf-balanced-migration.md`](./agc-osf-balanced-migration.md) for the current
> design and [`agc-osf-accession-search.md`](./agc-osf-accession-search.md) for `locate`.

## Goal (met)

Make `atb fetch-genomes` work **today** against the AGC test archives staged on
OSF node `z7q5y` ("ATB testing", 767 `.agc` batches under `agc_batches/`), while
keeping that test data in a **separate** index file — never folded into the
master `all_atb_files.tsv`, and never merged to `main`. The data is in test
mode; a dedicated, disposable index keeps it isolated.

## What was delivered

| # | Component | File(s) |
|---|-----------|---------|
| 1 | **OSF node crawler** → `*osf.Index`; paginated REST walk of `z7q5y/agc_batches/`; emits a 6-column TSV in the master-index shape so `osf.ParseIndex` round-trips it; species split on the `_global_ordered_` token | `internal/osf/agc_index.go` (+test) |
| 2 | **Archive resolver** — `SelectBySpecies` filters the index by `<Species>_global_ordered_` prefix → `[]ArchiveRef{Name,URL,MD5,SizeMB}` | `internal/agc/agc_osf.go` (+test) |
| 3 | **Whole-batch fetch** — carries the real OSF download URL + md5 per archive; a nil accession slice routes through `agc getcol` (entire batch = all genomes of a species) | `internal/agc/fetch.go` (+test) |
| 4 | **`atb fetch-genomes --species X`** (Mode A) — fetch index → select → download (cache-first, md5-verified) → extract FASTA; plus `--osf-node` and `--agc-index` overrides | `internal/cli/fetch_genomes_cmd.go` (+test) |
| 5 | **`atb agc index`** — crawls the node and writes the separate `atb_agc_files.tsv` you can commit (or let fetch-genomes crawl + cache on demand) | `internal/cli/agc_index_cmd.go` (+test) |
| 6 | **Config + sources** — optional `AGCConfig.OSFNode` override; `z7q5y` / `agc_batches` / `atb_agc_files.tsv` constants, marked test-provisional | `internal/config/config.go`, `internal/sources/sources.go` |
| 7 | **Committed fixture** — the real 767-row z7q5y snapshot (288 species) | `testdata/atb_agc_files.tsv` |

## How it works

```
atb agc index ──crawl z7q5y/agc_batches──► atb_agc_files.tsv   (6-col, master-index shape)
                                                  │
atb fetch-genomes --species X ────────────────────┤
   FetchAGCIndex (cache-first, 7d) ◄───────────────┘
       │ Index.Filter("<Species>_global_ordered_")     ← "searching"
       ▼
   SelectBySpecies → []ArchiveRef{Name, URL, MD5, SizeMB}  ← "downloading the url"
       ▼
   download .agc (cache-first, MD5-verified from the index row)
       ▼
   agc getcol <archive>  (whole batch = all genomes of the species) → FASTA
```

Three design choices made the separate-index constraint cheap:

- **Reusing the master-index TSV shape** means the existing parser and filter
  work verbatim — the separate file cost no new parsing code, only a different
  *source* feeding the same *shape*.
- **Each OSF row already carries `links.download` + `extra.hashes.md5`**, so the
  opaque-GUID problem (ADR's F1) dissolves and downloads are integrity-checked
  for free through the existing cache path.
- **A nil accession slice is the whole-batch sentinel** — it selects `agc getcol`
  (entire archive) instead of per-accession `getset`, which is exactly the
  by-species semantics with no separate code path.

## Validation (live against z7q5y)

- **Crawl:** 767 `.agc` batches / 288 species; the first GUID matches the ADR
  byte-for-byte.
- **Round-trip on *Mycoplasmoides pneumoniae*** (0.98 MB, 424 samples): FASTA
  record count **15 509 == per-sample `agc listctg` total across all 424
  samples**; the cached archive's md5 matches the index value.
- **Multi-batch path** exercised on *Salmonella* (136 batches).
- Built test-first (TDD); `go build ./... && go vet ./... && go test ./...` all
  green.

## Current state

| | |
|---|---|
| Branch | `feat/agc-osf-test` (off `feat/agc-reader` @ `67bbab2`) |
| Commits | `60c2ebd` docs · `6a8b5dd` feat · `6c84b39` test data (+2691 / −96, 22 files) |
| `main` | untouched at `8f3aa4a` — **not merged** |
| Remote | **not pushed** |

## Caveats

- The CLAUDE.md-mandated `gitnexus_detect_changes()` could not run — the GitNexus
  MCP server was degraded for the session (`FTS extension unavailable`). The gate
  was substituted with the full `build + vet + test` suite plus a manual
  `git diff` review.

## Out of scope / deferred

- **By-accession (Mode B)** needs the upstream accession→archive map (ADR's R1)
  and is deferred for test mode; the existing `ResolveArchives` path is left
  intact and reachable.
- **Promoting `z7q5y`** to a production node default and **registering AGC rows
  into the master index** wait until the data leaves test mode.
- A full project `FOR-DEVELOPERS.md` is deferred until the integrate decision, so
  it does not pollute a possibly-discarded WIP branch.
