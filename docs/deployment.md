# Deploying the ATB MCP Server

Deploy the ATB MCP server as a Docker container so ChatGPT, OpenAI API, or any remote MCP client can query the AllTheBacteria database.

## Quick Start (Docker Compose)

```bash
git clone https://github.com/allthebacteria/atb-cli.git
cd atb-cli
docker compose up -d
```

First run takes ~10 minutes (downloads 540 MB of parquet data + builds 1.2 GB SQLite index). Subsequent starts are instant.

The MCP SSE endpoint will be available at `http://localhost:8080/sse`.

## What happens on first start

1. The container starts and checks for data in `/data`
2. If no parquet files exist, it runs `atb fetch` to download core metadata tables (~540 MB from OSF)
3. If no SQLite index exists, it runs `atb index` to build the query index (~4 min, produces 1.2 GB file)
4. Starts the MCP HTTP/SSE server on port 8080

Data persists in a Docker volume, so you only pay the download/index cost once.

## Connect ChatGPT

1. Start the server (see above)
2. If running locally, expose it with ngrok: `ngrok http 8080`
3. In ChatGPT: Settings > Connected apps > Add MCP server
4. Enter the SSE endpoint URL: `https://your-ngrok-url.ngrok.io/sse`

## Deploy to Fly.io

```bash
# Install flyctl
curl -L https://fly.io/install.sh | sh

# Launch the app
fly launch --no-deploy

# Create a persistent volume (3 GB for parquet + index)
fly volumes create atb_data --size 3 --region lhr

# Deploy
fly deploy
```

Create `fly.toml`:

```toml
app = "atb-mcp"
primary_region = "lhr"

[build]

[http_service]
  internal_port = 8080
  force_https = true

[mounts]
  source = "atb_data"
  destination = "/data"

[[vm]]
  size = "shared-cpu-1x"
  memory = "2gb"
```

After deploy, your MCP endpoint is at `https://atb-mcp.fly.dev/sse`.

## Deploy to Railway

1. Connect your GitHub repo at [railway.app](https://railway.app)
2. Add a volume mounted at `/data` (3 GB)
3. Set environment variables:
   - `ATB_DATA_DIR=/data`
   - `ATB_HTTP_PORT=8080`
4. Deploy - Railway auto-detects the Dockerfile

Your endpoint: `https://your-app.up.railway.app/sse`

## Deploy to any VPS

```bash
# On your server
git clone https://github.com/allthebacteria/atb-cli.git
cd atb-cli
docker compose up -d

# Set up a reverse proxy (nginx example)
cat > /etc/nginx/sites-available/atb-mcp <<'EOF'
server {
    listen 443 ssl;
    server_name atb-mcp.yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/atb-mcp.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/atb-mcp.yourdomain.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_buffering off;
        proxy_cache off;
        # SSE requires no buffering
        proxy_read_timeout 86400s;
    }
}
EOF
```

## Using with pre-existing data

If you already have the parquet files and index somewhere, mount them directly:

```bash
docker run -d \
  -p 8080:8080 \
  -v /path/to/your/parquet/data:/data:ro \
  ghcr.io/allthebacteria/atb-cli:latest
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ATB_DATA_DIR` | `/data` | Directory for parquet files and SQLite index |
| `ATB_HTTP_PORT` | `8080` | Port for the HTTP/SSE server |

## Resource Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 1 vCPU | 2 vCPU |
| RAM | 512 MB | 2 GB |
| Disk | 3 GB | 5 GB |
| Network | Outbound to osf.io (first run only) | |

Queries use ~15 MB RAM each (SQLite indexed lookups). The 2 GB recommendation is for the index build step, which is memory-intensive but only runs once.

## Health Check

```bash
# Verify the server is running
curl -s http://localhost:8080/sse -H "Accept: text/event-stream" --max-time 2
```

## Updating the Database

When a new ATB release is published:

```bash
# Enter the running container
docker compose exec atb-mcp sh

# Re-fetch and rebuild
atb --data-dir /data fetch --force
atb --data-dir /data index --force

# Or restart the container after deleting the data
docker compose down
docker volume rm atb-cli_atb-data
docker compose up -d
```
