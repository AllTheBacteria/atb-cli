package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/allthebacteria/atb-cli/internal/config"
	"github.com/allthebacteria/atb-cli/internal/mcpserver"
)

func newMCPCmd() *cobra.Command {
	var httpAddr string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server for LLM integration",
		Long: `Start a Model Context Protocol (MCP) server over stdio or HTTP/SSE.

This allows LLMs like Claude and ChatGPT to query the AllTheBacteria database
through structured tool calls.

The server exposes 5 tools:
  atb_query        Search bacterial genomes by species, genus, quality, etc.
  atb_amr          Query AMR resistance genes for a species
  atb_info         Get all metadata for a specific sample accession
  atb_stats        Database summary statistics
  atb_species_list List available species sorted by sample count

Usage with Claude Code:
  claude mcp add atb -- atb mcp --data-dir ~/atb/metadata/parquet

Usage with Claude Desktop (add to claude_desktop_config.json):
  {
    "mcpServers": {
      "atb": {
        "command": "atb",
        "args": ["mcp", "--data-dir", "/path/to/data"]
      }
    }
  }

Usage with ChatGPT / OpenAI API (HTTP/SSE mode):
  Start the server with --http, then configure ChatGPT with the SSE URL.`,
		Example: `  # Start stdio server (for Claude Code, Cursor, Codex CLI)
  atb mcp --data-dir ~/atb/metadata/parquet

  # Start HTTP/SSE server (for ChatGPT, OpenAI API, remote clients)
  atb mcp --data-dir ~/atb/metadata/parquet --http :8080

  # ChatGPT: configure with URL http://your-host:8080/sse

  # Add to Claude Code
  claude mcp add atb -- atb mcp --data-dir ~/atb/metadata/parquet`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			dir := resolveDataDir(cfg)
			if dir == "" {
				dir = config.DefaultDataDir()
			}

			if httpAddr != "" {
				if err := mcpserver.ServeHTTP(cmd.Context(), dir, httpAddr); err != nil {
					return fmt.Errorf("MCP HTTP server error: %w", err)
				}
				return nil
			}

			if err := mcpserver.ServeStdio(cmd.Context(), dir); err != nil {
				return fmt.Errorf("MCP server error: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&httpAddr, "http", "", "Start HTTP/SSE server on this address (e.g., :8080)")

	return cmd
}
