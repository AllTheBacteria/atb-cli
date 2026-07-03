# atb-cli

A command-line tool for querying the [AllTheBacteria](https://osf.io/xv7q9/) genomics database (~3.2M bacterial genomes), searching AMR/stress/virulence genes, finding closest genomes via sketch distances, and downloading genome assemblies.

Single binary, no dependencies.

**Supported platforms:** Linux, macOS, Windows (amd64 and arm64)

**Full documentation: <https://allthebacteria.github.io/atb-cli/>** (also on [Read the Docs](https://atb-cli.readthedocs.io))

## Install

**One-line install** (Linux/macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/allthebacteria/atb-cli/main/install.sh | bash
```

Pre-built binaries for all platforms are also available on the
[releases page](https://github.com/allthebacteria/atb-cli/releases/latest).

See the [installation guide](https://allthebacteria.github.io/atb-cli/getting-started/installation/) for all methods.

## Quick Start

```bash
# 1. Download the database (~540 MB core tables)
atb fetch

# 2. Query
atb query --species "Escherichia coli" --hq-only --limit 10
```

See the [quick start guide](https://allthebacteria.github.io/atb-cli/getting-started/quickstart/) for next steps.

## Documentation

- [Guides](https://allthebacteria.github.io/atb-cli/guides/query/) — querying genomes, AMR, MLST, sketch distances, downloads, and more
- [CLI reference](https://allthebacteria.github.io/atb-cli/reference/cli/atb/) — every command and flag
- [LLM integration (MCP)](https://allthebacteria.github.io/atb-cli/mcp/) — use `atb` as an MCP server with Claude, ChatGPT, Cursor, and others

## Credits

`atb-cli` was designed and architected by [Thanh Le Viet](https://github.com/thanhleviet) in his personal capacity, using his own Claude account. The implementation was developed with coding assistance from [Claude](https://claude.ai) (Anthropic), an AI assistant that helped with code generation, testing, and documentation under human direction and review. Thanks to hackathon participants Jane Hawkey, Ahmed M Moustafa, Martin Hunt, and Zamin Iqbal for their input and feedback.

## License

[MIT](LICENSE)
