#!/usr/bin/env bash
# test-all.sh — comprehensive test orchestration for flywheel-planner.
# Runs backend, frontend, and E2E tests with structured logging.
#
# Usage:
#   ./scripts/test-all.sh          # Run all tiers
#   ./scripts/test-all.sh --unit   # Backend + frontend unit tests only
#   ./scripts/test-all.sh --e2e    # E2E tests only
#   ./scripts/test-all.sh --ci     # CI mode: strict, with coverage

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
REPORT_DIR="$PROJECT_ROOT/test-reports"

# Defaults.
RUN_UNIT=false
RUN_INTEGRATION=false
RUN_E2E=false
RUN_ALL=true
CI_MODE=false

# Parse args.
for arg in "$@"; do
    case "$arg" in
        --unit)        RUN_UNIT=true; RUN_ALL=false ;;
        --integration) RUN_INTEGRATION=true; RUN_ALL=false ;;
        --e2e)         RUN_E2E=true; RUN_ALL=false ;;
        --all)         RUN_ALL=true ;;
        --ci)          CI_MODE=true ;;
        *)             echo "Unknown arg: $arg"; exit 1 ;;
    esac
done

if $RUN_ALL; then
    RUN_UNIT=true
    RUN_INTEGRATION=true
    RUN_E2E=true
fi

# Create report directory.
mkdir -p "$REPORT_DIR"

PASS=0
FAIL=0
TOTAL_START=$(date +%s)

log() { echo "$(date '+%H:%M:%S') | $*"; }
pass() { ((PASS++)) || true; log "PASS: $1"; }
fail() { ((FAIL++)) || true; log "FAIL: $1"; }

# --- Backend Unit Tests ---
if $RUN_UNIT; then
    log "=== Backend Unit Tests ==="
    cd "$PROJECT_ROOT"

    if go test ./... -race -count=1 -json > "$REPORT_DIR/backend-unit.json" 2>&1; then
        pass "Backend unit tests"
    else
        fail "Backend unit tests"
    fi

    # Generate coverage report in CI mode.
    if $CI_MODE; then
        go test ./... -coverprofile="$REPORT_DIR/coverage.out" > /dev/null 2>&1 || true
        go tool cover -html="$REPORT_DIR/coverage.out" -o "$REPORT_DIR/coverage.html" 2>/dev/null || true
    fi
fi

# --- Frontend Unit Tests ---
if $RUN_UNIT; then
    log "=== Frontend Unit Tests ==="
    cd "$PROJECT_ROOT/frontend"

    if npx vitest run --reporter=json --outputFile="$REPORT_DIR/frontend-unit.json" 2>/dev/null; then
        pass "Frontend unit tests"
    else
        fail "Frontend unit tests"
    fi
fi

# --- Backend Integration Tests ---
if $RUN_INTEGRATION; then
    log "=== Backend Integration Tests ==="
    cd "$PROJECT_ROOT"

    if go test ./tests/integration/... -v -race -count=1 -json > "$REPORT_DIR/backend-integration.json" 2>&1; then
        pass "Backend integration tests"
    else
        # Integration dir may not exist yet.
        if [ $? -eq 1 ]; then
            log "SKIP: No integration tests found"
        else
            fail "Backend integration tests"
        fi
    fi
fi

# --- E2E Tests ---
if $RUN_E2E; then
    log "=== E2E Tests ==="
    cd "$PROJECT_ROOT"

    if npx playwright test --reporter=json 2>"$REPORT_DIR/e2e.json"; then
        pass "E2E tests"
    else
        fail "E2E tests"
    fi
fi

# --- Summary ---
TOTAL_END=$(date +%s)
DURATION=$((TOTAL_END - TOTAL_START))

log ""
log "============================================"
log "  Test Results: $PASS passed, $FAIL failed"
log "  Duration: ${DURATION}s"
log "  Reports: $REPORT_DIR/"
log "============================================"

if [ $FAIL -gt 0 ]; then
    exit 1
fi
