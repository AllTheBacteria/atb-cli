# Installation

Install `atb` on Linux, macOS, or Windows using the one-line script, a pre-built binary, or from source.

## Download pre-built binaries

Pre-built binaries for all platforms:

| Platform | Architecture | File |
|----------|-------------|------|
| Linux | x86_64 (amd64) | `atb-cli_<version>_linux_amd64.tar.gz` |
| Linux | ARM64 | `atb-cli_<version>_linux_arm64.tar.gz` |
| macOS | Intel (amd64) | `atb-cli_<version>_darwin_amd64.tar.gz` |
| macOS | Apple Silicon (arm64) | `atb-cli_<version>_darwin_arm64.tar.gz` |
| Windows | x86_64 (amd64) | `atb-cli_<version>_windows_amd64.zip` |
| Windows | ARM64 | `atb-cli_<version>_windows_arm64.zip` |

**Latest release:** [github.com/allthebacteria/atb-cli/releases/latest](https://github.com/allthebacteria/atb-cli/releases/latest)

Download the file for your platform, extract, and place the `atb` binary (or `atb.exe` on Windows) somewhere in your `PATH`.

## One-line installer (Linux/macOS)

**One-line install** (Linux/macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/allthebacteria/atb-cli/main/install.sh | bash
```

This detects your OS and architecture, downloads the latest release, and installs to `~/.local/bin`. It will add the directory to your `PATH` automatically if needed.

```bash
# Install a specific version
curl -fsSL https://raw.githubusercontent.com/allthebacteria/atb-cli/main/install.sh | ATB_VERSION=v0.1.0 bash

# Install the binary to a custom directory
curl -fsSL https://raw.githubusercontent.com/allthebacteria/atb-cli/main/install.sh | ATB_INSTALL_DIR=/usr/local/bin bash
```

!!! note "Environment variable placement"
    The env var goes **after** the pipe so it applies to `bash`, not `curl`. Writing `ATB_INSTALL_DIR=/path curl ... | bash` looks plausible but only sets the variable for the `curl` process — the install script run by `bash` never sees it.

!!! warning "Pick a data directory before fetching"
    `ATB_INSTALL_DIR` only controls where the `atb` binary lands, not where the database goes. A default `atb fetch` pulls the core parquet tables (~600 MB) **and** builds the per-genus AMR indexes, which expand to **~35 GB** on disk. Add another ~4.2 GB if you run `atb sketch fetch`. By default everything goes to `~/.local/share/atb/data`, which is often on a small home volume. To put it elsewhere, either pass `--data-dir` per command or set it once:

    ```bash
    atb config set general.data_dir /path/to/large/volume
    # or, ad hoc:
    atb fetch --data-dir /path/to/large/volume
    ```

    If you don't need AMR queries, you can skip that table to save the ~35 GB. `atb fetch` will prompt before downloading `amrfinderplus.parquet`; answer **n** to skip just that table and continue with the rest. To bypass the prompt non-interactively, either pre-select tables or pass `--yes`:

    ```bash
    # Skip AMR explicitly (non-interactive)
    atb fetch --tables assembly.parquet,assembly_stats.parquet,checkm2.parquet,sylph.parquet,run.parquet,mlst.parquet

    # Or accept the AMR download without prompting
    atb fetch --yes
    ```

## Windows

**Windows:** Download the `.zip` from the [Download pre-built binaries](#download-pre-built-binaries) table above, extract, and add `atb.exe` to your PATH.

## Other methods

```bash
# Go install (requires Go 1.23+)
go install github.com/allthebacteria/atb-cli/cmd/atb@latest

# From source
git clone https://github.com/allthebacteria/atb-cli.git
cd atb-cli
make build    # binary at ./bin/atb
```

## Docker

The repository ships a `Dockerfile` at its root for containerized use. You can build and run `atb` inside a container without installing Go or the binary directly on the host.

For MCP-server deployment with Docker Compose (`docker compose up -d`), see [MCP server](../mcp.md) — that page covers the full setup including public access, SSE endpoints, and cloud deployment options.

---

Next: [Quick start](quickstart.md)
