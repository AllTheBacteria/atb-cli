# ADR: Migrate `atb fetch-genomes` to OSF-hosted AGC archives

- **Status:** Proposed (architecture)
- **Date:** 2026-06-25
- **Author:** Architecture review
- **Scope:** `feat/agc-reader` branch + `internal/agc`, `internal/osf`, `internal/sources`
- **Trigger:** Tam Truong staged 767 per-species AGC batches on OSF node `z7q5y`
  ("ATB testing", 38.5 GB, uploaded 2026-06-20). This is the OSF migration the
  branch's `sources.go` flagged as "still under discussion".

> **Update (2026-06-28):** The CLI command discussed here as `atb fetch-genomes`
> has since moved to **`atb agc download`** (see
> [`agc-download-command.md`](agc-download-command.md)). This ADR is preserved as
> the original point-in-time record; the command and file names below describe the
> pre-rename CLI.

> **Update (v0.18.1):** The default AGC batch-index source has moved on from the
> single `z7q5y` test node described here. atb now defaults to a hosted OSF index TSV
> over the finalized balanced **v202505** collection (crawling its three OSF nodes is
> the fallback; `--agc-index`/`--osf-node` override). Accession-mode `atb agc download`
> and the read-only `atb agc locate` are officially released - graduated from preview
> in v0.18.0 (2026-08-03), current in **v0.18.1** (2026-08-04). See
> [`agc-osf-balanced-migration.md`](./agc-osf-balanced-migration.md) for the current
> design and [`agc-osf-accession-search.md`](./agc-osf-accession-search.md) for `locate`.

---

## 1. Context

### 1.1 What already exists (the `feat/agc-reader` branch)

The branch ships a **feature-complete, tested** genome-fetch stack:

| Layer | File | Role |
|-------|------|------|
| Binary mgmt | `internal/agc/agc.go` | Locate/install pinned `agc` v3.2.3 from GitHub releases |
| Query | `internal/agc/query.go` | Shell-out wrappers: `listset`, `listctg`, `listref`, `getset`, `info` |
| Fetch | `internal/agc/fetch.go` | Cache-first archive download + per-sample/combined extraction |
| Resolve | `internal/agc/resolve.go` | accession → archive map (fetch/parse/cache) |
| CLI | `internal/cli/fetch_genomes_cmd.go` | `atb fetch-genomes` (primary path) |
| CLI | `internal/cli/agc_cmd.go` | `atb agc ls/info/get` (low-level "escape hatch") |
| Config | `internal/config/config.go` | `AGCConfig{ArchiveMapURL, ArchiveBaseURL, ArchiveDir}` |

Key design choices, all **sound and retained by this ADR**:

- **Shell-out to the upstream `agc` binary** rather than a native Go reader or
  cgo. This mirrors the existing `internal/sketch` sketchlib pattern
  (`FindBinary` → next-to-atb / PATH / install hint; `InstallBinary` from
  GitHub releases). atb stays pure-Go and statically linked.
- **Cache-first archives**, grouped by archive so N samples in one batch cost
  one download.
- **Per-sample or `--combine`** extraction; dry-run; `--keep-going`.
- **Config seams** (`agc.archive_map_url`, `agc.archive_base_url`) already exist
  for redirecting the data source without code changes.

### 1.2 Why migrate (the data-distribution shift)

AllTheBacteria is moving genome distribution from per-batch MiniPhy `.tar.xz`
to **AGC archives**. Evidence from the live master index (`all_atb_files.tsv`,
OSF `r6gcp`):

```
1971  .tar.xz          <- legacy MiniPhy assembly tarballs
 650  .cobs_classic.xz <- COBS search indexes
   0  .agc             <- new channel not yet registered here
```

AGC's advantage is **cross-genome delta compression + few-second random access**
(`agc getset archive.agc SAMPLE`). For "give me species X" or "give me 500
genomes", one delta-compressed batch beats N independently-gzipped S3 objects on
both bytes and request count.

### 1.3 The two facts that break a naive migration

- **F1 — OSF download URLs are opaque GUIDs**, not name-addressable:
  `Mycoplasmoides_pneumoniae_global_ordered_0001.agc` →
  `https://osf.io/download/6a35ea36343d8094f71667f4/`.
  The branch builds `ArchiveBaseURL + name + ".agc"` (correct for CESGO's
  Nextcloud share, **wrong for OSF**).
- **F2 — z7q5y ships archives only.** No accession→archive map, no checksum
  manifest, and the files are not in the master index. The branch's resolver
  *requires* an accession→archive map (today a ~90 MB whitespace file on a
  provisional CESGO endpoint).

### 1.4 The file, investigated

`Mycoplasmoides_pneumoniae_global_ordered_0001.agc` (smallest batch, 0.98 MB):
header begins `01 70` then the **Zstandard** magic `28 B5 2F FD`; trailing
sections `params`, `splitters`, `segment-splitters`, `file_type_info` are AGC's
v3 metadata. The sample/contig collection is itself zstd-compressed (so
`strings` cannot reveal accessions — only `agc listset` can). Naming is
`<Species>_global_ordered_<NNNN>`; species map to 1..N batches
(*S. enterica* 136, *E. coli* 77, … long tail of 1).

---

## 2. Decision

**Keep the branch's engine. Replace only the data-source/resolution layer with
an OSF-native adapter built on the *existing* `internal/osf` index machinery,
structured as three resolution tiers with graceful fallback.**

The insight that makes this clean: **the master index format already is a
name → URL + md5 + size table.** If the AGC archives are registered in it
(an upstream action), then `internal/osf` *already* resolves
`archive_name → download URL` with integrity metadata — **no OSF-GUID problem,
no new fetch code, free checksums.** atb only filters the cached index for the
`.agc` row whose filename matches the wanted archive.

### 2.1 Two access modes + a shared address resolver

The key realization is that the AGC filenames *are* metadata. Because every
batch is named `<Species>_global_ordered_<NNNN>`, the set of batches for a
species is derivable from a plain node listing — **no upstream map required**.
That splits the problem into two access modes with very different dependencies:

- **Mode A — by-species** (`atb fetch-genomes --species "Escherichia coli"`):
  needs **zero new upstream data**. List the node → keep names with the species
  prefix → download those batches → extract. This is AGC's headline use case and
  it ships the day the archives are reachable.
- **Mode B — by-accession** (`atb fetch-genomes SAMEA…`): needs the **one**
  artifact only the producer can make — the accession→archive map — because the
  global-ordering→batch assignment is not encoded in any filename.

Both modes converge on one shared resolver (**R2: archive_name → download
URL**) and one correctness backstop (S3 per-sample `.fa.gz`).

```
  REQUEST
  ───────
  "all of species X" ─► Mode A: by-species   (NO upstream map needed)
                         node listing → filter <Species>_global_ordered_*
                         → batch names ─────────────┐
                                                     │
  "these accessions" ─► Mode B: by-accession (needs published R1 map)
                         accession → archive_name    │
                         via agc_index.tsv.gz ───────┤
                                                     ▼
                         R2: archive_name → download URL (+ md5)
                         via master index  (preferred — reuses internal/osf)
                         ── or cached OSF-API node crawl (self-service)
                                                     │
                                                     ▼
                         download (cache-first, md5) → agc getset → FASTA
                                                     │ accession in no batch
                                                     ▼
                         Backstop: S3 {accession}.fa.gz (existing atb download)
```

- **R1 (accession → archive), Mode B only:** only the data producer knows the
  global-ordering→batch assignment, so this map must come from upstream — atb
  cannot derive it without downloading all 38.5 GB and running `listset`.
  Publish it gzipped (`agc_index.tsv.gz`); the branch's `FetchMap` (7-day TTL,
  atomic temp-rename) already handles caching — add transparent gzip. Later, the
  deferred SQLite fast-path (seam already in `resolve.go`).
- **R2 (archive → URL), both modes:** resolve via `internal/osf` against the
  master index (**preferred** — gives md5 for free and reuses every line of the
  existing fetch/parse/cache code). If the archives are not yet registered there,
  resolve via a **cached OSF-API node crawl** (767 entries → name→GUID, with md5
  from `extra.hashes.md5`) — this path is fully self-service and works *today*
  against z7q5y for validation. Either way OSF's opaque-GUID scheme is handled by
  data, never by URL string-building.
- **Backstop (S3 per-sample):** for an accession in no AGC batch (ENA gaps), or
  when no map is present, fall back to the existing `{accession}.fa.gz` S3 path
  (`atb download`). Correctness floor; already implemented.

### 2.2 Sourcing / config

- **Do not hardcode `z7q5y`.** It is "ATB testing" — a staging node whose IDs
  will move on promotion (expected under the `xv7q9` umbrella, e.g. an
  `Assembly/AGC` component). Ship production defaults; reach z7q5y only via the
  existing `agc.archive_map_url` config or a hidden `--osf-node` flag for
  pre-release validation.
- Prefer **master-index resolution** so there is *no* AGC-specific base URL to
  pin. Keep `agc.archive_base_url` as an optional override only.

---

## 3. Upstream asks (blocking, data-side)

These are the only true blockers; they are data-publishing actions, not code:

1. **Promote** the AGC batches from `z7q5y` ("testing") to the production node.
2. **Register** every `.agc` batch in `all_atb_files.tsv` (gives atb URL + md5
   + size for free via `internal/osf`).
3. **Publish** the accession→archive map `agc_index.tsv.gz` and register it too.
4. **Guarantee** AGC sample names == ATB sample accessions (validate with
   `agc listset`); the branch's `getset archive accession` round-trip depends
   on it.

---

## 4. atb-side work (branch changes)

1. **R2 adapter** in `internal/agc` (or extend `internal/osf`): resolve
   `archive_name → {url, md5}` from the cached master index; fallback to a
   cached OSF-API node crawl. This is the load-bearing change — it removes the
   CESGO name-addressable assumption and makes OSF GUIDs a non-issue.
2. **Mode A (by-species)** in `fetch_genomes_cmd.go`: a `--species` selector
   that filters the node listing by the `<Species>_global_ordered_` prefix — no
   R1 map dependency. Ships first; validates the whole pipeline end-to-end.
3. **Swap defaults** from CESGO → master-index resolution; demote
   `archive_base_url` to optional.
4. **Gzip** support in `FetchMap` (the published R1 map is gzipped).
5. **Backstop wiring** in `ResolveArchives`: keep the existing
   `(groups, unresolved)` return, but route `unresolved` accessions to the S3
   `.fa.gz` path instead of only warning.
6. **Integrity:** md5-verify downloaded archives against the index value.
7. **Naming-consistency test** (fail-closed): assert every archive referenced by
   the map exists in the index/listing. This is the #1 silent-failure guard.
8. **Streaming mode** (`agc getset -s`) for large batches
   (`subthreshold_remainder_*` reach ~200 MB) to bound memory.
9. Keep `z7q5y` reachable behind config for validation only.

---

## 5. Validation plan

(Local build needs Go + `agc`, absent on the review box.)

1. Download a real z7q5y batch (e.g. the 0.98 MB *M. pneumoniae* batch).
2. `agc listset` → confirm sample names are ATB accessions.
3. `agc getset` one accession → FASTA; compare to that accession's S3 `.fa.gz`
   (byte/seq identity) to validate the AGC channel against the legacy source.
4. Run the branch's existing integration round-trip test against the real batch.
5. Only then flip defaults to the production node and release.

---

## 6. Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Pinning a *test* node (`z7q5y`) in release | High if rushed | Master-index resolution + config overrides; never default to z7q5y |
| Map archive names ≠ OSF filenames (drift) | Medium | Fail-closed naming-consistency test |
| AGC sample name ≠ accession | Medium | Validate via `listset` round-trip pre-release (§5) |
| 90 MB map first-use cost / memory | Medium | Gzip on the wire; deferred SQLite fast-path |
| ENA-gap genomes (~10k not in S3/AGC) | Low | Clear `unresolved` reporting; Tier-3 fallback |
| Big batch extract memory (~200 MB archives) | Low | `agc getset -s` streaming mode |
| CESGO endpoint disappears | Low (once migrated) | OSF becomes source of truth post-migration |

---

## 7. Consequences

- **Positive:** OSF-native (no GUID hack); reuses `internal/osf` (minimal new
  code); free md5 integrity; graceful degradation to S3; bulk species fetch
  becomes first-class; no dependency on the provisional CESGO host.
- **Negative / cost:** depends on four upstream data-publishing actions (§3);
  retains a shell-out dependency on the `agc` binary (acceptable — matches
  sketchlib); first-use map download remains until the SQLite fast-path lands.
- **Deferred:** SQLite resolver fast-path; per-genus map sharding; `agc` binary
  checksum-pinning.
