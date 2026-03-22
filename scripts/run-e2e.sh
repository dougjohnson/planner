#!/usr/bin/env bash
# run-e2e.sh — E2E test runner with server lifecycle management.
#
# Builds the Go binary, starts the server with mock providers,
# waits for health, runs Playwright tests, then cleans up.
#
# Usage:
#   ./scripts/run-e2e.sh              # Run all E2E tests
#   ./scripts/run-e2e.sh --headed     # Run with visible browser
#   ./scripts/run-e2e.sh --grep "PRD" # Filter tests by name

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BUILD_DIR="$PROJECT_ROOT/build"
BINARY="$BUILD_DIR/flywheel-planner"
DATA_DIR=$(mktemp -d)
PORT=17432  # Non-default port to avoid conflicts with dev server.
LISTEN_ADDR="127.0.0.1:$PORT"
SERVER_PID=""
PLAYWRIGHT_ARGS=()
EXIT_CODE=0

# Parse args — pass through to Playwright.
for arg in "$@"; do
    PLAYWRIGHT_ARGS+=("$arg")
done

# Cleanup function — runs on exit, SIGINT, SIGTERM.
cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        echo "Stopping server (PID $SERVER_PID)..."
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    rm -rf "$DATA_DIR"
    echo "Cleanup complete."
}
trap cleanup EXIT INT TERM

log() { echo "$(date '+%H:%M:%S') [e2e] $*"; }

# Step 1: Build the binary.
log "Building Go binary..."
cd "$PROJECT_ROOT"
mkdir -p "$BUILD_DIR"
go build -o "$BINARY" ./cmd/flywheel-planner
log "Binary built: $BINARY"

# Step 2: Start the server with mock providers.
log "Starting server on $LISTEN_ADDR (data: $DATA_DIR)..."
export FLYWHEEL_DATA_DIR="$DATA_DIR"
export FLYWHEEL_LISTEN_ADDR="$LISTEN_ADDR"
export FLYWHEEL_LOG_LEVEL="warn"
export FLYWHEEL_MOCK_PROVIDERS="true"

"$BINARY" &
SERVER_PID=$!
log "Server started (PID $SERVER_PID)"

# Step 3: Wait for health endpoint.
log "Waiting for server health..."
MAX_WAIT=30
WAITED=0
while [ $WAITED -lt $MAX_WAIT ]; do
    if curl -sf "http://$LISTEN_ADDR/api/health" > /dev/null 2>&1; then
        log "Server healthy after ${WAITED}s"
        break
    fi
    # Check if process died.
    if ! kill -0 "$SERVER_PID" 2>/dev/null; then
        log "ERROR: Server process died before becoming healthy"
        exit 1
    fi
    sleep 1
    WAITED=$((WAITED + 1))
done

if [ $WAITED -ge $MAX_WAIT ]; then
    log "ERROR: Server did not become healthy within ${MAX_WAIT}s"
    exit 1
fi

# Step 4: Run Playwright tests.
log "Running Playwright tests..."
cd "$PROJECT_ROOT"

# Set the base URL for Playwright.
export BASE_URL="http://$LISTEN_ADDR"

npx playwright test \
    --config tests/e2e/playwright.config.ts \
    "${PLAYWRIGHT_ARGS[@]}" || EXIT_CODE=$?

# Step 5: Report results.
if [ $EXIT_CODE -eq 0 ]; then
    log "All E2E tests PASSED"
else
    log "E2E tests FAILED (exit code $EXIT_CODE)"
fi

exit $EXIT_CODE
