# Implementation spec: AGC fetch over OSF test data (separate index)

- **Status:** Accepted (implementation) — test/development only
- **Date:** 2026-06-26
- **Branch:** `feat/agc-osf-test` (off `feat/agc-reader`) — **do not merge to main**
- **Builds on:** `docs/design/agc-osf-migration.md` (the OSF-migration ADR)

## Goal

Make `atb fetch-genomes` work **today** against the AGC test archives staged on
OSF node `z7q5y` ("ATB testing", 767 `.agc` batches under `agc_batches/`),
without waiting on any of the ADR's four upstream data-publishing actions.

The enabling decision (per the user): keep the AGC archives in a **separate
index TSV**, generated from OSF, rather than registering them into the master
`all_atb_files.tsv`. The data is in test mode; a dedicated file keeps it
isolated and disposable.

## Key facts (verified live against z7q5y)

- The OSF API lists every batch with the data we need:
  `GET https://api.osf.io/v2/nodes/z7q5y/files/osfstorage/<agc_batches-id>/`
  → per item: `attributes.name` (`<Species>_global_ordered_<NNNN>.agc`),
  `attributes.size` (bytes), `attributes.extra.hashes.md5`,
  `links.download` (`https://osf.io/download/<guid>/`). Paginated via
  `links.next` (≤100/page → 8 pages for 767 files).
- This solves **F1** (opaque GUIDs) by carrying the real URL per row, and makes
  the AGC index **the same 6-column shape** as the master index, so
  `osf.ParseIndex` / `osf.Index.Filter` are reused verbatim.
- Species split is on the literal token `_global_ordered_` (handles GTDB
  letter-suffixes like `Streptococcus_suis_AA`, `Pseudomonas_E_asiatica`, and
  the special `subthreshold_remainder` batch).

## Architecture (separate-TSV, OSF-native)

```
atb agc index  ──crawl OSF node──►  atb_agc_files.tsv   (6-col, master-index format)
                                         │
atb fetch-genomes --species X ───────────┤
   FetchAGCIndex (cache-first, 7d) ◄──────┘   (crawl node, or read --agc-index file/URL)
        │ osf.Index.Filter(species prefix)        ← "searching"
        ▼
   SelectBySpecies → []ArchiveRef{Name,URL,MD5,SizeMB}   ← "downloading the url"
        ▼
   download .agc (cache-first, MD5-verified via index)   ← F1 + integrity
        ▼
   agc getcol <archive>  (whole batch = all genomes of the species) → FASTA
```

By-accession (Mode B) still needs the upstream R1 map and is **out of scope for
test mode**; the existing `ResolveArchives` path is left intact and reachable.

## Components

1. **`internal/osf/agc_index.go` (new).** OSF node crawler → `*osf.Index`.
   - `parseAGCNodePage(r) (entries []Entry, next string, err error)` — pure,
     unit-tested against a captured API fixture.
   - `CrawlAGCNode(client, nodeID, folder) (*Index, error)` — follow `next`.
   - `WriteAGCIndexTSV(idx, w)` — emit the 6-col TSV (round-trips `ParseIndex`).
   - `FetchAGCIndex(cacheDir, nodeID string, force bool) (*Index, error)` —
     cache-first under `<cacheDir>/atb_agc_files.tsv`, mirrors `FetchIndex`.
   - `SpeciesFromArchive(name) string` — split on `_global_ordered_`.

2. **`internal/agc/agc_osf.go` (new).** Index → archive resolution.
   - `type ArchiveRef struct { Name, URL, MD5 string; SizeMB float64 }`
   - `RefsFromIndex(idx) map[string]ArchiveRef` — key by `TrimSuffix(.agc)`.
   - `SelectBySpecies(idx, species) []ArchiveRef` — normalize space→underscore,
     match `<species>_global_ordered_` prefix.

3. **`internal/agc/fetch.go` (edit).** Carry real URL+MD5; whole-batch extract.
   - `FetchSpec` gains `Refs map[string]ArchiveRef`. `downloadArchives` uses
     `Refs[archive].URL` + `.MD5` when present (OSF), else falls back to
     `archiveURL` (legacy CESGO) — backward compatible.
   - A `nil`/empty accession slice for an archive means **whole collection**:
     route to `agc getcol` (`extractWholeArchive`), output `<archive>.fa[.gz]`
     per-archive or into the combined stream.

4. **`internal/cli/fetch_genomes_cmd.go` (edit).** Add `--species`,
   `--osf-node` (default `z7q5y`, test-only), `--agc-index` (path/URL override).
   `--species` ⇒ Mode A: fetch index → select → groups with nil accessions →
   `FetchGenomes`. `--dry-run` lists matched batches (= "search").

5. **`internal/cli/agc_index_cmd.go` (new).** `atb agc index [--osf-node] [-o file]`
   crawls the node and writes the separate TSV (reproducible artifact).

6. **`internal/sources/sources.go` (edit).** `AGCTestNodeID="z7q5y"`,
   `AGCBatchesFolder="agc_batches"`, `AGCIndexFilename="atb_agc_files.tsv"`,
   OSF API base. Clearly marked **test/provisional**.

7. **`internal/config/config.go` (edit).** Optional `AGCConfig.OSFNode`
   override (empty ⇒ default test node). A speculative `IndexURL` field was
   considered and dropped: the per-archive download URLs already live in the
   TSV rows, so no URL-to-fetch-the-TSV is needed.

## Testing

- `osf/agc_index_test.go`: `parseAGCNodePage` on captured fixtures (name/url/
  md5/size + `next`); `CrawlAGCNode` over a 2-page `httptest` server;
  `WriteAGCIndexTSV`→`ParseIndex` round-trip; `SpeciesFromArchive` edge cases.
- `agc/agc_osf_test.go`: `RefsFromIndex` hit/miss; `SelectBySpecies`
  normalization + cross-species exclusion + `subthreshold_remainder`.
- `agc/fetch_test.go`: ref-based URL/MD5 download (local file server);
  whole-archive routing; **naming-consistency** (every ref present in index).
- CLI: `--species --dry-run` lists expected batches from a committed TSV.

## End-to-end validation (definition of "working")

1. `go build` → `atb`; `atb agc install` (agc v3.2.3).
2. `atb agc index -o atb_agc_files.tsv` → real 767-row TSV from z7q5y.
3. `atb fetch-genomes --species "Mycoplasmoides pneumoniae" --combine -o out.fa`
   (smallest batch, 0.98 MB, 424 samples) → download, MD5-verify, `agc getcol`
   → FASTA.
4. Confirm the FASTA record count equals the archive's total contigs (verified
   2026-06-26: 15 509 records == per-sample `agc listctg` total across 424
   samples), and the cached archive's MD5 matches the index value.

## Out of scope / deferred

By-accession Mode B (needs upstream R1 map); promoting z7q5y → production node
defaults; SQLite resolver fast-path. All per the ADR.
