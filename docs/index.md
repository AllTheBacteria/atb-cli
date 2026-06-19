# atb-cli

`atb-cli` (the `atb` command) queries the [AllTheBacteria](https://osf.io/xv7q9/)
genomics database (~3.2M bacterial genomes), searches AMR / stress / virulence
genes, finds the closest genomes via sketch distances, and downloads genome
assemblies — all from your terminal. It ships as a single binary with no
dependencies, and reads the project's Parquet metadata through a fast local
index, so most queries return in milliseconds.

**Supported platforms:** Linux, macOS, Windows (amd64 and arm64).

## What you can do

- **Query genomes** by species, quality, geography, sequencing platform, and date.
- **Search resistance, stress, and virulence genes** (AMRFinderPlus) and **MLST** types.
- **Find the closest genomes** to your own assembly across ~3.2M genomes.
- **Download assemblies** for any query result, with preview and parallelism controls.
- **Browse and fetch** the underlying data files from OSF.
- **Drive it from an LLM** through the built-in MCP server.

## Get started

1. [Install atb-cli](getting-started/installation.md)
2. [Quick start](getting-started/quickstart.md) — download the database and run your first query
3. Work through the [guides](guides/query.md) for task-by-task walkthroughs
4. Look up exact flags in the [CLI reference](reference/cli/atb.md)

## Credits

`atb-cli` was designed and architected by
[Thanh Le Viet](https://github.com/thanhleviet) in his personal capacity, using
his own Claude account. The implementation was developed with coding assistance
from [Claude](https://claude.ai) (Anthropic) under human direction and review.
Thanks to hackathon participants Jane Hawkey, Ahmed M Moustafa, Martin Hunt, and
Zamin Iqbal for their input and feedback.

Released under the [MIT License](https://github.com/AllTheBacteria/atb-cli/blob/main/LICENSE).
