# Data sources

All external URLs are defined in [`internal/sources/sources.go`](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go) -- a single file that documents every URL the tool accesses.

| Data | Source | Used by |
|------|--------|---------|
| **Parquet metadata** (assembly, QC, species, MLST, AMR) | [OSF (h7wzy)](https://osf.io/h7wzy/files/osfstorage) `Aggregated/Latest_2025-05/` | `atb fetch` |
| **ENA metadata** (geography, platform, dates) | Same OSF project, optional tables | `atb fetch --all` |
| **AMR/Stress/Virulence genes** | [AMRFinderPlus](https://github.com/ncbi/amr) v4.2.5 results as `amrfinderplus.parquet` (~58.5M rows, ~1.18 GB) | `atb amr` |
| **OSF file index** | [all_atb_files.tsv](https://osf.io/r6gcp/) (~3,000 files, 75+ categories) | `atb osf ls`, `atb osf download` |
| **Sketch database** | Same OSF project, `atb_sketchlib.aggregated.202408` (.skm + .skd, ~4.2 GB) | `atb sketch fetch` |
| **Genome assemblies** | `allthebacteria-assemblies.s3.eu-west-2.amazonaws.com` | `atb download`, `atb sketch query --download` |
| **sketchlib binary** | [bacpop/sketchlib.rust](https://github.com/bacpop/sketchlib.rust/releases) (Linux/macOS) | `atb sketch install` |

## Link registry

Every URL above is defined once, in
[`internal/sources/sources.go`](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go),
and read from there everywhere. This section maps each OSF link to the constant
and line that define it, so a change on either side is easy to trace:

- **A link changed on OSF** (a file was re-uploaded and its `/download/<id>/`
  URL changed): find the `<id>` in the tables below, open the linked line, and
  edit that one constant. Nothing else references the URL.
- **You are changing a link in code**: edit the constant at the linked line. The
  local cache is keyed on the URL, so users re-fetch automatically on the next run.

OSF `/download/<id>/` URLs are opaque -- the `<id>` is either a 5-character OSF
GUID (`r6gcp`) or a 24-character file id (`6a719381...`), and it names neither
the file nor its location. Use the **OSF id** column as the lookup key.

### OSF file index

| Constant | Defined at | OSF id | Local file | Used by |
|----------|-----------|--------|-----------|---------|
| `IndexURL` | [sources.go:20](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L20) | `r6gcp` | `all_atb_files.tsv` (~3,000 files) | `atb osf ls`, `atb osf download` |

### Parquet metadata tables

Node `h7wzy`, folder `Aggregated/Latest_2025-05/atb.metadata.202505.parquet/`.
All entries live in the [`TableURLs`](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L32)
map. The core tables download with `atb fetch`; the ENA tables need `atb fetch --all`.

| Table | Defined at | OSF id | Group |
|-------|-----------|--------|-------|
| `assembly.parquet` | [sources.go:34](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L34) | `4ku2n` | core |
| `assembly_stats.parquet` | [sources.go:35](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L35) | `69c51e86801fecc5d6146396` | core |
| `checkm2.parquet` | [sources.go:36](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L36) | `69c51e93cba7111bb21d27f2` | core |
| `sylph.parquet` | [sources.go:37](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L37) | `69c51f90cba7111bb21d2905` | core |
| `run.parquet` | [sources.go:38](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L38) | `69c51f68376eb79a651d2d85` | core |
| `mlst.parquet` | [sources.go:39](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L39) | `69c66d33fa3d973d94254f46` | core |
| `amrfinderplus.parquet` | [sources.go:40](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L40) | `69f1e5debb4f674d5fd949ad` | core |
| `ena_20250506.parquet` | [sources.go:43](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L43) | `69c51f3ab4f99c692d54cf73` | `--all` |
| `ena_20240801.parquet` | [sources.go:44](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L44) | `69c51f002e72f67915145d0e` | `--all` |
| `ena_20240625.parquet` | [sources.go:45](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L45) | `69c51ec99ce80b96ac54cd08` | `--all` |
| `ena_202505_used.parquet` | [sources.go:46](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L46) | `69c51f475eedad376954ce7b` | `--all` |
| `ena_661k.parquet` | [sources.go:47](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L47) | `69c51f57376eb79a651d2d83` | `--all` |

### Sketch database

Node `h7wzy`, folder `Aggregated/atb_sketchlib.aggregated.202408.*`.

| Constant | Defined at | OSF id | Local file | Used by |
|----------|-----------|--------|-----------|---------|
| `SketchSkmURL` | [sources.go:70](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L70) | `nwfkc` | `atb_sketchlib.skm` | `atb sketch fetch` |
| `SketchSkdURL` | [sources.go:75](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L75) | `92qmr` | `atb_sketchlib.skd` | `atb sketch fetch` |

### AGC genome archives

| Constant | Defined at | OSF id | Purpose | Used by |
|----------|-----------|--------|---------|---------|
| `AGCIndexURL` | [sources.go:148](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L148) | `6a719381a2e9e3d202b91f7d` | Published balanced-v202505 batch index (`atb_agc_files.tsv`); the default source | `atb agc download`, `atb agc locate` |
| `AGCArchiveMapURL` | [sources.go:118](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L118) | `gtqrx` | Accession-to-batch map (`assemblies_filelist.txt.gz`) | `atb agc download` (by accession), `atb agc locate` |
| `AGCBatchMetadataURL` | [sources.go:187](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L187) | `8y9r2` | Batch metadata joined in when the index is rebuilt (`batches_202505_metadata.tsv.gz`) | `atb agc index` |

The AGC index (`6a719381a2e9e3d202b91f7d`) resolves to node `h7wzy`, folder
`metadata_balanced_batches/atb_agc_files.tsv`, published md5
`8aea3d79da3e2a0af10c9904d0c3a10f` (1,268 batches). See the
[v0.18.1 verification](../design/agc-v0.18.1-smoke-test.md).

When `AGCIndexURL` is empty, the index is rebuilt by crawling the collection
nodes instead -- `4jq8u`, `jmeqg`, `kzcnr`
([sources.go:173](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L173)),
each under an `agc_batches/` folder, joined with the batch metadata above.
`OSFAPIBase`
([sources.go:138](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L138))
is the API root used for that crawl.

### Not on OSF

Two external sources in the same file are not hosted on OSF:

- `AssemblyBaseURL`
  ([sources.go:196](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L196))
  -- the S3 bucket of individual FASTA assemblies.
- `SketchlibRepo` / `AGCRepo`
  ([sources.go:93](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L93),
  [sources.go:104](https://github.com/AllTheBacteria/atb-cli/blob/main/internal/sources/sources.go#L104))
  -- GitHub release repos for the `sketchlib` and `agc` helper binaries.

See also: [Fetching & indexing data](../guides/fetch-and-index.md) for how to download these data sources locally.
