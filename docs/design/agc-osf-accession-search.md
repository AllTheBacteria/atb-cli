# Design: AGC accession → batch search and selective extraction over the full OSF collection

- Date: 2026-07-10 (reconciled with as-built code 2026-07-11)
- Status: **Implemented and released** - `atb agc locate` and accession-mode
  `atb agc download` shipped through the beta line and are now in the current
  **v0.18.1** release (2026-08-04). The §5 component signatures were reconciled with
  the code as merged at beta.3; §3 "Current state" is the pre-change baseline the
  design started from, kept for context.
- Scope: `internal/sources`, `internal/osf`, `internal/agc`, `internal/cli`, docs

> **Update (v0.18.1):** Since this design was reconciled at beta.3, the collection was
> finalized by the balanced-v202505 migration: the shipped collection nodes are
> `4jq8u`/`jmeqg`/`kzcnr` (not the `6g8by`/`9fqeh`/`xrzub` preview nodes named in §4),
> the folder is `agc_batches` (the `AGCArchivesFolder` constant now holds `agc_batches`,
> not `agc_archives`), and the default index is a single hosted OSF TSV rather than a
> live three-node crawl (which is now the fallback). The §4-§5 node IDs, folder names,
> and loader details are the beta.3 as-built record; see
> [`agc-osf-balanced-migration.md`](./agc-osf-balanced-migration.md) for the current
> design.

## 1. Motivation

Upstream has finished uploading the full **ATB v202505** AGC collection (2.76M genomes,
~103 GB) to OSF, split across three public nodes, and has published a single
authoritative accession → batch list. Two capabilities follow directly:

1. **Update the indexes** to point at the new full collection instead of the
   `z7q5y` test batches and the CESGO prototype map.
2. **Search accession → batch** and **extract only the samples of interest** from
   the collection, without downloading whole species' worth of archives.

The download-then-extract half already exists (`atb agc download <accession...>`),
but it resolves URLs against the *name-addressable* CESGO prototype. The new data
is OSF-only, where archive URLs are opaque per-file GUIDs. Bridging accession-mode
onto OSF is the core new work; the search command and the index update are
extensions of patterns already in the codebase.

## 2. New upstream data (observed 2026-07-10)

| Artifact | Location | Shape |
|---|---|---|
| Major-species archives | OSF node `6g8by`, folder `agc_archives/` | `<Species>_global_ordered_<NNNN>.agc`, each an opaque `/download/<guid>/` URL + md5 + size |
| Unknown-species archives | OSF node `9fqeh`, folder `agc_archives/` | same; **still uploading** at time of writing |
| Former-dustbin archives | OSF node `xrzub`, folder `agc_archives/` | same |
| Accession → batch list | OSF node `z7q5y`, file `atb202505_files_list.txt.zip` (`osf.io/download/2xjt8/`) | ZIP of a 148 MB text file, **2,763,297 lines**, two columns space-delimited: `<accession> <batch_name_without_.agc>` |

Observed stats from the list: **1,733 distinct batches**, **771 species prefixes**,
49–4,976 accessions per batch (median ~499). Accession prefixes are all BioSample
(`SAMN`, `SAMEA`, `SAMD`). The batch naming is the same `_global_ordered_`
convention the code already parses (`osf.SpeciesFromArchive`).

### Key mismatches with current code

1. The map is **ZIP-compressed**; `resolve.FetchMap` currently expects plain text.
2. Archives live under **`agc_archives/`**, but `sources.AGCBatchesFolder` is
   hard-coded to `agc_batches`.
3. The batch index must now span **three nodes**, not one.
4. Archives are **not name-addressable** on OSF, so accession-mode cannot build
   URLs from names.

## 3. Current state (what we build on)

- **Batch index** (`osf.Index` of `osf.Entry{Project, ProjectID, Filename, URL, MD5, SizeMB}`),
  a 6-column TSV. Built by crawling one OSF node's `agc_batches/` folder
  (`osf.FetchAGCIndex`/`CrawlAGCIndex`) or downloading a pre-built TSV
  (`osf.FetchAGCIndexFromURL`, `sources.AGCIndexURL`). URL-aware, 7-day cache.
- **Accession → archive map** (`agc.ArchiveMap map[string]string`), fetched and
  cached by `agc.FetchMap` from `sources.AGCArchiveMapURL`, parsed by
  `agc.ParseMap`, joined by `agc.ResolveArchives` into `groups` (archive → accessions)
  plus `unresolved`.
- **Download command** `atb agc download`, two modes:
  - **Mode A `--species`**: `SelectBySpecies` → `WholeArchiveGroups` → `spec.Refs`
    (URLs from index) → `agc getcol` (whole batch).
  - **Mode B accession**: `ResolveArchives` → `spec.BaseURL` (`sources.ArchiveURL(name)`,
    CESGO) → `agc getset` (per sample) → `<accession>.fa[.gz]`.
- Read-only local subcommands `atb agc ls|info|get`; index builder `atb agc index`.

## 4. Design overview

The unifying idea: **both download modes resolve archive URLs the same way species-mode
already does — from the batch index**. Accession-mode gains an index join (the
"bridge") and stops depending on name-addressable hosting. The accession → batch
map answers *which* batch; the batch index answers *where* that batch lives.

```
accession ──(map: atb202505_files_list)──▶ batch name ──(index: 3-node crawl)──▶ OSF URL+md5
                                                   │
                                                   ├─▶ atb agc locate  (print, no download)
                                                   └─▶ atb agc download (fetch batch, agc getset → per-sample FASTA)
```

## 5. Components

### 5.1 Data sources (`internal/sources/sources.go`)

- Add the three collection nodes with part labels, e.g.:
  ```go
  type AGCNode struct{ ID, Part string }
  var AGCCollectionNodes = []AGCNode{
      {"6g8by", "major"}, {"9fqeh", "unknown"}, {"xrzub", "dustbin"},
  }
  ```
- Add `AGCArchivesFolder = "agc_archives"` (new folder name). Keep
  `AGCBatchesFolder = "agc_batches"` for the legacy `z7q5y` test node so existing
  tests/behaviour are undisturbed.
- Repoint `AGCArchiveMapURL` to `https://osf.io/download/2xjt8/` (a ZIP) and update
  `AGCArchiveMapFilename` to the zipped artifact name (`agc_file_list.txt.zip`), so
  the cache filename matches what is stored on disk (see 5.3).
- Flip the **default** batch index to the **3-node collection crawl** over
  `agc_archives/` (no combined hosted TSV exists upstream yet). Keep `AGCIndexURL`
  and the single-node `useHostedAGCIndex` fast path reachable through the hidden
  `--osf-node z7q5y` override, so the legacy test node and its hosted-TSV tests
  keep working and a future combined collection TSV can be dropped in without a
  code change. Provide a `PartForNode(nodeID)` helper for `locate` output.
- Every URL/node stays overridable via config (existing `AGCConfig`), preserving
  the established preview posture.

### 5.2 Batch index across three nodes (`internal/osf/agc_index.go`)

- Add a multi-node crawl that, for each `AGCCollectionNodes` entry, resolves the
  `agc_archives/` folder (`findFolderURL`) and crawls it (`CrawlAGCNode`),
  stamping `ProjectID` with the node ID. Concatenate into one `osf.Index`.
  As built: `CrawlAGCCollection(rootURLFor func(nodeID string) string, nodes []sources.AGCNode) (*Index, error)` -
  the `rootURLFor` closure (`sources.OSFNodeFilesURL`) is injected so the crawl is testable.
- A dedicated `FetchAGCCollection(cacheDir, rootURLFor, nodes, force)` drives the
  multi-node crawl and caches the combined TSV under
  `<data-dir>/agc/atb_agc_files.tsv` with the existing `.source` marker
  (source key = a stable digest of the node set, so changing the node list
  invalidates the cache). The single-node `FetchAGCIndex` stays for `--osf-node`.
- Folder is chosen **per node**: collection nodes crawl `AGCArchivesFolder`
  (`agc_archives`); the legacy `z7q5y` node keeps `AGCBatchesFolder`
  (`agc_batches`) when reached via `--osf-node`.
- As built, `loadAGCBatchIndex` in `internal/cli` is the default dispatcher (no
  `--osf-node`, no `--agc-index`): it returns the combined `FetchAGCCollection`
  crawl, and delegates to the single-node `loadAGCIndex`/`useHostedAGCIndex` path
  for an explicit `--osf-node`. **Both** download modes call this one loader, so
  `--species` and accession-mode see the same collection.
- No `Entry` schema change: `ProjectID` already carries the node; `part` is derived
  via `sources.PartForNode`. `SpeciesFromArchive` is unchanged.
- Graceful partial availability: a node that is still uploading simply contributes
  fewer rows; the crawl of an empty/partial `agc_archives/` is not an error.

### 5.3 Accession → batch map with ZIP handling (`internal/agc/resolve.go`)

- `FetchMap` downloads the artifact and caches it **as downloaded** (the ~11 MB
  zip) rather than the 148 MB expansion. Cache filename keyed to the artifact
  (e.g. `agc_file_list.txt.zip`), 7-day + `--refresh` semantics unchanged.
- Introduce a reader that transparently decompresses: if the cached bytes start
  with the ZIP magic `PK` (a 2-byte sniff, so a valid-but-empty `PK\x05\x06`
  archive still routes to the zip reader and surfaces as an error instead of a
  silently empty map), open the single entry via `archive/zip` and stream it;
  otherwise read as plain text. The map's first column is always an accession,
  never `PK`, so plain text stays unambiguous. This keeps `ParseMap` untouched and
  stays robust if upstream later publishes the list uncompressed.
- `ParseMap` and `ResolveArchives` are unchanged in behaviour (two-column,
  whitespace-delimited, exact accession lookup, order-preserving groups).
- Scale note: `ParseMap` loads the full map into a `map[string]string`
  (~2.76M entries). This matches the existing resolve.go pattern and the prior
  CESGO map's scale. If memory pressure appears, the escape hatch is an on-disk
  indexed store mirroring `internal/index/builder.go`; out of scope now (YAGNI).

### 5.4 The bridge (accession-mode URLs from the index)

- Add a join helper (in `internal/agc`) that takes `groups map[string][]string`
  (archive → accessions, from `ResolveArchives`) and the batch index refs
  (`RefsFromIndex`), and returns:
  - `refs map[string]agc.ArchiveRef` for the archives present in the index
    (feeds `FetchSpec.Refs`, exactly like Mode A), and
  - a list of archives **absent** from the index (batch known from the map but not
    yet crawlable, e.g. still-uploading node), reported as a distinct
    "batch not yet available" bucket separate from "accession unknown".
- Suggested signature:
  `func RefsForGroups(groups map[string][]string, byName map[string]ArchiveRef) (refs map[string]ArchiveRef, missing []string)`.

### 5.5 `atb agc download` accession-mode over OSF (`internal/cli/agc_download_cmd.go`)

- Mode B now loads the batch index (as Mode A does), runs `ResolveArchives`, then
  `RefsForGroups` to populate `spec.Refs` (drop reliance on
  `spec.BaseURL`/`sources.ArchiveURL` for the collection; keep base-URL support as
  an override path for name-addressable mirrors).
- Extraction is unchanged: per-sample `agc getset` → `<output-dir>/<accession>.fa[.gz]`,
  `--combine` and `--dry-run` as today.
- Reporting: unresolved accessions and "batch not yet available" archives are
  surfaced; the run fails unless `--keep-going` (default true), matching current
  behaviour.

### 5.6 New `atb agc locate` subcommand (`internal/cli/agc_locate_cmd.go`)

Read-only lookup, mirroring `ls`/`info` (small focused command) and `download`'s
input conventions. **Does not require the agc binary** and never downloads.

- Args: `<accession...>`; flags: `--from <file>` (CSV/TSV with a
  `sample_accession` column or one-per-line; `-` for stdin), `--refresh`
  (re-fetch map/index), `--format` (`tsv` default; `json` for pipelines).
- Behaviour: fetch/cache the map + batch index, `ResolveArchives`, join for
  part/URL, print one row per accession.
- Default TSV columns: `accession`, `batch`, `part`. `json` additionally includes
  the resolved OSF `url`. Unresolved accessions print `batch = <unresolved>`;
  known-but-unavailable batches print `part = <not-yet-available>`.
- As built, registered via `newAGCLocateCmd()` in the `agc` command group
  (`agc_cmd.go`); documented alongside `download` (the search half of the same
  workflow).

### 5.7 Extraction

No new extraction code. `atb agc download <accession...>` already extracts only the
requested samples via `agc getset`. It inherits OSF support from the bridge (5.4/5.5).

## 6. Data flow (end to end)

**`atb agc locate SAMEA2247573 --from ids.txt`**
1. Gather accessions (args + `--from`/stdin), dedupe.
2. `FetchMap` (zip-aware) → `ParseMap` → `ArchiveMap`.
3. `FetchAGCCollection` (3-node crawl, cached) → `RefsFromIndex`.
4. `ResolveArchives` → groups + unresolved; join with refs for part/URL.
5. Print `accession, batch, part[, url]`.

**`atb agc download SAMEA2247573 -o ./out`**
1–4 as above (steps 2–4 identical: same resolver, same index).
5. `RefsForGroups` → `spec.Refs`; `FetchGenomes` downloads only the needed batch
   archives (cache-first, md5-verified) and runs `agc getset` per accession.
6. Write `./out/SAMEA2247573.fa[.gz]`; report unresolved / not-yet-available.

## 7. Error handling

- **ZIP**: corrupt/empty archive → clear error naming the cached file; non-zip
  bytes fall back to plain-text parsing (forward-compatible).
- **Accession not in map** → `unresolved` bucket; warn; fail unless `--keep-going`.
- **Batch in map but not in index** (still uploading / node not yet crawled) →
  distinct "not yet available" bucket, so users can tell "wrong accession" from
  "come back later".
- **OSF crawl / network** → existing wrapped errors; partial node contributes what
  it has rather than failing the whole index.
- **agc binary missing** → only relevant to `download`/extraction
  (`agc.FindBinary`); `locate` works without it.

## 8. Config and churn posture

- Repoint defaults to the new collection; keep every URL/node config-overridable
  (`agc.archive_map_url`, `agc.archive_base_url`, node overrides). Label the
  collection **preview** in docs, consistent with the prior `z7q5y` posture.
- **Repeatable refresh**: `atb agc index --refresh` re-crawls the three nodes;
  `locate`/`download --refresh` re-fetch the map. When upstream re-cuts the
  100 MB-balanced batches and renames them, this is a **data refresh**, not a code
  change, provided the batch naming keeps a parseable species prefix.
- **Coupling invariant**: the map's batch-name column must equal the index's
  archive stem (`Filename` minus `.agc`). The map and the index are regenerated
  **together** on any re-batch/rename. Documented as a watch item; if the new ATB
  naming abandons the `_global_ordered_` convention, `osf.SpeciesFromArchive` needs
  updating (flagged, not handled now).

## 9. Testing (mirroring existing suites)

- **Multi-node crawl**: `httptest` servers returning OSF JSON pages for
  `agc_archives/` across ≥2 fake nodes; assert combined index, `ProjectID`
  stamping, part derivation, and partial-node tolerance (extends
  `agc_osf_test.go`).
- **ZIP map**: small zipped fixture; assert `FetchMap` caches and the reader
  decompresses; plain-text fallback path; `ParseMap` unchanged.
- **Bridge**: `RefsForGroups` — archive present → ref; archive absent → `missing`.
- **`locate` command**: table tests over a fixture map + index; TSV and JSON output;
  `--from` file and stdin; unresolved and not-yet-available rows.
- **Download Mode B over OSF**: `spec.Refs` populated from index (not base URL);
  unresolved/not-yet-available reporting; `--dry-run` lists batches without
  fetching.
- Keep existing CESGO/base-URL tests green via the override path.

## 10. Rollout

1. Land code + tests behind config-overridable defaults.
2. Regenerate CLI reference (`make docs`); update the AGC guide with `locate` and the
   new collection, labelled preview (hand-written guides drift, so update
   explicitly).
3. Ship as a pre-release consistent with prior AGC beta posture.

## 11. Non-goals / future

- Publishing a single combined hosted index TSV for the collection (would let
  `FetchAGCIndexFromURL` replace the multi-node crawl) — future, once upstream or
  we publish it.
- On-disk/SQLite accession→batch store for very low-memory environments — only if
  the in-RAM map becomes a problem.
- Handling a future non-`global_ordered` ATB naming scheme — revisit when the
  100 MB-balanced batches are renamed.
