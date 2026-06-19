# Performance

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

Full benchmark details, methodology, and comparisons across query tiers: **[docs/BENCHMARKS.md](https://github.com/AllTheBacteria/atb-cli/blob/main/docs/BENCHMARKS.md)**

See also: [Fetching & indexing data](../guides/fetch-and-index.md) for the one-time setup that enables these query speeds.
