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

See also: [Fetching & indexing data](../guides/fetch-and-index.md) for how to download these data sources locally.
