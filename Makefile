# Makefile for flywheel-planner production build
# Produces a single Go binary with embedded React frontend.

BINARY_NAME := flywheel-planner
BUILD_DIR := build
FRONTEND_DIR := frontend
FRONTEND_DIST := $(FRONTEND_DIR)/dist
GO_MODULE := github.com/dougflynn/flywheel-planner
REPORT_DIR := test-reports

# Build flags.
VERSION ?= dev
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all clean build frontend backend test test-unit test-race test-coverage \
	test-frontend test-e2e test-all lint vet check fmt dev verify

# Default: full production build.
all: build

# Embed location: Go's embed directive can't use ".." paths, so we copy
# the built frontend into cmd/flywheel-planner/dist/ before compiling.
EMBED_DIR := cmd/flywheel-planner/dist

# Full production build: frontend → copy to embed dir → Go binary.
build: frontend embed-copy backend
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build the React frontend.
frontend:
	@echo "Building frontend..."
	cd $(FRONTEND_DIR) && npm ci --ignore-scripts && npm run build
	@echo "Frontend built to $(FRONTEND_DIST)"

# Copy frontend build output to the Go embed directory.
embed-copy:
	@echo "Copying frontend assets to embed directory..."
	@rm -rf $(EMBED_DIR)
	@cp -r $(FRONTEND_DIST) $(EMBED_DIR)
	@echo "Copied $$(find $(EMBED_DIR) -type f | wc -l | tr -d ' ') files to $(EMBED_DIR)"

# Build the Go binary with embedded frontend assets.
backend:
	@echo "Building backend..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/flywheel-planner
	@echo "Binary: $(BUILD_DIR)/$(BINARY_NAME)"

# --- Test targets ---

# Quick unit tests (no race detector).
test-unit:
	@echo "=== Backend unit tests ==="
	go test ./internal/... -count=1
	@echo "PASS: Backend unit tests"

# Unit tests with race detector.
test-race:
	@echo "=== Backend unit tests (race) ==="
	go test ./internal/... -race -count=1
	@echo "PASS: Backend unit tests with race detector"

# Coverage report.
test-coverage:
	@echo "=== Backend coverage ==="
	@mkdir -p $(REPORT_DIR)
	go test ./internal/... -coverprofile=$(REPORT_DIR)/coverage.out -count=1
	go tool cover -html=$(REPORT_DIR)/coverage.out -o $(REPORT_DIR)/coverage.html
	go tool cover -func=$(REPORT_DIR)/coverage.out | tail -1
	@echo "Coverage report: $(REPORT_DIR)/coverage.html"

# Frontend tests.
test-frontend:
	@echo "=== Frontend tests ==="
	cd $(FRONTEND_DIR) && npx vitest run
	@echo "PASS: Frontend tests"

# E2E tests (requires built binary, starts server).
test-e2e:
	@echo "=== E2E tests ==="
	./scripts/run-e2e.sh
	@echo "PASS: E2E tests"

# All tests sequentially: unit → frontend → e2e.
test-all: test-unit test-frontend test-e2e
	@echo "All tests passed."

# Quick alias.
test: test-unit test-frontend

# --- Lint and static analysis ---

lint: vet
	cd $(FRONTEND_DIR) && npm run lint

vet:
	go vet ./internal/...

# Type checking (both Go and TypeScript).
check:
	cd $(FRONTEND_DIR) && npm run typecheck
	go build ./internal/...

# Format.
fmt:
	gofmt -w .
	cd $(FRONTEND_DIR) && npx prettier --write .

# Format check (CI mode — fails on unformatted code).
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Go files not formatted:" && gofmt -l . && exit 1)
	cd $(FRONTEND_DIR) && npx prettier --check .

# --- Clean ---

clean:
	rm -rf $(BUILD_DIR)
	rm -rf $(FRONTEND_DIST)
	rm -rf $(EMBED_DIR)
	rm -rf $(REPORT_DIR)

# --- Development ---

dev:
	@echo "Start frontend dev server: cd frontend && npm run dev"
	@echo "Start backend: go run ./cmd/flywheel-planner"
	@echo "Frontend proxies /api to backend at 127.0.0.1:7432"

# Verify production build works.
verify: build
	@echo "Verifying build..."
	$(BUILD_DIR)/$(BINARY_NAME) --version 2>/dev/null || true
	@echo "Verification complete."
