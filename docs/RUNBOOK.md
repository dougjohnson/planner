# Flywheel Planner — Operator Runbook

## Quick Start

```bash
# Build production binary (includes embedded frontend)
make build

# Run with defaults (listens on 127.0.0.1:7432, data in ~/.flywheel-planner)
./build/flywheel-planner

# Open in browser
open http://127.0.0.1:7432
```

## Configuration

All configuration via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `FLYWHEEL_DATA_DIR` | `~/.flywheel-planner` | Local data root |
| `FLYWHEEL_LISTEN_ADDR` | `127.0.0.1:7432` | HTTP listen address |
| `FLYWHEEL_LOG_LEVEL` | `info` | Log level: debug, info, warn, error |
| `FLYWHEEL_OPENAI_API_KEY` | — | OpenAI API key |
| `FLYWHEEL_ANTHROPIC_API_KEY` | — | Anthropic API key |
| `FLYWHEEL_MOCK_PROVIDERS` | `false` | Enable mock providers for testing |
| `FLYWHEEL_MOCK_SCENARIO` | `happy-path-prd` | Mock scenario selection |

## Data Directory Structure

```
~/.flywheel-planner/
├── app.db                    # SQLite database (WAL mode)
├── app.db-wal               # WAL file (auto-managed)
├── credentials.json          # API keys (mode 0600, never exported)
└── projects/
    └── <slug>-<id>/
        ├── inputs/           # Seed PRDs and uploads
        ├── foundations/      # AGENTS.md, tech stack, architecture
        ├── raw/              # Raw model responses
        ├── prompts/          # Rendered prompt snapshots (redacted)
        ├── exports/          # Export bundles (.zip)
        └── manifests/        # Export manifests
```

## Database

- **Engine:** SQLite with WAL journal mode
- **Path:** `$FLYWHEEL_DATA_DIR/app.db`
- **Pragmas:** `journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000`, `synchronous=NORMAL`
- **Migrations:** Embedded in binary, run automatically on startup
- **Backup:** Copy `app.db` while the application is stopped

## Startup Sequence

1. Load configuration from environment
2. Initialize credential-redacting logger
3. Ensure data directory exists (0700 permissions)
4. Open SQLite database with hardened pragmas
5. Run schema migrations
6. Seed canonical prompt templates (idempotent)
7. Recover interrupted workflow runs
8. Start HTTP server

If any boot step fails, the application exits with a clear error message.

## Security

- **Loopback only:** Binds to `127.0.0.1` by default — never exposed to network
- **Credentials:** API keys in memory only, never in database or logs
- **Redaction:** All log output passes through credential-stripping handler
- **File permissions:** Data directory and credentials file use 0700/0600
- **CSP headers:** `default-src 'self'`, `X-Frame-Options: DENY`
- **No symlinks:** Data directory must not be a symlink

## Troubleshooting

### `make build` fails with embed error
- If you see `pattern all:dist: no matching files`, the embed directory is missing.
- Run `make frontend embed-copy` to populate it, or create the placeholder:
  ```bash
  mkdir -p cmd/flywheel-planner/dist
  echo '<html><body>dev</body></html>' > cmd/flywheel-planner/dist/index.html
  ```

### `go test -race` fails on macOS
The race detector requires CGo. Install Xcode Command Line Tools: `xcode-select --install`

### Application won't start
- Check log output for specific boot step failure
- Verify data directory permissions: `ls -la ~/.flywheel-planner`
- Verify SQLite is not locked: check for stale `.db-wal` files

### API keys not working
- Check environment variables: `echo $FLYWHEEL_OPENAI_API_KEY`
- Use the Models page (/models) to trigger credential validation
- Check `credentials.json` permissions: must be 0600

### Workflow stuck
- Check for interrupted runs on the project dashboard
- Use Resume/Retry/Abandon actions
- Check server logs for provider errors

### Export issues
- Run stabilization checks before exporting
- Address any blocking findings (errors block export, warnings don't)
- Verify disk space in data directory

## Monitoring

- **Health endpoint:** `GET /api/health` → `{"status":"ok"}`
- **SSE connection:** Dashboard shows Live/Reconnecting/Offline indicator
- **Usage metrics:** Token consumption and estimated costs on project dashboard
- **Structured logs:** JSON format to stdout, suitable for log aggregation

## Backup & Recovery

```bash
# Stop the application first
kill $(pgrep flywheel-planner)

# Backup
cp ~/.flywheel-planner/app.db ~/backup/app.db
cp -r ~/.flywheel-planner/projects/ ~/backup/projects/

# Restore
cp ~/backup/app.db ~/.flywheel-planner/app.db
cp -r ~/backup/projects/ ~/.flywheel-planner/projects/
```

## Build from Source

### Prerequisites

| Tool | Minimum Version | Check |
|------|----------------|-------|
| Go | 1.25.0 | `go version` |
| Node.js | 18.x | `node --version` |
| npm | 10.x | `npm --version` |

**macOS:**
- Install via Homebrew: `brew install go node`
- `xcode-select --install` is required only for `go test -race` (CGo dependency). The SQLite driver (`modernc.org/sqlite`) is pure Go and does not need CGo.

**Linux (Ubuntu/Debian):** See https://go.dev/dl/ for Go, https://nodejs.org/ for Node.

### Production Build

```bash
make build          # Frontend → embed-copy → Go binary
# Output: build/flywheel-planner (single binary, ~14 MB)
```

**What `make build` does:**
1. `make frontend` — `npm ci && npm run build` in `frontend/`
2. `make embed-copy` — copies `frontend/dist/` to `cmd/flywheel-planner/dist/` (Go's `embed` directive cannot use `..` paths)
3. `make backend` — `go build` embeds the copied assets via `//go:embed all:dist`

### Build Without Make

```bash
cd frontend && npm ci && npm run build && cd ..
rm -rf cmd/flywheel-planner/dist
cp -r frontend/dist cmd/flywheel-planner/dist
go build -ldflags "-s -w" -o flywheel-planner ./cmd/flywheel-planner
```

### Development Mode

No production build needed — the Vite dev server proxies `/api` to the Go backend:

```bash
# Terminal 1: backend (uses placeholder frontend embed)
go run ./cmd/flywheel-planner

# Terminal 2: frontend dev server with hot reload
cd frontend && npm run dev
# Open http://localhost:5173
```

### Tests and Quality Gates

```bash
make test           # Backend unit tests + frontend tests
make test-race      # Backend with race detector (requires CGo)
make test-coverage  # HTML coverage report in test-reports/
make lint           # go vet + ESLint
make check          # TypeScript + Go build verification
make fmt-check      # Formatting check (CI mode)
```
