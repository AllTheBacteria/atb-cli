# LLM Integration (MCP)

`atb` includes a built-in [Model Context Protocol](https://modelcontextprotocol.io) server, allowing LLMs to query the AllTheBacteria database directly through natural language.

**Two transport modes:**
- **stdio** (default) - for Claude Code, Claude Desktop, Cursor, VS Code Copilot, Windsurf, OpenAI Codex CLI
- **HTTP/SSE** (`--http :8080`) - for ChatGPT, OpenAI Responses API, remote clients

**Tools exposed:**

| Tool | Description |
|------|-------------|
| `atb_query` | Search genomes by species, genus, quality, N50 |
| `atb_amr` | Query AMR resistance genes by species and drug class |
| `atb_mlst` | Query MLST scheme, ST, and allele calls |
| `atb_info` | Get full metadata for a specific sample |
| `atb_stats` | Database summary statistics |
| `atb_species_list` | List available species with genome counts |

## Quick try (no install needed, requires Go)

```bash
# Claude Code - runs directly from GitHub, available in all projects
claude mcp add --scope user atb -- go run github.com/allthebacteria/atb-cli/cmd/atb@latest mcp
```

First call takes ~10s to compile; cached after that.

## Setup

```bash
# 1. Install atb
curl -fsSL https://raw.githubusercontent.com/allthebacteria/atb-cli/main/install.sh | bash

# 2. Fetch the database and build the index
atb fetch
```

See [Fetching & indexing data](guides/fetch-and-index.md) for details on data directory options and selective table downloads.

## stdio mode (Claude, Cursor, Codex CLI)

=== "Claude Code"

    ```bash
    # Claude Code (available globally in all projects)
    claude mcp add --scope user atb -- atb mcp

    # Claude Code (current project only)
    claude mcp add atb -- atb mcp
    ```

=== "Claude Desktop"

    ```json
    {
      "mcpServers": {
        "atb": {
          "command": "atb",
          "args": ["mcp"]
        }
      }
    }
    ```

    Add to `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS or `%APPDATA%\Claude\claude_desktop_config.json` on Windows.

=== "Cursor"

    ```
    # Cursor (Settings > MCP Servers > Add)
    # Command: atb mcp
    ```

=== "OpenAI Codex CLI"

    ```toml
    # OpenAI Codex CLI (~/.codex/config.toml)
    [mcp_servers.atb]
    command = "atb"
    args = ["mcp"]
    ```

!!! note "Restart required"
    After adding, restart your client for the MCP server to become available.

## HTTP/SSE mode (ChatGPT, OpenAI API, remote clients)

```bash
# Start the HTTP/SSE server
atb mcp --http :8080
```

Then configure your client with the SSE endpoint URL:
- **ChatGPT:** Settings > Connected apps > Add MCP server > `http://your-host:8080/sse`
- **OpenAI Responses API:** Use `server_url: "http://your-host:8080/sse"` in the MCP tool config

For public access, deploy with Docker or use a tunnel:

```bash
# Docker (auto-downloads data on first run)
docker compose up -d
# SSE endpoint: http://localhost:8080/sse

# Or quick public URL with ngrok
atb mcp --http :8080 &
ngrok http 8080
```

!!! note "Non-default data location"
    If your data is in a non-default location, add `--data-dir /your/path` to all commands above.

## What you can ask your LLM

Once connected, you can ask natural language questions like:

- "How many Salmonella genomes are in the database?"
- "Find me 20 high-quality E. coli genomes with N50 > 200000"
- "What beta-lactam resistance genes does Klebsiella pneumoniae have?"
- "Show me all metadata for sample SAMD00000355"
- "What are the top 10 species by genome count?"

The LLM will call the appropriate `atb` tools and interpret the results for you.

For the full `atb mcp` command reference, see [CLI: atb mcp](reference/cli/atb_mcp.md).
