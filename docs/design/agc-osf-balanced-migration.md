# ADR: migrate AGC to the finalized balanced v202505 collection and graduate to stable

- **Status:** Accepted (design). Supersedes the preview/test posture recorded in
  [`agc-osf-migration.md`](./agc-osf-migration.md),
  [`agc-osf-test-summary.md`](./agc-osf-test-summary.md), and the "PREVIEW" default
  in [`agc-osf-accession-search.md`](./agc-osf-accession-search.md).
- **Date:** 2026-08-01
- **Scope:** `internal/sources/sources.go`, `internal/osf/agc_index.go`,
  `internal/agc/resolve.go`, `internal/agc/locate.go`,
  `internal/cli/agc_index_cmd.go`, `internal/cli/agc_locate_cmd.go`,
  `internal/cli/agc_download_cmd.go`, `testdata/`, `docs/`, release tooling.
- **Decisions locked in this review:**
  1. Migrate atb from the preview/test nodes to the collaborator's finalized
     "balanced" collection, and graduate the AGC feature out of preview by cutting
     **v0.18.0 final**.
  2. Obtain the batch index as a single **hosted combined TSV** published to OSF.
     The crawl-and-join runs once at publish time, not in every user's atb.
  3. Retire the now-dead legacy paths (the `z7q5y` test-node crawl and the CESGO
     archive host) as part of graduating.

---

## 1. Context: what the collaborator shipped (verified against OSF)

The finalized balanced ATB **v202505** AGC collection is published and its figures
were confirmed by reading the live OSF API and file headers on 2026-08-01.

| Fact | Verified value |
|------|----------------|
| Hosting nodes | `4jq8u`, `jmeqg`, `kzcnr`, each with an **`agc_batches/`** folder |
| Batches | 1,268, named `atb.assembly.202505_all.batch.NNNN.agc` |
| Genomes | 2,763,297 |
| Total size | 102.4 GB; per-batch 0.98 MB to 127.94 MB |
| Distinct species | 1,016 |
| Assembly list (OSF `gtqrx`) | `assemblies_filelist.txt.gz`, 10.3 MB gzip, 2,763,297 rows of `accession <sep> batch_name` (batch name has no `.agc`) |
| Batch metadata (OSF `8y9r2`) | `batches_202505_metadata.tsv.gz`, 20 KB gzip, TSV columns `batch_name`, `old_name`, `size_bytes`, `nb_genomes` |

Both metadata files live in the `metadata_balanced_batches/` folder on OSF node
`z7q5y` (confirmed 2026-08-01: `gtqrx` md5 `25ca33a06fc9dee14ea2b1d19e29c8b2`,
`8y9r2` md5 `d8ef1ee3d805253954f3b9ff9f0f2a57`). That folder is where the combined
index (section 3.2) should be uploaded, alongside the two source files.

### 1.1 The defining change: batches are renamed and species-agnostic

The previous data encoded the species in the batch filename
(`Salmonella_enterica_global_ordered_0072.agc`), which
`osf.SpeciesFromArchive` parsed by splitting on `_global_ordered_`. The finalized
batches are numbered and carry no species in the filename
(`atb.assembly.202505_all.batch.0021.agc`).

The species survives only in the batch-metadata `old_name` column, in two forms:

- Major species: `<Species>_global_ordered.partNNN.agc`
  (for example `Acinetobacter_baumannii_global_ordered.part003.agc`).
- Remainder tier: `unknown.partNNN.agc` (the old `subthreshold_remainder`).

Consequence: species-to-batch is no longer derivable from a batch filename. It is
recoverable only by joining the batch metadata. The old accession-to-batch map
(`agc_file_list.txt.zip`, OSF `2xjt8`) and the hosted index TSV (OSF
`6a477a94...`) both use the retired names and are superseded.

### 1.2 The two published files are complementary, not complete on their own

- Batch metadata (`8y9r2`) has `batch_name`, `old_name` (so, species), and
  `nb_genomes`, but **no URL, no md5, no node**.
- The OSF node listing has the URL, md5, and size, but only the numbered
  `batch_name`; it has no species and no genome count.

A complete batch index is the **join** of the two on `batch_name`. This ADR
performs that join once, at publish time.

---

## 2. Gap: what the finalized data breaks in atb today

atb on `main` (v0.18.0-beta.3) points entirely at preview/test data:

1. `sources.AGCCollectionNodes` is `6g8by`/`9fqeh`/`xrzub`; the finalized nodes are
   `4jq8u`/`jmeqg`/`kzcnr`.
2. `sources.AGCArchivesFolder` is `agc_archives`; the finalized nodes use
   `agc_batches`.
3. `osf.SpeciesFromArchive` derives species from the batch filename, which the
   renamed batches no longer carry.
4. `sources.AGCArchiveMapURL` points at the retired zipped map with old names;
   the finalized accession-to-batch map is `assemblies_filelist.txt.gz` (gzip, new
   names).
5. `sources.AGCIndexURL` points at the retired hosted TSV built over `z7q5y`.
6. `sources.PartForNode` labels a node major/unknown/dustbin; in the finalized
   layout batches of every tier are spread across all three nodes, so the node no
   longer implies a tier.

The old preview nodes are still populated, so the shipped beta is not broken today,
but it serves data the collaborator has superseded.

---

## 3. Decision

### 3.1 Runtime model (no crawl in a user's atb)

A user's atb downloads two finished artifacts and joins nothing at runtime:

- The **hosted combined batch index** (`sources.AGCIndexURL`), about 1,268 rows in
  the existing 6-column index TSV. Used by `atb agc download --species` (filter on
  the species column) and to resolve a batch name to its URL and md5.
- The **accession-to-batch map** `assemblies_filelist.txt.gz`
  (`sources.AGCArchiveMapURL`). Used by by-accession `atb agc download` and by
  `atb agc locate`.

`--species` reads only the small batch index and never downloads the large map.
By-accession and `locate` download the map exactly as they do today.

### 3.2 Publish step: `atb agc index` builds the combined TSV

Extend the existing `atb agc index` command so it can produce the hosted index:

1. Crawl the three finalized nodes' `agc_batches/` folders
   (`osf.CrawlAGCCollection` over the new `AGCCollectionNodes`) to get, per batch:
   `batch_name`, node id, OSF download URL, md5, size.
2. Download and parse the batch metadata (`8y9r2`) to get, per batch: `old_name`
   and `nb_genomes`.
3. Join on `batch_name` and write the combined 6-column index TSV (schema in 3.6),
   with the species derived from `old_name`.

A maintainer runs this, then uploads the resulting `atb_agc_files.tsv` to the
`metadata_balanced_batches/` folder on OSF node `z7q5y` (where `gtqrx` and `8y9r2`
already live) and records the new `/download/<guid>/` URL in `sources.AGCIndexURL`.

### 3.3 Source constant changes (`internal/sources/sources.go`)

- `AGCCollectionNodes` becomes `4jq8u`/`jmeqg`/`kzcnr`.
- `AGCArchivesFolder` becomes `agc_batches` (the finalized folder name).
- `AGCArchiveMapURL` becomes the OSF download URL for `gtqrx`;
  `AGCArchiveMapFilename` becomes `assemblies_filelist.txt.gz`.
- `AGCIndexURL` becomes the hosted combined-index URL once uploaded (see 4).
- Retire the legacy `z7q5y` test-node constants (`AGCTestNodeID`,
  `AGCBatchesFolder`) and the CESGO archive host (`AGCArchiveBaseURL`, `ArchiveURL`).
- Rework the node-to-tier model: `AGCNode.Part` and `PartForNode` no longer hold,
  because a node no longer implies a tier. `locate` reads species and node from the
  index entry instead (3.4).

### 3.4 Runtime code changes

- `internal/agc/resolve.go` (`openMap`): add gzip detection (magic `1f 8b`)
  alongside the existing zip (`PK`) and plain-text paths, so the gzipped map is
  decompressed on read. Confirm or adjust `ParseMap` field splitting so the map's
  delimiter (whitespace) is accepted; column 1 is the accession, column 2 the batch
  name without extension.
- `internal/osf/agc_index.go` (`SpeciesFromArchive`): derive the species from the
  `old_name` forms `<Species>_global_ordered.partNNN` and `unknown.partNNN`
  (cut at the first of `_global_ordered` or `.part`; `unknown` maps to `unknown`).
  Add the build helper that crawls the collection and joins the batch metadata
  (3.2). This function runs at publish time only.
- `internal/agc/locate.go` and `internal/cli/agc_locate_cmd.go`: replace the
  node-tier `Part` field with the **species** and **node** taken from the batch's
  index entry (`Project` and `ProjectID`). Drop the `PartForNode` dependency.

### 3.5 Graduation to stable

- Remove "preview" and "test data" wording from command help and the guide docs;
  the hosted combined index is the hard default.
- Regenerate the CLI reference with `make docs` (the CI staleness gate needs Go
  1.25).
- Update `CHANGELOG.md` and cut **v0.18.0 final** (not a pre-release).

### 3.6 Index schema (reuse the existing 6-column TSV)

The finalized data maps onto the current `osf.ParseIndex` and
`osf.WriteAGCIndexTSV` schema with no format change:

| Column | Source |
|--------|--------|
| `project` | species, from `SpeciesFromArchive(old_name)` |
| `project_id` | node id (`4jq8u`/`jmeqg`/`kzcnr`) |
| `filename` | `batch_name` (`atb.assembly.202505_all.batch.NNNN.agc`) |
| `url` | OSF download URL from the node listing |
| `md5` | md5 from the node listing |
| `size_mb` | size from the node listing |

`nb_genomes` is not carried in v1 to avoid a schema change; it can be added as a
seventh column later if a genome count is wanted in `--species`/`locate` output.

---

## 4. Migration and rollout sequence

Ordered so every step lands green before the one external dependency is resolved:

1. Update constants, runtime code, and `atb agc index` (sections 3.3, 3.4, 3.2)
   with `AGCIndexURL` left empty. With an empty `AGCIndexURL`, `useHostedAGCIndex`
   is false and atb falls back to `FetchAGCCollection`, crawling the finalized
   nodes directly. The feature is fully working against live data at this point.
2. Refresh fixtures and tests (section 5); run the live e2e smoke against the
   finalized nodes over the crawl path.
3. Run `atb agc index` to build the combined TSV; upload it to OSF `z7q5y`; set
   `sources.AGCIndexURL` to its `/download/<guid>/` URL. atb now downloads the
   single hosted file; the crawl remains the automatic fallback.
4. Graduation (section 3.5): strip preview labels, regenerate docs, update the
   changelog, cut v0.18.0 final.

Running impact analysis (`gitnexus_impact`) before editing each symbol, and
`gitnexus_detect_changes` before committing, is required by the repo conventions in
`CLAUDE.md` and applies to every code edit in this plan.

---

## 5. Testing and validation

- Replace `testdata/atb_agc_files.tsv` (retired `z7q5y` names) with a
  balanced-naming fixture: numbered batches, the three finalized nodes in
  `project_id`, and species in `project`.
- Update unit tests that assert node ids, the folder name, or the naming
  convention.
- Add `SpeciesFromArchive` cases for `<Species>_global_ordered.partNNN` and
  `unknown.partNNN`, and a gzip map-parse case in `resolve_test.go`.
- Live e2e smoke against the finalized OSF nodes: `atb agc locate <accession>`,
  `atb agc download --species <species>`, and one by-accession download, each
  md5-verified.

---

## 6. External dependency

Cutting v0.18.0 final needs the combined index uploaded to a stable OSF GUID. The
recommended owner is the maintainer: generate the TSV with `atb agc index`, upload
it to the `metadata_balanced_batches/` folder on node `z7q5y`, record the GUID in
`AGCIndexURL`. Alternatively the collaborator, who already publishes `gtqrx` and
`8y9r2` in that folder, publishes it. Until the GUID is set, the crawl fallback
(section 4, step 1) keeps the feature fully functional, so no code is blocked on
the upload.

---

## 7. Out of scope

- `atb fetch-annotations` over `.bakpack` annotations
  ([`agc-annotations-bakpack.md`](./agc-annotations-bakpack.md)). The genome
  retrieval it depends on has landed, so it is unblocked, but it is a separate
  feature tracked in its own ADR.
- Carrying `nb_genomes` in the index (3.6) and any change to the accession-map
  format beyond the compression and naming update.

---

## 8. Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Batch metadata `old_name` has a species form the parser misses | Medium | Enumerate distinct `old_name` prefixes from `8y9r2` at build time; assert every batch resolves to a non-empty species before writing the TSV |
| The accession map delimiter differs from what `ParseMap` splits on | Medium | Confirm the delimiter against `gtqrx` and add a parse test with a real sample line |
| Old preview nodes removed before the migration ships | Low | The migration targets the finalized nodes, which are independent of the preview nodes; no dependency on the preview nodes remaining |
| Hosted index GUID not yet uploaded at release time | Low | Crawl fallback keeps the feature working; the upload gates only the final `AGCIndexURL` flip, not the code |
| `make docs` staleness gate fails in CI | Low | Regenerate with Go 1.25 locally before pushing |
