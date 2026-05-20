#!/bin/sh
set -e

DATA_DIR="${ATB_DATA_DIR:-${ATB_DATADIR:-/data}}"
HTTP_PORT="${ATB_HTTP_PORT:-8080}"

# Check if database exists
if [ ! -f "$DATA_DIR/assembly.parquet" ]; then
    echo "==> No database found in $DATA_DIR"
    echo "==> Downloading core metadata tables from OSF (~540 MB)..."
    atb --data-dir "$DATA_DIR" fetch
    echo ""
fi

# Check if index exists
if [ ! -f "$DATA_DIR/atb_index.sqlite" ]; then
    echo "==> Building SQLite query index (this takes ~4 minutes)..."
    atb --data-dir "$DATA_DIR" index
    echo ""
fi

echo "==> Starting MCP server on :${HTTP_PORT}"
echo "==> SSE endpoint: http://localhost:${HTTP_PORT}/sse"
echo "==> Data directory: ${DATA_DIR}"
echo ""

exec atb --data-dir "$DATA_DIR" mcp --http ":${HTTP_PORT}"
